import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  BookOpen,
  CheckCircle2,
  Clock,
  Copy,
  Database,
  FileSpreadsheet,
  FileText,
  FolderOpen,
  FolderTree,
  HardDrive,
  History,
  Home,
  Loader2,
  Moon,
  Play,
  Quote,
  RefreshCw,
  Save,
  Square,
  Sun,
  TerminalSquare,
  Trash2,
  X,
} from "lucide-react";

import type {
  AssistantConfigDTO,
  AssistantDoneDTO,
  AssistantHistoryDTO,
  AssistantOutputDTO,
  AssistantStateDTO,
  AssistantStatusDTO,
  Theme,
} from "../types";
import { api, isWails, onEvent } from "../lib/api";
import { cn, shortPath } from "../lib/format";
import {
  Modal,
  ToastStack,
  Tooltip,
  type ToastItem,
  type ToastKind,
} from "./ui";
import { CommandSheet } from "./CommandSheet";

const MAX_CONSOLE_LINES = 3000;

const STATUS_TONE: Record<string, "idle" | "running" | "success" | "error" | "stopped"> = {
  idle: "idle",
  running: "running",
  completed: "success",
  failed: "error",
  stopped: "stopped",
};

function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms <= 0) return "0 秒";
  const total = Math.round(ms / 1000);
  if (total < 60) return `${total} 秒`;
  const minutes = Math.floor(total / 60);
  const seconds = total % 60;
  if (minutes < 60) return `${minutes} 分 ${String(seconds).padStart(2, "0")} 秒`;
  const hours = Math.floor(minutes / 60);
  return `${hours} 小时 ${String(minutes % 60).padStart(2, "0")} 分`;
}

const LANGUAGE_LABELS: Record<string, string> = {
  typescript: "typescript → javascript",
  kotlin: "kotlin → java",
};

function actionLabel(action: string): string {
  return action === "create" ? "创建数据库" : action === "analyze" ? "运行分析" : "任务";
}

function defaultOutputName(format: string): string {
  return format.toLowerCase() === "csv" ? "codeql-results.csv" : "codeql-results.sarif";
}

export function AssistantWorkspace({
  active,
  version,
  theme,
  onToggleTheme,
  onHome,
  onRunningChange,
  onOpenAudit,
}: {
  active: boolean;
  version: string;
  theme: Theme;
  onToggleTheme: () => void;
  onHome: () => void;
  onRunningChange?: (running: boolean, label: string) => void;
  onOpenAudit?: (csvPath: string, srcRoot: string) => void;
}) {
  const [boot, setBoot] = useState<AssistantStateDTO | null>(null);
  const [bootError, setBootError] = useState("");
  const [config, setConfig] = useState<AssistantConfigDTO | null>(null);
  const [profiles, setProfiles] = useState<string[]>([]);
  const [history, setHistory] = useState<AssistantHistoryDTO[]>([]);
  const [quote, setQuote] = useState("");
  const [lines, setLines] = useState<{ text: string; error: boolean }[]>([]);
  const [running, setRunning] = useState(false);
  const [status, setStatus] = useState("idle");
  const [statusLabel, setStatusLabel] = useState("就绪");
  const [action, setAction] = useState("");
  const [startedAt, setStartedAt] = useState("");
  const [elapsedBase, setElapsedBase] = useState(0);
  const [now, setNow] = useState(Date.now());
  const [logPath, setLogPath] = useState("");
  const [selectedProfile, setSelectedProfile] = useState("");
  const [sheetOpen, setSheetOpen] = useState(
    () => new URLSearchParams(window.location.search).get("sheet") === "1",
  );
  const [historyOpen, setHistoryOpen] = useState(false);
  const [saveProfileOpen, setSaveProfileOpen] = useState(false);
  const [profileName, setProfileName] = useState("");
  const [deleteProfile, setDeleteProfile] = useState<string | null>(null);
  const [clearHistoryArmed, setClearHistoryArmed] = useState(false);
  const [starting, setStarting] = useState(false);
  const [readyCsv, setReadyCsv] = useState<{ path: string; srcRoot: string } | null>(null);
  const [toasts, setToasts] = useState<ToastItem[]>([]);

  const consoleRef = useRef<HTMLDivElement>(null);
  const stickRef = useRef(true);
  const configRef = useRef<AssistantConfigDTO | null>(null);
  const toastIdRef = useRef(0);

  configRef.current = config;

  const notify = useCallback((kind: ToastKind, message: string) => {
    const id = ++toastIdRef.current;
    setToasts((prev) => [...prev.slice(-4), { id, kind, message }]);
    window.setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, kind === "error" ? 8000 : 4600);
  }, []);

  const refreshMeta = useCallback(async () => {
    try {
      const next = await api.AssistantGetState();
      setProfiles(next.profiles);
      setHistory(next.history);
      setQuote(next.quote);
      setLogPath(next.logPath);
      setRunning(next.running);
      setStatus(next.status);
      setStatusLabel(next.statusLabel);
      setAction(next.action);
      setStartedAt(next.startedAt);
      setElapsedBase(next.elapsedMs);
      setNow(Date.now());
    } catch {
      /* 元数据刷新失败不影响正在运行的任务 */
    }
  }, []);

  // 启动：读取运行器配置与历史。
  useEffect(() => {
    let cancelled = false;
    api
      .AssistantBootstrap()
      .then((state) => {
        if (cancelled) return;
        setBoot(state);
        setConfig(state.config);
        setProfiles(state.profiles);
        setHistory(state.history);
        setQuote(state.quote);
        setRunning(state.running);
        setStatus(state.status);
        setStatusLabel(state.statusLabel);
        setAction(state.action);
        setStartedAt(state.startedAt);
        setElapsedBase(state.elapsedMs);
        setLogPath(state.logPath);
      })
      .catch((err) => {
        if (!cancelled) setBootError(String(err));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // 后端事件：批量输出、状态变化、任务结束。
  useEffect(() => {
    const offOutput = onEvent<AssistantOutputDTO>("assistant:output", (payload) => {
      const incoming = (payload?.lines ?? []).map((l) => ({ text: l.text, error: !!l.error }));
      setLines((prev) => {
        const next = payload?.reset ? incoming : [...prev, ...incoming];
        return next.length > MAX_CONSOLE_LINES ? next.slice(-MAX_CONSOLE_LINES) : next;
      });
    });
    const offStatus = onEvent<AssistantStatusDTO>("assistant:status", (payload) => {
      setStarting(false);
      setRunning(payload.running);
      setStatus(payload.status);
      setStatusLabel(payload.statusLabel);
      setAction(payload.action);
      setStartedAt(payload.startedAt);
      setElapsedBase(payload.elapsedMs);
      setLogPath(payload.logPath || "");
      setNow(Date.now());
    });
    const offDone = onEvent<AssistantDoneDTO>("assistant:done", (payload) => {
      setStarting(false);
      setRunning(false);
      setStatus(payload.status);
      setStatusLabel(payload.statusLabel);
      const ok = payload.status === "completed";
      if (ok) {
        notify(
          "success",
          `${actionLabel(payload.action)}完成，耗时 ${formatDuration(payload.durationMs)}`,
        );
      } else {
        notify(
          "error",
          `${actionLabel(payload.action)}${payload.status === "stopped" ? "已停止" : "失败"}${
            payload.error ? `：${payload.error}` : ""
          }`,
        );
      }
      if (
        payload.action === "analyze" &&
        ok &&
        payload.outputFile &&
        /\.(csv|sarif|json)$/i.test(payload.outputFile)
      ) {
        setReadyCsv({ path: payload.outputFile, srcRoot: configRef.current?.sourceRoot || "" });
      }
      void refreshMeta();
    });
    return () => {
      offOutput();
      offStatus();
      offDone();
    };
  }, [notify, refreshMeta]);

  useEffect(() => {
    onRunningChange?.(running, statusLabel);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [running, statusLabel]);

  useEffect(() => {
    if (!running) return;
    const timer = window.setInterval(() => setNow(Date.now()), 1000);
    return () => window.clearInterval(timer);
  }, [running]);

  // 每次从首页进入运行器时同步一次后端状态，并把控制台带回底部。
  useEffect(() => {
    if (!active) return;
    void refreshMeta();
    const el = consoleRef.current;
    if (el && stickRef.current) el.scrollTop = el.scrollHeight;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [active]);

  // 控制台自动滚动：用户主动向上翻时不再打扰。
  useEffect(() => {
    const el = consoleRef.current;
    if (!el || !stickRef.current) return;
    el.scrollTop = el.scrollHeight;
  }, [lines]);

  // 配置自动保存（防抖）。
  useEffect(() => {
    if (!config || !boot) return;
    const timer = window.setTimeout(() => {
      api.AssistantSaveConfig(config).catch(() => {});
    }, 700);
    return () => window.clearTimeout(timer);
  }, [config, boot]);

  const patchConfig = (patch: Partial<AssistantConfigDTO>) => {
    setConfig((prev) => (prev ? { ...prev, ...patch } : prev));
  };

  const elapsedMs = running && startedAt ? Math.max(0, now - Date.parse(startedAt)) : elapsedBase;

  const tone = STATUS_TONE[status] ?? "idle";

  // 把连续的同类型行合并成少量 <span>，避免几千行日志每次刷新都重建几千个 DOM 节点。
  const consoleChunks = useMemo(() => {
    const chunks: { error: boolean; text: string }[] = [];
    for (const line of lines) {
      const text = (line.text || " ") + "\n";
      const last = chunks[chunks.length - 1];
      if (last && last.error === line.error) {
        last.text += text;
      } else {
        chunks.push({ error: line.error, text });
      }
    }
    return chunks;
  }, [lines]);

  const copyConsole = async () => {
    if (lines.length === 0) return;
    try {
      await api.CopyText(lines.map((l) => l.text).join("\n"));
      notify("success", "控制台输出已复制");
    } catch (err) {
      notify("error", String(err));
    }
  };

  const runCreate = async () => {
    if (!config || running || starting) return;
    setStarting(true);
    try {
      await api.AssistantRunCreate(config);
      // 保持 starting，等后端 assistant:status 事件到达后再解除，避免重复点击。
    } catch (err) {
      setStarting(false);
      notify("error", String(err));
    }
  };

  const runAnalyze = async () => {
    if (!config || running || starting) return;
    setStarting(true);
    try {
      await api.AssistantRunAnalyze(config);
    } catch (err) {
      setStarting(false);
      notify("error", String(err));
    }
  };

  const stop = async () => {
    try {
      await api.AssistantStop();
    } catch (err) {
      notify("error", String(err));
    }
  };

  const loadProfile = async (name: string) => {
    setSelectedProfile(name);
    if (!name) return;
    try {
      const next = await api.AssistantLoadProfile(name);
      setConfig(next);
      notify("success", `已载入配置「${name}」`);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const saveProfile = async () => {
    if (!config) return;
    const name = profileName.trim();
    if (!name) {
      notify("error", "请输入配置名称");
      return;
    }
    try {
      await api.AssistantSaveProfile(name, config);
      const next = await api.AssistantListProfiles();
      const cleanName = name.replace(/\.json$/i, "");
      setProfiles(next);
      setSelectedProfile(cleanName);
      setConfig((prev) => (prev ? { ...prev, name: cleanName } : prev));
      setSaveProfileOpen(false);
      setProfileName("");
      notify("success", `配置「${name}」已保存`);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const confirmDeleteProfile = async () => {
    if (!deleteProfile) return;
    try {
      await api.AssistantDeleteProfile(deleteProfile);
      const next = await api.AssistantListProfiles();
      setProfiles(next);
      setDeleteProfile(null);
      setSelectedProfile("");
      notify("success", `配置「${deleteProfile}」已删除`);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const clearHistory = async () => {
    try {
      await api.AssistantClearHistory();
      setHistory([]);
      setClearHistoryArmed(false);
      notify("success", "运行历史已清空");
    } catch (err) {
      notify("error", String(err));
    }
  };

  const openLog = async () => {
    try {
      await api.AssistantOpenLog();
    } catch (err) {
      notify("error", String(err));
    }
  };

  const openPath = async (path: string) => {
    try {
      await api.AssistantOpenPath(path);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const canRun = !!config && !running && !starting;
  const statusIcon =
    tone === "running" ? (
      <Loader2 size={12} className="spin" />
    ) : tone === "success" ? (
      <CheckCircle2 size={12} />
    ) : tone === "error" ? (
      <AlertCircle size={12} />
    ) : tone === "stopped" ? (
      <Square size={11} />
    ) : (
      <TerminalSquare size={12} />
    );

  if (bootError) {
    return (
      <div className="assistant-shell">
        <div className="empty-pane">
          <div className="empty-icon">
            <AlertCircle size={20} className="text-sev-error" />
          </div>
          <div className="empty-title">运行器后端未就绪</div>
          <div className="empty-desc">{bootError}</div>
          <button className="btn btn-outline" onClick={onHome}>
            <Home size={14} /> 返回首页
          </button>
        </div>
      </div>
    );
  }

  if (!boot || !config) {
    return (
      <div className="assistant-shell">
        <div className="empty-pane">
          <Loader2 size={22} className="spin text-accent" />
          <div className="empty-title">正在启动 CodeQL 运行器…</div>
        </div>
      </div>
    );
  }

  return (
    <div className="assistant-shell">
      <header className={cn("topbar assistant-topbar", isWails() && boot.platform === "darwin" && "topbar--mac")}>
        <button className="btn no-drag" onClick={onHome} title="返回工具选择">
          <Home size={14} />
          <span className="hide-narrow">首页</span>
        </button>
        <div className="brand">
          <div className="brand-mark assistant-brand-mark">
            <TerminalSquare size={17} strokeWidth={2.1} />
          </div>
          <div className="flex flex-col leading-none">
            <span className="brand-title">CodeQL 运行器</span>
            <span className="brand-sub text-[8.5px] text-tertiary tracking-[0.08em] mt-1">
              DATABASE &amp; ANALYSIS RUNNER
            </span>
          </div>
          <span className="brand-tag">Pro</span>
        </div>

        <div className="topbar-sep" />
        <div className="assistant-topbar-quote" title={quote || "正在加载语录"}>
          <Quote size={13} className="text-accent flex-none" />
          <span className="truncate">“{quote}”</span>
          <button
            className="btn btn-icon btn-sm assistant-quote-refresh no-drag"
            title="换一句"
            onClick={() => {
              api
                .AssistantRandomQuote()
                .then(setQuote)
                .catch(() => {});
            }}
          >
            <RefreshCw size={10} />
          </button>
        </div>

        <div className="flex-1" />

        <Tooltip tip="查看可复制的 CodeQL 命令">
          <button className="btn no-drag" onClick={() => setSheetOpen(true)}>
            <BookOpen size={14} />
            <span className="hide-narrow">命令示例</span>
          </button>
        </Tooltip>
        <button className="btn no-drag" onClick={() => setHistoryOpen(true)}>
          <History size={14} />
          <span className="hide-narrow">历史</span>
          {history.length > 0 && <span className="assistant-count">{history.length}</span>}
        </button>
        <div className="topbar-sep" />
        <Tooltip tip={theme === "dark" ? "切换到浅色主题" : "切换到深色主题"}>
          <button className="btn btn-icon no-drag" onClick={onToggleTheme}>
            {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
          </button>
        </Tooltip>
      </header>

      <div className="assistant-body">
        <aside className="pane assistant-config">
          <div className="assistant-pane-head">
            <div>
              <div className="text-[12px] font-semibold text-primary">运行配置</div>
              <div className="text-[10px] text-tertiary mt-0.5">
                修改只在下次运行时生效，不会打断当前任务
              </div>
            </div>
            <span className="chip">{config.language || "java"}</span>
          </div>

          <div className="assistant-form">
            <section className="assistant-section">
              <div className="assistant-section-title">
                <TerminalSquare size={12} /> CodeQL 与查询
              </div>
              <PathField
                label="CodeQL 二进制"
                value={config.binaryPath}
                placeholder="/usr/local/bin/codeql"
                onChange={(v) => patchConfig({ binaryPath: v })}
                actions={[
                  {
                    label: "选择",
                    icon: FileText,
                    onClick: async () => {
                      const v = await api.AssistantChooseBinary();
                      if (v) patchConfig({ binaryPath: v });
                    },
                  },
                ]}
              />
              <PathField
                label="查询 / 套件"
                value={config.queryPath}
                placeholder=".ql / .qls 文件，或查询包目录"
                onChange={(v) => patchConfig({ queryPath: v })}
                actions={[
                  {
                    label: "文件",
                    icon: FileText,
                    onClick: async () => {
                      const v = await api.AssistantChooseQueryFile();
                      if (v) patchConfig({ queryPath: v });
                    },
                  },
                  {
                    label: "目录",
                    icon: FolderTree,
                    onClick: async () => {
                      const v = await api.AssistantChooseQueryDir();
                      if (v) patchConfig({ queryPath: v });
                    },
                  },
                ]}
              />
            </section>

            <section className="assistant-section">
              <div className="assistant-section-title">
                <HardDrive size={12} /> 数据库与源码
              </div>
              <PathField
                label="数据库目录"
                value={config.databasePath}
                placeholder="~/codeql-db/my-project"
                onChange={(v) => patchConfig({ databasePath: v })}
                actions={[
                  {
                    label: "选择",
                    icon: FolderTree,
                    onClick: async () => {
                      const v = await api.AssistantChooseDirectory("选择 CodeQL 数据库目录", config.databasePath);
                      if (v) patchConfig({ databasePath: v });
                    },
                  },
                ]}
              />
              <PathField
                label="源码根目录"
                value={config.sourceRoot}
                placeholder="/path/to/project"
                onChange={(v) => patchConfig({ sourceRoot: v })}
                actions={[
                  {
                    label: "选择",
                    icon: FolderOpen,
                    onClick: async () => {
                      const v = await api.AssistantChooseDirectory("选择源码根目录", config.sourceRoot);
                      if (v) patchConfig({ sourceRoot: v });
                    },
                  },
                ]}
              />
              <div className="assistant-inline-grid">
                <label className="assistant-field">
                  <span className="assistant-label">编程语言</span>
                  <select
                    className="select"
                    value={config.language}
                    onChange={(e) => patchConfig({ language: e.target.value })}
                  >
                    {(boot.languages.length ? boot.languages : [config.language]).map((lang) => (
                      <option key={lang} value={lang}>
                        {LANGUAGE_LABELS[lang] ?? lang}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="assistant-field">
                  <span className="assistant-label">输出格式</span>
                  <select
                    className="select"
                    value={config.outputFormat}
                    onChange={(e) => patchConfig({ outputFormat: e.target.value })}
                  >
                    {(boot.formats.length ? boot.formats : [config.outputFormat]).map((fmt) => (
                      <option key={fmt} value={fmt}>
                        {fmt}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
              <span className="assistant-hint">
                TypeScript 由 CodeQL 的 javascript extractor 处理，Kotlin 由 java extractor 处理，执行时会自动映射。
              </span>
              <PathField
                label="输出文件"
                value={config.outputFile}
                placeholder="留空则由 CodeQL 决定，推荐显式指定"
                onChange={(v) => patchConfig({ outputFile: v })}
                actions={[
                  {
                    label: "另存为",
                    icon: Save,
                    onClick: async () => {
                      const v = await api.AssistantChooseOutputFile(defaultOutputName(config.outputFormat));
                      if (v) patchConfig({ outputFile: v });
                    },
                  },
                ]}
              />
            </section>

            <section className="assistant-section">
              <div className="assistant-section-title">
                <Database size={12} /> 资源与参数
              </div>
              <div className="assistant-inline-grid">
                <label className="assistant-field">
                  <span className="assistant-label">内存限额（MB）</span>
                  <input
                    className="input"
                    type="number"
                    min={0}
                    step={512}
                    value={config.ramLimit}
                    onChange={(e) => patchConfig({ ramLimit: Number(e.target.value) || 0 })}
                  />
                </label>
                <label className="assistant-field">
                  <span className="assistant-label">线程数</span>
                  <input
                    className="input"
                    type="number"
                    min={0}
                    step={1}
                    value={config.threads}
                    onChange={(e) => patchConfig({ threads: Number(e.target.value) || 0 })}
                  />
                </label>
              </div>
              <span className="assistant-hint">
                内存限额或线程数填 0 时，命令会省略对应的 --ram / --threads，由 CodeQL 自己决定。
              </span>
              <label className="assistant-field">
                <span className="assistant-label">附加参数</span>
                <input
                  className="input font-mono-ui"
                  value={config.extraArgs}
                  placeholder={'例如 --no-sandbox --external="a b"'}
                  onChange={(e) => patchConfig({ extraArgs: e.target.value })}
                />
                <span className="assistant-hint">支持双引号包裹含空格的参数，会追加到创建与分析的命令末尾。</span>
              </label>
            </section>

            <section className="assistant-section">
              <div className="assistant-section-title">
                <Save size={12} /> 配置快照
              </div>
              <div className="assistant-profile-row">
                <select
                  className="select"
                  value={selectedProfile}
                  onChange={(e) => void loadProfile(e.target.value)}
                >
                  <option value="">选择已保存的配置…</option>
                  {profiles.map((name) => (
                    <option key={name} value={name}>
                      {name}
                    </option>
                  ))}
                </select>
                <button
                  className="btn btn-sm"
                  onClick={() => {
                    setProfileName(config.name || selectedProfile || "");
                    setSaveProfileOpen(true);
                  }}
                  title="保存当前配置快照"
                >
                  <Save size={12} /> 保存
                </button>
                <button
                  className="btn btn-sm"
                  disabled={!selectedProfile}
                  onClick={() => setDeleteProfile(selectedProfile)}
                  title="删除选中的配置快照"
                >
                  <Trash2 size={12} /> 删除
                </button>
              </div>
            </section>
          </div>

          <div className="assistant-config-foot">
            <div className="assistant-actions">
              <button className="btn btn-outline" disabled={!canRun} onClick={() => void runCreate()}>
                <Database size={14} />
                创建数据库
              </button>
              <button className="btn btn-primary" disabled={!canRun} onClick={() => void runAnalyze()}>
                <Play size={14} />
                开始分析
              </button>
              <button className="btn btn-danger" disabled={!running} onClick={() => void stop()}>
                <Square size={13} />
                强制停止
              </button>
            </div>
          </div>
        </aside>

        <main className="pane assistant-console">
          <div className="assistant-console-head">
            <div className="flex items-center gap-2 min-w-0">
              <TerminalSquare size={13} className="text-accent flex-none" />
              <span className="text-[12px] font-semibold text-primary">控制台输出</span>
              {running && (
                <span className={cn("assistant-status-pill small", `tone-${tone}`)}>
                  {statusIcon}
                  {statusLabel}
                </span>
              )}
              {running && (
                <span className="text-[10px] text-tertiary flex items-center gap-1">
                  <Clock size={10} />
                  {formatDuration(elapsedMs)}
                </span>
              )}
            </div>
            <div className="flex-1" />
            <button
              className="btn btn-sm"
              disabled={!logPath}
              onClick={() => void openLog()}
              title={logPath ? `打开完整日志：${logPath}` : "本次运行还没有日志"}
            >
              <FileText size={12} />
              打开日志
            </button>
            <button className="btn btn-sm" disabled={lines.length === 0} onClick={() => void copyConsole()}>
              <Copy size={12} />
              复制
            </button>
            <button className="btn btn-sm" disabled={lines.length === 0} onClick={() => setLines([])}>
              <Trash2 size={12} />
              清空
            </button>
          </div>

          {readyCsv && (
            <div className="assistant-result-banner">
              <FileSpreadsheet size={14} className="text-ai-fp flex-none" />
              <div className="min-w-0 flex-1">
                <div className="text-[11.5px] font-semibold text-primary">
                  已生成 {readyCsv.path.toLowerCase().endsWith(".csv") ? "CSV" : "SARIF v2.1.0"} 分析结果
                </div>
                <div className="text-[10px] text-tertiary font-mono truncate" title={readyCsv.path}>
                  {readyCsv.path}
                </div>
              </div>
              {onOpenAudit && (
                <button
                  className="btn btn-sm btn-soft"
                  onClick={() => onOpenAudit(readyCsv.path, readyCsv.srcRoot)}
                >
                  用审计工作台打开
                </button>
              )}
              <button className="btn btn-icon btn-sm" onClick={() => setReadyCsv(null)} title="关闭提示">
                <X size={11} />
              </button>
            </div>
          )}

          <div
            ref={consoleRef}
            className="assistant-console-scroll"
            onScroll={(e) => {
              const el = e.currentTarget;
              stickRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
            }}
          >
            {lines.length === 0 ? (
              <div className="assistant-console-empty">
                <div className="assistant-console-empty-icon">
                  <TerminalSquare size={20} />
                </div>
                <div className="text-[12px] font-semibold text-primary">控制台已就绪</div>
                <div className="text-[11px] text-tertiary mt-1 max-w-[420px] text-center leading-relaxed">
                  点击「创建数据库」或「开始分析」后，CodeQL 的实时输出会显示在这里。
                  完整日志会同时写入本地文件，即使输出被折叠也不会丢失。
                </div>
                <div className="assistant-console-empty-actions">
                  <button className="btn btn-sm btn-outline" onClick={() => setSheetOpen(true)}>
                    <BookOpen size={12} /> 查看可复制命令
                  </button>
                </div>
              </div>
            ) : (
              <pre className="console-pre">
                {consoleChunks.map((chunk, index) => (
                  <span key={index} className={cn(chunk.error && "console-line--error")}>
                    {chunk.text}
                  </span>
                ))}
              </pre>
            )}
          </div>

          <div className="assistant-console-foot">
            <span className="flex items-center gap-1.5 min-w-0 truncate">
              <span className="text-tertiary">日志</span>
              <span className="font-mono text-[10px] text-secondary truncate" title={logPath}>
                {logPath ? shortPath(logPath, 52) : "本次运行尚未创建"}
              </span>
            </span>
            <div className="flex-1" />
            <span className="text-[10px] text-tertiary">
              控制台保留最近 {MAX_CONSOLE_LINES} 行 · 完整日志保留 3 天
            </span>
          </div>
        </main>
      </div>

      <footer className="statusbar assistant-statusbar">
        <span className={cn("status-chip", `tone-text-${tone}`)}>
          {statusIcon}
          {statusLabel}
          {action && <span className="opacity-60">· {actionLabel(action)}</span>}
          {(running || elapsedBase > 0) && <span className="opacity-60">· {formatDuration(elapsedMs)}</span>}
        </span>
        <div className="flex-1" />
        {config.binaryPath && (
          <span className="status-chip" title={config.binaryPath}>
            <TerminalSquare size={10} />
            {shortPath(config.binaryPath, 36)}
          </span>
        )}
        {config.databasePath && (
          <span className="status-chip" title={config.databasePath}>
            <Database size={10} />
            {shortPath(config.databasePath, 30)}
          </span>
        )}
        <button
          className="status-chip assistant-link-chip"
          onClick={() => void api.AssistantOpenDataDir().catch((e) => notify("error", String(e)))}
          title={boot.dataDir}
        >
          <FolderOpen size={10} />
          数据目录
        </button>
        <span className="status-chip text-tertiary">
          {boot.platform === "darwin" ? "macOS" : boot.platform === "windows" ? "Windows" : "Linux"} · v
          {version}
        </span>
      </footer>

      <CommandSheet
        open={sheetOpen}
        onClose={() => setSheetOpen(false)}
        config={config}
        notify={notify}
      />

      <Modal
        open={historyOpen}
        onClose={() => {
          setHistoryOpen(false);
          setClearHistoryArmed(false);
        }}
        title="运行历史"
        subtitle="最多保留最近 100 次创建数据库 / 分析任务，记录写在 ~/.CodeQLAuditPro/assistant/history.json"
        icon={History}
        size="lg"
        footer={
          <>
            <button
              className={cn("btn btn-sm", clearHistoryArmed ? "btn-danger" : "")}
              disabled={history.length === 0}
              onClick={() => {
                if (!clearHistoryArmed) {
                  setClearHistoryArmed(true);
                  return;
                }
                void clearHistory();
              }}
            >
              <Trash2 size={12} />
              {clearHistoryArmed ? "确认清空" : "清空历史"}
            </button>
            <div className="flex-1" />
            <button
              className="btn btn-outline"
              onClick={() => {
                setHistoryOpen(false);
                setClearHistoryArmed(false);
              }}
            >
              关闭
            </button>
          </>
        }
      >
        {history.length === 0 ? (
          <div className="empty-pane" style={{ minHeight: 220 }}>
            <div className="empty-icon">
              <History size={20} />
            </div>
            <div className="empty-title">暂无运行记录</div>
            <div className="empty-desc">创建数据库或运行一次分析后，这里会记录状态、耗时与输出文件。</div>
          </div>
        ) : (
          <div className="assistant-history-list">
            {history.map((entry, index) => (
              <div className="assistant-history-item" key={`${entry.timestamp}-${index}`}>
                <div className={cn("assistant-history-dot", `tone-${statusToneOf(entry.status)}`)} />
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-[11.5px] font-semibold text-primary">
                      {entry.action === "create" ? "创建数据库" : entry.action === "analyze" ? "运行分析" : entry.status}
                    </span>
                    <span className="chip">{entry.language || "—"}</span>
                    <span
                      className={cn(
                        "badge",
                        entry.status === "已完成" ? "text-ai-fp" : entry.status === "失败" ? "text-sev-error" : "text-tertiary",
                      )}
                    >
                      {entry.status}
                    </span>
                  </div>
                  <div className="text-[10px] text-tertiary font-mono mt-1 truncate" title={entry.databasePath}>
                    {entry.databasePath}
                    {entry.queryPath ? `  ·  ${entry.queryPath}` : ""}
                  </div>
                  {entry.outputFile && (
                    <button
                      className="assistant-history-output"
                      onClick={() => void openPath(entry.outputFile)}
                      title={entry.outputFile}
                    >
                      <FileText size={10} />
                      {shortPath(entry.outputFile, 64)}
                    </button>
                  )}
                </div>
                <div className="assistant-history-meta">
                  <span>{new Date(entry.timestamp).toLocaleString("zh-CN", { hour12: false })}</span>
                  <span className="flex items-center gap-1">
                    <Clock size={10} />
                    {formatDuration(entry.durationMs)}
                  </span>
                </div>
              </div>
            ))}
          </div>
        )}
      </Modal>

      <Modal
        open={saveProfileOpen}
        onClose={() => setSaveProfileOpen(false)}
        title="保存配置快照"
        subtitle="保存到 ~/.CodeQLAuditPro/assistant/profiles，下次可从下拉框直接载入。"
        icon={Save}
        size="sm"
        footer={
          <>
            <button className="btn btn-outline" onClick={() => setSaveProfileOpen(false)}>
              取消
            </button>
            <button className="btn btn-primary" onClick={() => void saveProfile()}>
              <Save size={13} /> 保存
            </button>
          </>
        }
      >
        <label className="assistant-field">
          <span className="assistant-label">配置名称</span>
          <input
            className="input"
            autoFocus
            data-autofocus
            value={profileName}
            placeholder="例如 java-analysis"
            onChange={(e) => setProfileName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") void saveProfile();
            }}
          />
          <span className="assistant-hint">会覆盖同名的旧配置。</span>
        </label>
      </Modal>

      <Modal
        open={!!deleteProfile}
        onClose={() => setDeleteProfile(null)}
        title="删除配置快照"
        icon={Trash2}
        size="sm"
        footer={
          <>
            <button className="btn btn-outline" onClick={() => setDeleteProfile(null)}>
              取消
            </button>
            <button className="btn btn-danger" onClick={() => void confirmDeleteProfile()}>
              <Trash2 size={13} /> 删除
            </button>
          </>
        }
      >
        <div className="text-[12px] text-secondary">
          确定要删除配置「<span className="text-primary font-semibold">{deleteProfile}</span>」吗？
          此操作不可恢复。
        </div>
      </Modal>

      <ToastStack items={toasts} onClose={(id) => setToasts((t) => t.filter((x) => x.id !== id))} />
    </div>
  );
}

function statusToneOf(status: string): "idle" | "running" | "success" | "error" | "stopped" {
  if (status === "已完成") return "success";
  if (status === "失败") return "error";
  if (status === "已停止") return "stopped";
  return "idle";
}

function PathField({
  label,
  value,
  placeholder,
  onChange,
  actions,
}: {
  label: string;
  value: string;
  placeholder?: string;
  onChange: (value: string) => void;
  actions: { label: string; icon: typeof FolderOpen; onClick: () => void | Promise<void> }[];
}) {
  return (
    <label className="assistant-field">
      <span className="assistant-label">{label}</span>
      <div className="assistant-path-row">
        <input
          className="input font-mono-ui"
          value={value}
          placeholder={placeholder}
          spellCheck={false}
          onChange={(e) => onChange(e.target.value)}
        />
        {actions.map(({ label: actionLabelText, icon: Icon, onClick }) => (
          <button
            key={actionLabelText}
            className="btn btn-sm assistant-path-btn"
            onClick={(e) => {
              e.preventDefault();
              void onClick();
            }}
            title={actionLabelText}
          >
            <Icon size={12} />
            {actionLabelText}
          </button>
        ))}
      </div>
    </label>
  );
}
