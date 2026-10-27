import {
  ArrowRight,
  FileSpreadsheet,
  FolderOpen,
  FolderTree,
  GitBranch,
  History,
  Languages,
  ScanSearch,
  ShieldCheck,
  Sparkles,
  Target,
  Trash2,
} from "lucide-react";
import type { BootstrapDTO, ProjectHistoryDTO } from "../types";
import { cn, fmtDateTime, shortPath } from "../lib/format";

export function StartScreen({
  boot,
  history,
  onOpenCSV,
  onOpenSrc,
  onOpenProject,
  onDeleteProject,
  onOpenHistory,
  loading,
}: {
  boot: BootstrapDTO | null;
  history: ProjectHistoryDTO[];
  onOpenCSV: () => void;
  onOpenSrc: () => void;
  onOpenProject: (item: ProjectHistoryDTO) => void;
  onDeleteProject: (item: ProjectHistoryDTO) => void;
  onOpenHistory: () => void;
  loading: boolean;
}) {
  return (
    <div className="start-screen">
      {/* 背景装饰单独裁剪，避免外溢的网格 / 光晕给启动页制造出可以滚动的空白。 */}
      <div className="start-decor" aria-hidden="true">
        <div className="start-grid" />
        <div className="start-orb" style={{ background: "radial-gradient(circle, #6366f1, transparent 65%)", left: "12%", top: "-15%" }} />
        <div className="start-orb" style={{ background: "radial-gradient(circle, #22d3ee, transparent 65%)", right: "8%", bottom: "-22%", opacity: 0.28 }} />
      </div>

      <div className="start-card start-card--wide">
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

        <section className="start-history">
          <div className="start-history-head">
            <div className="start-history-title">
              <History size={13} className="text-accent" />
              历史审计项目
              <span className="text-[10px] font-normal text-tertiary">
                记录已读标记、AI 判定与路径映射，可随时删除
              </span>
            </div>
            {history.length > 0 && (
              <button className="btn btn-sm text-tertiary" onClick={onOpenHistory}>
                管理 / 删除
              </button>
            )}
          </div>

          {history.length === 0 ? (
            <div className="start-history-empty">
              还没有历史项目。打开 CodeQL 结果文件并载入后，会自动记录在这里。
            </div>
          ) : (
            <div className="start-history-list">
              {history.slice(0, 6).map((item) => (
                <div key={item.csvPath} className={cn("start-history-row", item.active && "active")}>
                  <div className="start-history-icon">
                    <FileSpreadsheet size={14} />
                  </div>
                  <div className="min-w-0 flex-1">
                    <div className="start-history-name">
                      <span className="font-mono truncate" title={item.csvPath}>
                        {item.csvName || shortPath(item.csvPath, 56)}
                      </span>
                      {item.active && (
                        <span className="badge bg-accent-soft text-accent border-accent">当前</span>
                      )}
                      {!item.resultExists && (
                        <span className="badge bg-sev-error-soft text-sev-error border-sev-error/25">
                          结果文件缺失
                        </span>
                      )}
                      {item.formatLabel && <span className="badge tag-chip font-mono">{item.formatLabel}</span>}
                    </div>
                    <div className="start-history-meta" title={`${item.csvPath}\n${item.srcRoot}`}>
                      <span className="truncate">{shortPath(item.csvPath, 58)}</span>
                      <span className="sep">·</span>
                      <span>{item.totalFindings > 0 ? `${item.totalFindings} 条` : "条数未统计"}</span>
                      <span className="sep">·</span>
                      <span>已读 {item.reviewedCount}</span>
                      <span className="sep">·</span>
                      <span>AI 判定 {item.verdictCount}</span>
                      {item.lastOpened && (
                        <>
                          <span className="sep">·</span>
                          <span>{fmtDateTime(item.lastOpened)}</span>
                        </>
                      )}
                    </div>
                    <div className="start-history-meta" title={item.srcRoot}>
                      <FolderTree size={10} />
                      <span className="truncate">
                        {item.srcRoot ? shortPath(item.srcRoot, 62) : "未设置源码目录"}
                      </span>
                    </div>
                  </div>
                  <div className="start-history-actions">
                    <button
                      className="btn btn-soft btn-sm"
                      disabled={!item.resultExists || loading}
                      title={item.resultExists ? "载入这个历史项目" : "结果文件已不存在"}
                      onClick={() => onOpenProject(item)}
                    >
                      继续审计
                      <ArrowRight size={11} />
                    </button>
                    <button
                      className="btn btn-icon btn-sm"
                      title="删除这条历史记录"
                      onClick={() => onDeleteProject(item)}
                    >
                      <Trash2 size={12} />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}

          {history.length > 6 && (
            <div className="start-history-more">
              <button className="btn btn-sm" onClick={onOpenHistory}>
                查看全部 {history.length} 个历史项目
              </button>
            </div>
          )}
        </section>

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

        <div className="mt-5 text-[10px] text-tertiary">
          历史项目保存在本地 SQLite（history.db）　/　快捷键 {boot?.defaultSearchShortcut ?? "Cmd+F"} 在当前文件中查找
        </div>
      </div>
    </div>
  );
}
