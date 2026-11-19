import { useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import {
  AlertTriangle,
  Filter,
  RotateCcw,
  Search,
  Sparkles,
  X,
  CheckCheck,
} from "lucide-react";
import type { FindingDTO, ProjectDTO } from "../types";
import { cn, severityMeta, verdictMeta } from "../lib/format";
import { Toggle, Tooltip } from "./ui";

export interface FindingsFilter {
  keyword: string;
  severity: string;
  rule: string;
  file: string;
  verdict: string;
  onlyUnread: boolean;
  onlyUnresolved: boolean;
}

export const EMPTY_FILTER: FindingsFilter = {
  keyword: "",
  severity: "",
  rule: "",
  file: "",
  verdict: "",
  onlyUnread: false,
  onlyUnresolved: false,
};

/** 列表行高（px）。必须与 index.css 里 .finding-row 的高度保持一致。 */
export const ROW_HEIGHT = 78;

const AI_OPTIONS = [
  { value: "", label: "全部 AI 判定" },
  { value: "false_positive", label: "AI 误报" },
  { value: "true_positive", label: "AI 确认" },
  { value: "uncertain", label: "AI 存疑" },
  { value: "__none__", label: "未验证" },
];

export function Sidebar({
  project,
  findings,
  selectedKey,
  filter,
  onFilterChange,
  onSelect,
  onMarkAllRead,
  onResetRead,
  onOpenMapping,
}: {
  project: ProjectDTO;
  findings: FindingDTO[];
  selectedKey: string | null;
  filter: FindingsFilter;
  onFilterChange: (next: FindingsFilter) => void;
  onSelect: (finding: FindingDTO) => void;
  onMarkAllRead: () => void;
  onResetRead: () => void;
  onOpenMapping: () => void;
}) {
  const [advanced, setAdvanced] = useState(false);
  const listRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: findings.length,
    getScrollElement: () => listRef.current,
    // 行高在 CSS 里被写死成 ROW_HEIGHT（标签单行、超出渐隐），
    // 所以 estimateSize 与真实高度完全一致：虚拟列表不需要动态测量，
    // 也就不会出现「估算高度 < 实际高度」导致的行重叠（重影），
    // 以及「估算高度 > 实际高度」导致的滚动条虚长、下拉出现大片空白。
    // 仍然按漏洞 key 缓存行位置：筛选后同一个漏洞换了下标也不会错位。
    getItemKey: (index) => findings[index]?.key ?? index,
    estimateSize: () => ROW_HEIGHT,
    overscan: 10,
  });

  const activeFilterCount = useMemo(() => {
    let n = 0;
    if (filter.severity) n++;
    if (filter.rule) n++;
    if (filter.file) n++;
    if (filter.verdict) n++;
    if (filter.onlyUnread) n++;
    if (filter.onlyUnresolved) n++;
    return n;
  }, [filter]);

  useEffect(() => {
    if (!selectedKey) return;
    const index = findings.findIndex((f) => f.key === selectedKey);
    if (index >= 0) virtualizer.scrollToIndex(index, { align: "auto" });
  }, [selectedKey, findings, virtualizer]);

  const stats = project.stats;
  const sevBar = project.severities;

  return (
    <aside className="pane pane-left" style={{ width: "100%" }}>
      <div className="sidebar-head">
        {/* 项目摘要 */}
        <div className="project-card">
          <div className="flex items-start gap-2">
            <div className="w-7 h-7 rounded-lg bg-accent-soft border border-accent flex items-center justify-center flex-none">
              <Sparkles size={13} className="text-accent" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="text-[12px] font-semibold text-primary truncate" title={project.csvPath}>
                {project.csvName}
              </div>
              <div className="text-[9.5px] text-tertiary truncate mt-0.5">
                {project.srcRoot ? project.srcRoot : "未设置源码目录"}
              </div>
            </div>
            <div className="flex flex-col items-end gap-1 flex-none">
              <span className="badge bg-sev-error-soft text-sev-error border-sev-error/25">
                {stats.total} 条
              </span>
              <span
                className="badge bg-accent-soft text-accent border-accent font-mono"
                title={project.toolName ? `${project.toolName} ${project.toolVersion}` : undefined}
              >
                {project.formatLabel || "CSV"}
              </span>
            </div>
          </div>

          <div className="sev-bar mt-2.5">
            {sevBar.map((s) => (
              <span
                key={s.value}
                className={severityMeta(s.value).bar}
                style={{ width: `${stats.total ? (s.count / stats.total) * 100 : 0}%` }}
                title={`${s.label} ${s.count} 条`}
              />
            ))}
          </div>

          <div className="stat-grid">
            <div className="stat-cell">
              <b>{stats.unread}</b>
              <span>未读</span>
            </div>
            <div className="stat-cell">
              <b>{stats.resolved}</b>
              <span>已定位</span>
            </div>
            <div className="stat-cell">
              <b>{stats.verified}</b>
              <span>AI 已验证</span>
            </div>
          </div>
        </div>

        {/* 搜索 + 高级筛选入口 */}
        <div className="flex items-center gap-1.5">
          <div className="relative flex-1 min-w-0">
            <Search size={13} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-tertiary pointer-events-none" />
            <input
              className="input pl-8 pr-8"
              value={filter.keyword}
              onChange={(e) => onFilterChange({ ...filter, keyword: e.target.value })}
              placeholder="搜索规则 / 文件 / 消息 / CWE / 指纹"
              spellCheck={false}
            />
            {filter.keyword && (
              <button
                className="absolute right-1.5 top-1/2 -translate-y-1/2 btn btn-icon btn-sm"
                style={{ width: 22, height: 22 }}
                onClick={() => onFilterChange({ ...filter, keyword: "" })}
                title="清空搜索"
              >
                <X size={11} />
              </button>
            )}
          </div>
          <button
            className={cn("btn btn-icon relative flex-none", (advanced || activeFilterCount > 0) && "btn-soft")}
            style={{ height: 32, width: 32 }}
            onClick={() => setAdvanced((v) => !v)}
            title="更多筛选（规则 / 文件 / AI 判定 / 已读 / 未定位）"
          >
            <Filter size={13} />
            {activeFilterCount > 0 && (
              <span
                className="absolute -top-1 -right-1 w-4 h-4 rounded-full bg-accent text-white text-[8.5px] grid place-items-center border border-[var(--panel)]"
              >
                {activeFilterCount}
              </span>
            )}
          </button>
        </div>

        {/* 快速筛选 */}
        <div className="filter-row">
          <button
            className={cn("chip", filter.severity === "" && "chip-active")}
            onClick={() => onFilterChange({ ...filter, severity: "" })}
          >
            全部 <span className="opacity-60">{stats.total}</span>
          </button>
          {project.severities.map((s) => {
            const meta = severityMeta(s.value);
            return (
              <button
                key={s.value}
                className={cn("chip", filter.severity === s.value && "chip-active")}
                onClick={() =>
                  onFilterChange({
                    ...filter,
                    severity: filter.severity === s.value ? "" : s.value,
                  })
                }
              >
                <span className={cn("chip-dot", meta.dot)} />
                {s.label} <span className="opacity-60">{s.count}</span>
              </button>
            );
          })}
        </div>

        {advanced && (
          <div
            className="rounded-lg border border-subtle bg-surface-2 p-2.5 flex flex-col gap-2"
            style={{ animation: "slide-down 130ms ease" }}
          >
            <div className="grid grid-cols-2 gap-2">
              <label className="flex flex-col gap-1">
                <span className="text-[9.5px] text-tertiary">规则</span>
                <select
                  className="select"
                  value={filter.rule}
                  onChange={(e) => onFilterChange({ ...filter, rule: e.target.value })}
                >
                  <option value="">全部规则</option>
                  {project.rules.map((r) => (
                    <option key={r.value} value={r.value}>
                      {r.label}（{r.count}）
                    </option>
                  ))}
                </select>
              </label>
              <label className="flex flex-col gap-1">
                <span className="text-[9.5px] text-tertiary">文件</span>
                <select
                  className="select"
                  value={filter.file}
                  onChange={(e) => onFilterChange({ ...filter, file: e.target.value })}
                >
                  <option value="">全部文件</option>
                  {project.files.map((f) => (
                    <option key={f.value} value={f.value}>
                      {f.label}（{f.count}）
                    </option>
                  ))}
                </select>
              </label>
            </div>
            <label className="flex flex-col gap-1">
              <span className="text-[9.5px] text-tertiary">AI 判定</span>
              <select
                className="select"
                value={filter.verdict}
                onChange={(e) => onFilterChange({ ...filter, verdict: e.target.value })}
              >
                {AI_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </label>
            <div className="flex items-center justify-between gap-2 pt-0.5">
              <Toggle
                checked={filter.onlyUnread}
                onChange={(v) => onFilterChange({ ...filter, onlyUnread: v })}
                label="只看未读"
              />
              <Toggle
                checked={filter.onlyUnresolved}
                onChange={(v) => onFilterChange({ ...filter, onlyUnresolved: v })}
                label="查看未定位"
              />
            </div>
            {activeFilterCount > 0 && (
              <button
                className="btn btn-sm self-start text-tertiary"
                onClick={() => onFilterChange({ ...EMPTY_FILTER, keyword: filter.keyword })}
              >
                <RotateCcw size={11} /> 重置筛选
              </button>
            )}
          </div>
        )}
      </div>

      {/* 列表 */}
      <div className="finding-scroll" ref={listRef}>
        {findings.length === 0 ? (
          <div className="empty-pane" style={{ minHeight: 220 }}>
            <div className="empty-icon">
              <Filter size={18} />
            </div>
            <div className="empty-title">没有匹配的漏洞</div>
            <div className="empty-desc">调整搜索关键词或筛选条件，或查看未定位的条目。</div>
          </div>
        ) : (
          <div style={{ height: virtualizer.getTotalSize(), position: "relative" }}>
            {virtualizer.getVirtualItems().map((item) => {
              const f = findings[item.index];
              return (
                <div
                  key={f.key}
                  data-index={item.index}
                  style={{
                    position: "absolute",
                    top: 0,
                    left: 0,
                    width: "100%",
                    height: ROW_HEIGHT,
                    // 行高固定：直接把行摆到绝对位置上，不需要动态测量回调。
                    transform: `translateY(${item.start}px)`,
                  }}
                >
                  <FindingRow
                    finding={f}
                    selected={f.key === selectedKey}
                    onSelect={() => onSelect(f)}
                  />
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* 底栏 */}
      <div className="sidebar-foot">
        <div className="text-[10px] text-tertiary whitespace-nowrap">
          显示 <b className="text-secondary">{findings.length}</b> / {stats.total}
        </div>
        <div className="flex-1" />
        <Tooltip tip="把当前筛选结果全部标为已读">
          <button className="btn btn-icon btn-sm" onClick={onMarkAllRead} disabled={findings.length === 0} title="把当前筛选结果全部标为已读">
            <CheckCheck size={13} />
          </button>
        </Tooltip>
        <Tooltip tip="清空当前项目的已读标记">
          <button className="btn btn-icon btn-sm" onClick={onResetRead} title="清空当前项目的已读标记">
            <RotateCcw size={12} />
          </button>
        </Tooltip>
        <button className="btn btn-sm text-tertiary" onClick={onOpenMapping}>
          路径映射
        </button>
      </div>
    </aside>
  );
}

function FindingRow({
  finding,
  selected,
  onSelect,
}: {
  finding: FindingDTO;
  selected: boolean;
  onSelect: () => void;
}) {
  const sev = severityMeta(finding.severity);
  const v = verdictMeta(finding.ai?.verdict);
  const cwes = finding.cwes ?? [];
  const pathCount = finding.codeFlows?.length ?? 0;

  // 徽标在窄侧栏里必须保持可读：等级与 AI 研判排在最前，
  // 其余标签放后面，放不下的部分由 CSS 渐隐裁掉（见 .finding-tags）。
  const tagSummary = [
    `等级：${finding.severityZh || sev.label}`,
    finding.ai ? `AI 研判：${finding.ai.verdictZh || v?.label || finding.ai.verdict}` : "AI 研判：未验证",
    cwes.length > 0 ? `CWE：${cwes.join(", ")}` : "",
    pathCount > 0 ? `${pathCount} 条污点路径` : "",
    finding.sink.resolved ? "" : "未定位",
    finding.translatePending ? "待翻译" : "",
  ]
    .filter(Boolean)
    .join("\n");

  return (
    <button
      className={cn("finding-row", selected && "selected", finding.reviewed && "read")}
      onClick={onSelect}
      title={`${finding.ruleDisplay}\n${finding.sink.file}:${finding.sink.startLine}\n\n${tagSummary}`}
      aria-current={selected}
    >
      <span className={cn("finding-sev", sev.bar)} />
      <span className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5">
          {!finding.reviewed && <span className="unread-dot" />}
          <span className="finding-title flex-1">{finding.ruleDisplay}</span>
          <span className="text-[9px] text-tertiary font-mono flex-none">#{finding.index}</span>
        </div>
        <div className="finding-meta" title={finding.sink.file}>
          {finding.sink.file}:{finding.sink.startLine}
          {finding.reviewed && <span className="opacity-60"> · 已读</span>}
        </div>
        <div className="finding-tags">
          <span className="badge" data-tone={sev.key} title={`漏洞等级：${finding.severityZh || sev.label}`}>
            {finding.severityZh || sev.label}
          </span>
          {finding.ai && (
            <span
              className="badge"
              data-tone={finding.ai.verdict}
              title={finding.ai.reason || `AI 研判：${finding.ai.verdictZh || v?.label || "未知"}`}
            >
              <Sparkles size={8} />
              {finding.ai.verdictZh || v?.label || finding.ai.verdict}
            </span>
          )}
          {!finding.sink.resolved && (
            <span className="badge" data-tone="muted" title="源码里没找到这个文件或行号">
              <AlertTriangle size={8} /> 未定位
            </span>
          )}
          {pathCount > 0 && (
            <span className="badge" data-tone="info" title="完整污点传播路径">
              {pathCount} 路径
            </span>
          )}
          {cwes.slice(0, 2).map((cwe) => (
            <span key={cwe} className="badge" data-tone="muted" title={cwes.join(", ")}>
              {cwe}
            </span>
          ))}
          {finding.translatePending && (
            <span className="badge" data-tone="warning">
              待翻译
            </span>
          )}
        </div>
      </span>
    </button>
  );
}
