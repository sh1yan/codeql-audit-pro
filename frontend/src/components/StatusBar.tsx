import {
  AlertCircle,
  Bug,
  CheckCircle2,
  EyeOff,
  FileCode2,
  Languages,
  Loader2,
  ShieldCheck,
  Sparkles,
  Target,
} from "lucide-react";
import type { ProjectDTO } from "../types";
import { cn } from "../lib/format";

export function StatusBar({
  project,
  message,
  tone,
  busy,
  version,
  platform,
}: {
  project: ProjectDTO | null;
  message: string;
  tone: "info" | "success" | "error";
  busy?: boolean;
  version: string;
  platform: string;
}) {
  const st = project?.stats;
  const TransIcon = st?.translationLLM ? Sparkles : Languages;
  return (
    <footer className="statusbar">
      <div className="flex items-center gap-1.5 min-w-0 truncate" style={{ maxWidth: "46vw" }}>
        {busy ? (
          <Loader2 size={11} className="spin flex-none text-accent" />
        ) : tone === "success" ? (
          <CheckCircle2 size={11} className="flex-none text-ai-fp" />
        ) : tone === "error" ? (
          <AlertCircle size={11} className="flex-none text-sev-error" />
        ) : (
          <ShieldCheck size={11} className="flex-none text-accent" />
        )}
        <span className={cn("truncate", tone === "error" ? "text-sev-error" : "text-secondary")}>
          {message || "就绪"}
        </span>
      </div>
      <div className="flex-1" />
      {st && (
        <>
          <span className="status-chip" title="漏洞总数 / 规则种类 / 涉及文件">
            <Bug size={10} className="text-sev-error" />
            <b>{st.total}</b> 漏洞
            <span className="opacity-40">·</span>
            <b>{st.ruleKinds}</b> 规则
            <span className="opacity-40">·</span>
            <FileCode2 size={10} />
            <b>{st.fileKinds}</b> 文件
          </span>
          <span className="status-chip" title="已定位到本地源码的比例">
            <Target size={10} className="text-cyan" />
            <b>{st.resolved}</b>
            <span className="opacity-50">/{st.total}</span> 已定位
          </span>
          <span className="status-chip" title="尚未查看的漏洞">
            <EyeOff size={10} className="text-blue" />
            <b>{st.unread}</b> 未读
          </span>
          <span className="status-chip" title="AI 误报验证结论分布">
            <Sparkles size={10} className="text-accent" />
            <b>{st.verified}</b> 已验证
            {st.verified > 0 && (
              <span className="opacity-60">
                （误报 {st.falsePositive} · 确认 {st.truePositive} · 存疑 {st.uncertain}）
              </span>
            )}
          </span>
          <span className="status-chip" title="翻译命中来源">
            <TransIcon size={10} className="text-ai-fp" />
            <b>{st.translationBuiltin + st.translationStore + st.translationLLM}</b> 命中
            {st.pending > 0 && <span className="text-sev-warning">· {st.pending} 待翻译</span>}
          </span>
        </>
      )}
      <span className="status-chip text-tertiary">
        {platform === "darwin" ? "macOS" : platform === "windows" ? "Windows" : "Linux"} · Wails v
        {version}
      </span>
    </footer>
  );
}
