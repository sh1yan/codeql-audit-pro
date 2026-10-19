import { Component, type ErrorInfo, type ReactNode } from "react";
import { AlertTriangle, RefreshCw } from "lucide-react";

type Props = { children: ReactNode };
type State = { error: Error | null };

/**
 * 顶层错误边界。
 *
 * 之前没有错误边界时，任何渲染期异常（例如后端把空数组序列化成 null、
 * 组件直接读 .length）都会让 React 卸载整棵树，用户只看到一片空白。
 * 这里至少把错误显示出来，并给出重新加载入口。
 */
export class ErrorBoundary extends Component<Props, State> {
  state: State = { error: null };

  static getDerivedStateFromError(error: Error): State {
    return { error };
  }

  componentDidCatch(error: Error, info: ErrorInfo) {
    console.error("界面渲染出错", error, info.componentStack);
  }

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;
    return (
      <div className="app-shell">
        <div className="start-screen">
          <div className="start-card">
            <div
              className="start-logo"
              style={{ background: "linear-gradient(135deg,#f87171,#dc2626)" }}
            >
              <AlertTriangle size={30} />
            </div>
            <div className="start-title" style={{ fontSize: 22 }}>
              界面渲染出错
            </div>
            <p className="start-desc">
              页面已停止渲染，避免继续显示一片空白。可以重新加载后继续使用；如果反复出现，请把下面的错误信息一起反馈。
            </p>
            <pre className="error-detail">{error.stack || error.message || String(error)}</pre>
            <button className="btn btn-primary" onClick={() => window.location.reload()}>
              <RefreshCw size={14} /> 重新加载
            </button>
          </div>
        </div>
      </div>
    );
  }
}
