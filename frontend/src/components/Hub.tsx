import {
  ArrowRight,
  BrainCircuit,
  Database,
  FileCode2,
  FileSpreadsheet,
  Fingerprint,
  FolderTree,
  Gauge,
  History,
  Layers,
  Loader2,
  Moon,
  PlayCircle,
  ScanSearch,
  ShieldCheck,
  Sparkles,
  Sun,
  TerminalSquare,
  Wand2,
  Workflow,
} from "lucide-react";
import type { Theme } from "../types";
import { cn, shortPath } from "../lib/format";
import { isWails } from "../lib/api";

export function Hub({
  version,
  platform,
  theme,
  lastProject,
  assistantRunning,
  assistantStatus,
  onToggleTheme,
  onOpenAudit,
  onOpenAssistant,
}: {
  version: string;
  platform: string;
  theme: Theme;
  lastProject: string;
  assistantRunning: boolean;
  assistantStatus: string;
  onToggleTheme: () => void;
  onOpenAudit: () => void;
  onOpenAssistant: () => void;
}) {
  const platformLabel =
    platform === "darwin" ? "macOS" : platform === "windows" ? "Windows" : "Linux";

  return (
    <div className="hub-screen">
      <div className="start-grid" />
      <div
        className="start-orb"
        style={{
          background: "radial-gradient(circle, #6366f1, transparent 66%)",
          left: "4%",
          top: "-24%",
          opacity: 0.42,
        }}
      />
      <div
        className="start-orb"
        style={{
          background: "radial-gradient(circle, #22d3ee, transparent 66%)",
          right: "2%",
          bottom: "-30%",
          opacity: 0.25,
        }}
      />

      <header className={cn("hub-topbar", isWails() && platform === "darwin" && "hub-topbar--mac")}>
        <div className="brand">
          <div className="brand-mark">
            <ShieldCheck size={17} strokeWidth={2.1} />
          </div>
          <div className="flex flex-col leading-none">
            <span className="brand-title">CodeQLAuditPro</span>
            <span className="brand-sub text-[8.5px] text-tertiary tracking-[0.08em] mt-1">
              CODEQL DATABASE · QUERY · AUDIT
            </span>
          </div>
          <span className="brand-tag">Pro</span>
        </div>
        <div className="flex-1" />
        <span className="chip" title="所有分析均在本机运行">
          <span className="w-1.5 h-1.5 rounded-full bg-ai-fp" />
          本地工作台
        </span>
        <span className="chip" title="审计工作台支持的结果格式">
          <FileCode2 size={10} /> SARIF v2.1.0 · CSV
        </span>
        <span className="chip" title="当前平台">
          {platformLabel}
        </span>
        <button
          className="btn btn-icon no-drag"
          onClick={onToggleTheme}
          title={theme === "dark" ? "切换到浅色主题" : "切换到深色主题"}
        >
          {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
        </button>
      </header>

      <main className="hub-main">
        <div className="hub-hero">
          <div className="hub-eyebrow">
            <Sparkles size={12} />
            CODEQL AUDIT WORKBENCH
          </div>
          <h1 className="hub-title">
            一个工作台，
            <span className="start-gradient">贯穿 CodeQL 审计全流程</span>
          </h1>
          <p className="hub-desc">
            CodeQLAuditPro 把数据库构建、查询分析、结果审计收进同一个桌面应用。
            结果支持 <b>SARIF v2.1.0</b> 与 CSV；载入后自动对应本地源码，
            并把完整污点传播路径、CWE 标签、指纹与规则元数据一并呈现给审计人员。
          </p>
          <div className="hub-flow">
            <span className="hub-flow-step">
              <FolderTree size={12} /> 源码
            </span>
            <span className="hub-flow-arrow">→</span>
            <span className="hub-flow-step">
              <Database size={12} /> 创建数据库
            </span>
            <span className="hub-flow-arrow">→</span>
            <span className="hub-flow-step">
              <PlayCircle size={12} /> 运行查询
            </span>
            <span className="hub-flow-arrow">→</span>
            <span className="hub-flow-step">
              <FileCode2 size={12} /> SARIF / CSV
            </span>
            <span className="hub-flow-arrow">→</span>
            <span className="hub-flow-step">
              <ScanSearch size={12} /> 源码审计
            </span>
          </div>
        </div>

        <div className="hub-tools">
          <article className="hub-card hub-card--assistant">
            <div className="hub-card-head">
              <div className="hub-icon hub-icon--assistant">
                <TerminalSquare size={24} strokeWidth={1.9} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="hub-card-kicker">WORKFLOW 01 · DATABASE &amp; QUERY</div>
                <h2 className="hub-card-title">CodeQL 运行器</h2>
              </div>
              <span className="badge hub-badge hub-badge--cyan">建库 / 查询</span>
            </div>
            <p className="hub-card-desc">
              可视化配置 CodeQL CLI，创建审计数据库、运行查询套件，并输出
              <b> SARIF v2.1.0</b> 或 CSV。所有参数都能一键复制成命令行，
              不依赖本工具也能独立执行。
            </p>
            <ul className="hub-feature-list">
              <li>
                <Database size={13} /> 图形化创建数据库（10 种语言，编译型语言可配构建命令）
              </li>
              <li>
                <PlayCircle size={13} /> 运行查询 / 套件，输出 SARIF v2.1.0 或 CSV
              </li>
              <li>
                <Gauge size={13} /> 内存、线程、附加参数与配置快照
              </li>
              <li>
                <History size={13} /> 实时控制台、强制停止、完整日志与运行历史
              </li>
            </ul>
            <div className="hub-recent hub-recent--assistant">
              {assistantRunning ? (
                <>
                  <Loader2 size={12} className="spin text-cyan flex-none" />
                  <span className="text-cyan">
                    {assistantStatus || "任务运行中"} · 切换页面不会中断
                  </span>
                </>
              ) : (
                <>
                  <TerminalSquare size={12} className="text-cyan flex-none" />
                  <span className="text-tertiary">分析完成后可直接带着结果进入审计工作台</span>
                </>
              )}
            </div>
            <button className="btn btn-outline hub-enter hub-enter--assistant" onClick={onOpenAssistant}>
              进入 CodeQL 运行器
              <ArrowRight size={14} />
            </button>
          </article>

          <article className="hub-card hub-card--audit">
            <div className="hub-card-head">
              <div className="hub-icon hub-icon--audit">
                <ShieldCheck size={24} strokeWidth={1.9} />
              </div>
              <div className="min-w-0 flex-1">
                <div className="hub-card-kicker">WORKFLOW 02 · RESULTS &amp; AUDIT</div>
                <h2 className="hub-card-title">审计工作台</h2>
              </div>
              <span className="badge hub-badge">结果审计</span>
            </div>
            <p className="hub-card-desc">
              载入 CodeQL 的 <b>SARIF v2.1.0</b> 或 CSV 结果，与本地源码逐行对应。
              三栏界面里完成「看列表 → 点告警 → 读源码 → 顺污点路径追到 source」的审计闭环。
            </p>
            <ul className="hub-feature-list">
              <li>
                <Workflow size={13} /> 完整 codeFlow 污点传播路径，逐节点跳转源码
              </li>
              <li>
                <Fingerprint size={13} /> CWE 标签、SARIF 指纹、precision 与 security-severity
              </li>
              <li>
                <ScanSearch size={13} /> 行列级高亮、跨文件证据链、路径映射与离线翻译
              </li>
              <li>
                <BrainCircuit size={13} /> AI 误报验证（单条 / 批量）与 32 列审计报告
              </li>
            </ul>
            {lastProject && (
              <div className="hub-recent" title={lastProject}>
                <FileSpreadsheet size={12} className="text-accent flex-none" />
                <span className="text-tertiary flex-none">上次结果</span>
                <span className="font-mono truncate">{shortPath(lastProject, 56)}</span>
              </div>
            )}
            <button className="btn btn-primary hub-enter" onClick={onOpenAudit}>
              进入审计工作台
              <ArrowRight size={14} />
            </button>
          </article>
        </div>

        <div className="hub-foot">
          <span className="hub-foot-item">
            <Layers size={11} />
            一个应用 · 两个工作流共享同一进程，切换页面不会中断任务
          </span>
          <span className="hub-foot-sep" />
          <span className="hub-foot-item">
            <Wand2 size={11} /> 结果与源码自动对应
          </span>
          <span className="hub-foot-sep" />
          <span>v{version}</span>
          <span className="hub-foot-sep" />
          <span>{platformLabel}</span>
        </div>
      </main>
    </div>
  );
}
