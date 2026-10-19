import {
  ArrowRight,
  FileSpreadsheet,
  FolderOpen,
  FolderTree,
  GitBranch,
  Languages,
  ScanSearch,
  ShieldCheck,
  Sparkles,
  Target,
} from "lucide-react";
import type { BootstrapDTO } from "../types";
import { shortPath } from "../lib/format";

export function StartScreen({
  boot,
  onOpenCSV,
  onOpenSrc,
  onLoadLast,
  loading,
}: {
  boot: BootstrapDTO | null;
  onOpenCSV: () => void;
  onOpenSrc: () => void;
  onLoadLast: () => void;
  loading: boolean;
}) {
  const recent = boot?.lastProject || "";
  return (
    <div className="start-screen">
      <div className="start-grid" />
      <div className="start-orb" style={{ background: "radial-gradient(circle, #6366f1, transparent 65%)", left: "12%", top: "-15%" }} />
      <div className="start-orb" style={{ background: "radial-gradient(circle, #22d3ee, transparent 65%)", right: "8%", bottom: "-22%", opacity: 0.28 }} />

      <div className="start-card">
        <div className="start-logo">
          <ShieldCheck size={32} strokeWidth={1.9} />
        </div>
        <h1 className="start-title">
          把 CodeQL 的原始输出
          <br />
          <span className="start-gradient">变成可点、可跳、可读的审计界面</span>
        </h1>
        <p className="start-desc">
          支持 SARIF v2.1.0 与 CSV：完整还原污点传播路径、CWE 标签、指纹与规则元数据，
          再把容器路径映射到本地源码。三栏工作台里完成
          「看列表 → 点漏洞 → 看高亮代码 → 顺着数据流追源头」的闭环。
        </p>

        <div className="start-actions">
          <button className="btn btn-primary" style={{ height: 36, padding: "0 16px" }} onClick={onOpenCSV}>
            <FolderOpen size={15} />
            打开 CodeQL 结果文件
          </button>
          <button className="btn btn-outline" style={{ height: 36, padding: "0 14px" }} onClick={onOpenSrc} disabled={loading}>
            <FolderTree size={15} />
            选择源码目录
          </button>
        </div>

        {recent && (
          <div className="start-recent">
            <div className="w-8 h-8 rounded-lg bg-accent-soft border border-accent flex items-center justify-center flex-none">
              <FileSpreadsheet size={15} className="text-accent" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-[10px] text-tertiary mb-0.5">最近打开</div>
              <div className="text-[11.5px] text-primary font-mono truncate" title={recent}>
                {shortPath(recent, 72)}
              </div>
            </div>
            <button className="btn btn-soft btn-sm" onClick={onLoadLast} disabled={loading}>
              继续审计
              <ArrowRight size={12} />
            </button>
          </div>
        )}

        <div className="start-features">
          <span className="chip">
            <FileSpreadsheet size={10} /> CSV
          </span>
          <span className="chip">
            <GitBranch size={10} /> SARIF v2.1.0 · codeFlow
          </span>
          <span className="chip">
            <ScanSearch size={10} /> 行列级高亮
          </span>
          <span className="chip">
            <Target size={10} /> 路径映射
          </span>
          <span className="chip">
            <Sparkles size={10} /> AI 误报验证
          </span>
          <span className="chip">
            <Languages size={10} /> 离线中文化
          </span>
        </div>

        <div className="mt-6 text-[10px] text-tertiary">
          支持 macOS · Windows · Linux　/　快捷键 {boot?.defaultSearchShortcut ?? "Cmd+F"} 在当前文件中查找
        </div>
      </div>
    </div>
  );
}
