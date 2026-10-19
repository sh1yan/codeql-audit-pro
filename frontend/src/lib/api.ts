import type { AppApi } from "../types";

// Wails 在运行时把绑定方法挂到 window.go.main.App 上。
// 我们刻意不依赖 wailsjs 生成的 TypeScript 绑定：手写这一层可以让前端
// 在没有 Wails 的浏览器里也能用 mock 后端做视觉与交互自检。

type Backend = AppApi;

const localBus = new Map<string, Set<(payload: unknown) => void>>();

export function emitLocal(event: string, payload: unknown) {
  for (const cb of localBus.get(event) ?? []) {
    try {
      cb(payload);
    } catch (err) {
      console.error("local event handler failed", err);
    }
  }
}

function wailsRuntime(): any {
  return (window as any).runtime;
}

export function isWails(): boolean {
  return !!(window as any).go?.main?.App;
}

export function onEvent<T = unknown>(event: string, cb: (payload: T) => void): () => void {
  const rt = wailsRuntime();
  if (rt?.EventsOn) {
    rt.EventsOn(event, cb);
    return () => {
      try {
        rt.EventsOff(event);
      } catch {
        /* ignore */
      }
    };
  }
  const set = localBus.get(event) ?? new Set();
  set.add(cb as (p: unknown) => void);
  localBus.set(event, set);
  return () => set.delete(cb as (p: unknown) => void);
}

let backendPromise: Promise<Backend> | null = null;

function getBackend(): Promise<Backend> {
  if (backendPromise) return backendPromise;
  backendPromise = (async () => {
    const go = (window as any).go?.main?.App;
    if (go) {
      return go as Backend;
    }
    if (import.meta.env.DEV) {
      const mod = await import("../dev/mock");
      return mod.mockApi as unknown as Backend;
    }
    throw new Error(
      "未检测到 Wails 运行时。请用 `wails dev` 启动，或直接运行打包后的桌面应用。",
    );
  })();
  return backendPromise;
}

function call(name: string, args: unknown[]): Promise<unknown> {
  return getBackend().then((backend) => {
    const fn = (backend as any)[name];
    if (typeof fn !== "function") {
      throw new Error(`后端方法不存在：${name}`);
    }
    return fn.apply(backend, args);
  });
}

// Proxy 让 api.xxx(...) 的调用保持完整类型检查（方法签名来自 AppApi）。
export const api = new Proxy(
  {},
  {
    get(_target, prop: string) {
      return (...args: unknown[]) => call(prop, args);
    },
  },
) as AppApi;
