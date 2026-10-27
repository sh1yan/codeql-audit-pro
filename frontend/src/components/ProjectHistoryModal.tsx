import {
  AlertTriangle,
  ArrowRight,
  Database,
  FolderTree,
  History,
  RefreshCw,
  Trash2,
} from "lucide-react";
import type { ProjectHistoryDTO } from "../types";
import { Modal, WarningBox } from "./ui";
import { cn, fmtDateTime, shortPath } from "../lib/format";

function humanSize(bytes: number): string {
  if (!bytes || bytes <= 0) return "";
  const units = ["B", "KB", "MB", "GB"];
  let value = bytes;
  let unit = 0;
  while (value >= 1024 && unit < units.length - 1) {
    value /= 1024;
    unit++;
  }
  return `${value >= 10 || unit === 0 ? value.toFixed(0) : value.toFixed(1)} ${units[unit]}`;
}

export function ProjectHistoryModal({
  open,
  onClose,
  projects,
  activeCsvPath,
  onOpen,
  onDelete,
  onClear,
  onRefresh,
}: {
  open: boolean;
  onClose: () => void;
  projects: ProjectHistoryDTO[];
  activeCsvPath: string;
  onOpen: (item: ProjectHistoryDTO) => void;
  onDelete: (item: ProjectHistoryDTO) => void;
  onClear: () => void;
  onRefresh: () => void;
}) {
  const active = projects.find(
    (p) => activeCsvPath && p.csvPath.toLowerCase() === activeCsvPath.toLowerCase(),
  );
  const missing = projects.filter((p) => !p.resultExists).length;

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="历史审计项目"
      subtitle="记录保存在本地 SQLite（history.db）；删除只移除审计记录，不会删除结果文件、源码或翻译库"
      icon={History}
      size="xl"
      footer={
        <>
          <div className="text-[10px] text-tertiary mr-auto">
            共 {projects.length} 个历史项目
            {missing > 0 ? ` · ${missing} 个结果文件已缺失` : ""}
            {active ? ` · 当前：${active.csvName}` : ""}
          </div>
          <button className="btn btn-outline" onClick={onRefresh} title="重新读取 history.db">
            <RefreshCw size={12} /> 刷新
          </button>
          <button
            className="btn btn-danger"
            onClick={onClear}
            disabled={projects.length === 0}
            title="清空全部历史项目记录"
          >
            <Trash2 size={12} /> 清空全部
          </button>
          <button className="btn btn-outline" onClick={onClose}>
            关闭
          </button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <WarningBox>
          删除历史项目会同时删除该项目的<strong>已读标记、AI 误报判定、路径前缀规则与手工映射</strong>。
          结果文件、源码目录和全局翻译库不会受影响。
        </WarningBox>

        {projects.length === 0 ? (
          <div className="empty-pane" style={{ minHeight: 240 }}>
            <div className="empty-icon">
              <Database size={18} />
            </div>
            <div className="empty-title">还没有历史项目</div>
            <div className="empty-desc">
              打开一个 CodeQL 结果文件（CSV / SARIF）后，项目会自动记录在这里。
            </div>
          </div>
        ) : (
          <div className="history-table-wrap">
            <table className="data-table history-table">
              <thead>
                <tr>
                  <th>项目 / 结果文件</th>
                  <th>源码目录</th>
                  <th>审计进度</th>
                  <th>最近打开</th>
                  <th style={{ width: 132 }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {projects.map((item) => {
                  const isActive =
                    activeCsvPath && item.csvPath.toLowerCase() === activeCsvPath.toLowerCase();
                  return (
                    <tr key={item.csvPath} className={cn(isActive && "history-row-active")}>
                      <td>
                        <div className="flex items-center gap-1.5 min-w-0">
                          <span className="history-file font-mono truncate" title={item.csvPath}>
                            {shortPath(item.csvPath, 58)}
                          </span>
                          {isActive && (
                            <span className="badge bg-accent-soft text-accent border-accent flex-none">
                              当前
                            </span>
                          )}
                          {item.formatLabel && (
                            <span className="badge tag-chip font-mono flex-none">{item.formatLabel}</span>
                          )}
                        </div>
                        <div className="history-sub">
                          {item.csvName}
                          {item.resultSize > 0 ? ` · ${humanSize(item.resultSize)}` : ""}
                          {item.toolName ? ` · ${item.toolName}` : ""}
                        </div>
                        {!item.resultExists && (
                          <div className="history-missing">
                            <AlertTriangle size={10} /> 结果文件已不存在，无法继续载入
                          </div>
                        )}
                      </td>
                      <td>
                        <div className="flex items-center gap-1.5 min-w-0">
                          <FolderTree size={11} className="text-cyan flex-none" />
                          <span className="font-mono text-[10.5px] truncate" title={item.srcRoot}>
                            {item.srcRoot ? shortPath(item.srcRoot, 42) : "未设置"}
                          </span>
                        </div>
                        {!item.sourceExists && item.srcRoot && (
                          <div className="history-missing">源码目录不可用</div>
                        )}
                      </td>
                      <td>
                        <div className="font-mono text-[11px] text-primary">
                          {item.totalFindings > 0
                            ? `${item.resolvedFindings}/${item.totalFindings} 已定位`
                            : "未统计"}
                        </div>
                        <div className="history-sub">
                          已读 {item.reviewedCount} · AI 判定 {item.verdictCount}
                        </div>
                      </td>
                      <td className="font-mono text-[10.5px] text-secondary whitespace-nowrap">
                        {item.lastOpened ? fmtDateTime(item.lastOpened) : "—"}
                      </td>
                      <td>
                        <div className="flex items-center gap-1 justify-end">
                          <button
                            className="btn btn-soft btn-sm"
                            disabled={!item.resultExists}
                            title={item.resultExists ? "载入这个项目" : "结果文件已不存在"}
                            onClick={() => onOpen(item)}
                          >
                            载入 <ArrowRight size={11} />
                          </button>
                          <button
                            className="btn btn-icon btn-sm"
                            style={{ color: "var(--red)" }}
                            title="删除这条历史记录"
                            onClick={() => onDelete(item)}
                          >
                            <Trash2 size={12} />
                          </button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Modal>
  );
}
