import { useMemo } from "react";
import {
  AlertTriangle,
  ArrowLeft,
  Check,
  ChevronRight,
  Code2,
  ExternalLink,
  FileCode2,
  FolderSearch,
  Search,
  SlidersHorizontal,
} from "lucide-react";
import type { ActiveLocation, FileDTO, FindingDTO, Theme } from "../types";
import { CodeView, type HighlightRange } from "./CodeView";
import { EmptyPane, Spinner, WarningBox } from "./ui";
import { cn, fileBase, shortPath } from "../lib/format";

export function CodePanel({
  finding,
  file,
  fileError,
  fileLoading,
  active,
  focusLine,
  historyLength,
  showRuler,
  theme,
  searchOpen,
  searchShortcut,
  onBack,
  onOpenEditor,
  onReveal,
  onToggleRuler,
  onToggleSearch,
  onOpenMapping,
  onCopyStatus,
}: {
  finding: FindingDTO | null;
  file: FileDTO | null;
  fileError: string | null;
  fileLoading: boolean;
  active: ActiveLocation | null;
  focusLine: number;
  historyLength: number;
  showRuler: boolean;
  theme: Theme;
  searchOpen: boolean;
  searchShortcut: string;
  onBack: () => void;
  onOpenEditor: () => void;
  onReveal: () => void;
  onToggleRuler: () => void;
  onToggleSearch: () => void;
  onOpenMapping: () => void;
  onCopyStatus: (msg: string) => void;
}) {
  const highlights = useMemo<HighlightRange[]>(() => {
    if (!file || !finding) return [];
    const out: HighlightRange[] = [];
    const add = (loc: FindingDTO["sink"], kind: "sink" | "source", isActive: boolean) => {
      if (!loc.resolved || !loc.resolvedPath || loc.resolvedPath !== file.path) return;
      out.push({
        startLine: loc.startLine,
        startCol: loc.startCol,
        endLine: loc.endLine,
        endCol: loc.endCol,
        kind,
        active: isActive,
        label: `${kind === "sink" ? "沉淀点" : "来源"} · ${loc.displayPath}:${loc.startLine}`,
      });
    };
    add(finding.sink, "sink", active?.kind === "sink");
    finding.sources.forEach((s, i) => add(s, "source", active?.kind === "source" && active.index === i));
    // SARIF 的完整污点传播路径：每一步都在代码视图里标出来，当前步骤高亮。
    for (const cf of finding.codeFlows ?? []) {
      for (const step of cf.steps) {
        const loc = step.location;
        if (!loc.resolved || !loc.resolvedPath || loc.resolvedPath !== file.path) continue;
        out.push({
          startLine: loc.startLine,
          startCol: loc.startCol,
          endLine: loc.endLine,
          endCol: loc.endCol,
          kind: "source",
          active:
            active?.kind === "flow" &&
            active.flowIndex === cf.index &&
            active.stepIndex === step.index,
          label: `路径 ${cf.index} · 步骤 ${step.index} · ${loc.displayPath}:${loc.startLine}`,
        });
      }
    }
    if (active?.kind === "link" && active.loc.resolvedPath === file.path) {
      out.push({
        startLine: active.loc.startLine,
        startCol: active.loc.startCol,
        endLine: active.loc.endLine,
        endCol: active.loc.endCol,
        kind: "source",
        active: true,
        label: `链接位置 · ${active.loc.displayPath}:${active.loc.startLine}`,
      });
    }
    return out;
  }, [file, finding, active]);

  const activeHighlight = highlights.find((h) => h.active) ?? null;

  if (!finding) {
    return (
      <section className="pane pane-center">
        <EmptyPane
          icon={Code2}
          title="选择一条漏洞开始审计"
          desc="左侧点击漏洞后，这里会打开对应源码，并把沉淀点滚到视口中央。"
        />
      </section>
    );
  }

  if (fileLoading && !file) {
    return (
      <section className="pane pane-center">
        <div className="flex-1 flex flex-col p-4 gap-3">
          <div className="skeleton h-6 w-2/5" />
          <div className="skeleton flex-1" style={{ minHeight: 260 }} />
        </div>
      </section>
    );
  }

  if (fileError && !file) {
    return (
      <section className="pane pane-center">
        <div className="empty-pane">
          <div className="empty-icon" style={{ color: "var(--amber)" }}>
            <FolderSearch size={20} />
          </div>
          <div className="empty-title">无法打开源码文件</div>
          <div className="empty-desc whitespace-pre-wrap">{fileError}</div>
          <div className="flex items-center gap-2 mt-1">
            <button className="btn btn-soft" onClick={onOpenMapping}>
              <SlidersHorizontal size={13} />
              打开路径映射
            </button>
            <button className="btn btn-outline" onClick={onOpenEditor}>
              <ExternalLink size={13} />
              用编辑器打开
            </button>
          </div>
        </div>
      </section>
    );
  }

  if (!file) {
    return (
      <section className="pane pane-center">
        <EmptyPane
          icon={FileCode2}
          title="尚未打开文件"
          desc="这条漏洞的沉淀点还没有定位到本地源码，可尝试在路径映射里手工指定。"
        >
          <button className="btn btn-soft btn-sm mt-1" onClick={onOpenMapping}>
            打开路径映射
          </button>
        </EmptyPane>
      </section>
    );
  }

  const line = active?.loc.startLine || focusLine || 1;
  const outOfRange = line > file.totalLines;
  const col = active?.loc.startCol || 1;

  return (
    <section className="pane pane-center">
      <div className="code-head">
        <button
          className="btn btn-icon btn-sm"
          onClick={onBack}
          disabled={historyLength === 0}
          title={historyLength === 0 ? "没有可返回的跳转记录" : "返回上一次跳转"}
        >
          <ArrowLeft size={14} />
        </button>
        <div className="code-file">
          <div className="code-file-icon">
            <FileCode2 size={12} />
          </div>
          <div className="min-w-0">
            <div className="code-file-path" title={file.path}>
              {shortPath(file.displayPath || file.path, 78)}
            </div>
            <div className="code-file-sub">
              <span>{file.totalLines} 行</span>
              <span>编码 {file.encoding}</span>
              <span className="text-accent">
                第 {line} 行{active?.loc.columnRange ? ` · 列 ${active.loc.columnRange}` : ""}
              </span>
              {outOfRange && (
                <span className="text-sev-warning flex items-center gap-1">
                  <AlertTriangle size={9} /> 行号越界
                </span>
              )}
            </div>
          </div>
        </div>

        <button
          className={cn("btn btn-sm", searchOpen && "btn-soft")}
          onClick={onToggleSearch}
          title={`在当前文件中查找（${searchShortcut}）`}
        >
          <Search size={13} />
          <span className="hide-narrow">查找</span>
        </button>
        <button
          className={cn("btn btn-sm", showRuler && "btn-soft")}
          onClick={onToggleRuler}
          title="显示/隐藏列标尺"
        >
          {showRuler ? <Check size={12} /> : <ChevronRight size={12} />}
          <span className="hide-narrow">列标尺</span>
        </button>
        <button className="btn btn-sm" onClick={onReveal} title="在文件管理器中显示">
          <FolderSearch size={13} />
        </button>
        <button className="btn btn-outline btn-sm" onClick={onOpenEditor} title="用外部编辑器打开并定位">
          <ExternalLink size={13} />
          <span className="hide-narrow">编辑器打开</span>
        </button>
      </div>

      {fileError && (
        <div className="px-3 pt-2">
          <WarningBox error>{fileError}</WarningBox>
        </div>
      )}
      {outOfRange && (
        <div className="px-3 pt-2">
          <WarningBox>
            证据链行号 <b>{line}</b> 超出该文件总行数 <b>{file.totalLines}</b>
            （本地源码版本可能与生成数据库时不一致）。代码视图已定位到文件末尾。
          </WarningBox>
        </div>
      )}

      <CodeView
        file={file}
        focusLine={Math.min(line, file.totalLines || 1)}
        highlights={highlights}
        activeHighlight={activeHighlight}
        showRuler={showRuler}
        theme={theme}
        searchOpen={searchOpen}
        searchShortcut={searchShortcut}
        onSearchClose={onToggleSearch}
        onCopyStatus={onCopyStatus}
        onOpenEditor={() => onOpenEditorWithPosition(onOpenEditor, file, line, col)}
      />

      {fileLoading && (
        <div className="absolute top-12 right-3 z-10 chip bg-surface-3">
          <Spinner size={11} /> 正在读取文件…
        </div>
      )}

      <div className="flex items-center gap-2 px-3 py-1.5 border-t border-subtle text-[9.5px] text-tertiary bg-surface-2">
        <span className="flex items-center gap-1">
          <span className="w-2 h-2 rounded-sm" style={{ background: "var(--red)" }} /> 沉淀点
        </span>
        <span className="flex items-center gap-1">
          <span className="w-2 h-2 rounded-sm" style={{ background: "var(--amber)" }} /> 来源 / 路径节点
        </span>
        {finding.codeFlows?.length > 0 && (
          <span className="flex items-center gap-1 text-cyan" title="当前告警带完整 SARIF codeFlow">
            · {finding.codeFlows.length} 条污点路径
          </span>
        )}
        <span className="opacity-70">拖拽可跨行选中代码 · Cmd/Ctrl+C 复制 · Cmd/Ctrl+A 全选</span>
        <div className="flex-1" />
        <span className="font-mono">{fileBase(file.path)}</span>
      </div>
    </section>
  );
}

function onOpenEditorWithPosition(
  open: () => void,
  file: FileDTO,
  line: number,
  col: number,
) {
  // 由 App 的 onOpenEditor 读取当前 active/focus 状态；这里保留函数形式，
  // 便于以后把行列直接传给外部编辑器。
  void file;
  void line;
  void col;
  open();
}
