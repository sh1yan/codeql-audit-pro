import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  Compartment,
  EditorState,
  StateEffect,
  StateField,
  type Extension,
  type Range,
} from "@codemirror/state";
import { defaultKeymap } from "@codemirror/commands";
import {
  Decoration,
  EditorView,
  drawSelection,
  highlightActiveLine,
  highlightActiveLineGutter,
  highlightSpecialChars,
  keymap,
  lineNumbers,
  type DecorationSet,
} from "@codemirror/view";
import { HighlightStyle, LanguageDescription, syntaxHighlighting } from "@codemirror/language";
import { languages } from "@codemirror/language-data";
import { tags } from "@lezer/highlight";
import {
  ArrowDown,
  ArrowUp,
  CaseSensitive,
  Check,
  Clipboard,
  Code2,
  ExternalLink,
  Search,
  TextSelect,
  X,
} from "lucide-react";
import type { FileDTO, LocationDTO, Theme } from "../types";
import { api } from "../lib/api";
import { cn } from "../lib/format";

export interface HighlightRange {
  startLine: number;
  startCol: number;
  endLine: number;
  endCol: number;
  kind: "sink" | "source";
  active: boolean;
  label?: string;
}

interface SearchMatch {
  line: number;
  from: number;
  to: number;
}

interface RulerTick {
  col: number;
  x: number;
}

interface RulerBand {
  x: number;
  width: number;
  label: string;
  kind: "sink" | "source";
}

interface DecoPayload {
  highlights: HighlightRange[];
  matches: SearchMatch[];
  current: number;
}

const setDeco = StateEffect.define<DecoPayload>();

const decoField = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(deco, tr) {
    deco = deco.map(tr.changes);
    for (const e of tr.effects) {
      if (e.is(setDeco)) deco = buildDeco(tr.state, e.value);
    }
    return deco;
  },
  provide: (f) => EditorView.decorations.from(f),
});

function buildDeco(state: EditorState, payload: DecoPayload): DecorationSet {
  const ranges: Range<Decoration>[] = [];
  const lineKinds = new Map<number, "sink" | "source">();
  for (const h of payload.highlights) {
    const start = Math.max(1, h.startLine);
    const end = Math.min(state.doc.lines, Math.max(start, h.endLine || start));
    for (let ln = start; ln <= end; ln++) {
      const prev = lineKinds.get(ln);
      if (h.kind === "sink" || !prev) lineKinds.set(ln, h.kind);
    }
  }
  for (const [ln, kind] of lineKinds) {
    ranges.push(
      Decoration.line({ class: kind === "sink" ? "cm-hl-sink-line" : "cm-hl-source-line" }).range(
        state.doc.line(ln).from,
      ),
    );
  }

  for (const h of payload.highlights) {
    const startLine = Math.max(1, h.startLine);
    const endLine = Math.min(state.doc.lines, Math.max(startLine, h.endLine || startLine));
    if (startLine > state.doc.lines) continue;
    for (let ln = startLine; ln <= endLine; ln++) {
      const line = state.doc.line(ln);
      if (!line.length && startLine === endLine) continue;
      const startCol = ln === startLine ? Math.max(1, h.startCol || 1) : 1;
      const endCol =
        ln === endLine
          ? Math.max(Math.max(1, h.startCol || 1), h.endCol || h.startCol || 1)
          : line.length + 1;
      if (startCol > line.length) continue;
      const from = line.from + Math.min(startCol - 1, line.length);
      const to = line.from + Math.min(Math.max(endCol, startCol), line.length);
      if (to <= from) continue;
      const kind = h.kind === "sink" ? "cm-hl-sink-span" : "cm-hl-source-span";
      ranges.push(
        Decoration.mark({
          class: cn(kind, h.active && "cm-hl-active"),
          attributes: h.label ? { title: h.label } : undefined,
        }).range(from, to),
      );
    }
  }

  for (let i = 0; i < payload.matches.length; i++) {
    const m = payload.matches[i];
    if (m.line < 1 || m.line > state.doc.lines) continue;
    const line = state.doc.line(m.line);
    const from = line.from + Math.min(m.from, line.length);
    const to = line.from + Math.min(m.to, line.length);
    if (to <= from) continue;
    ranges.push(
      Decoration.mark({
        class: i === payload.current ? "cm-search-current" : "cm-search-match",
      }).range(from, to),
    );
  }

  ranges.sort((a, b) => a.from - b.from || a.value.startSide - b.value.startSide);
  return Decoration.set(ranges, true);
}

const highlightStyle = HighlightStyle.define([
  { tag: tags.keyword, color: "var(--syntax-keyword)" },
  { tag: [tags.controlKeyword, tags.moduleKeyword, tags.operatorKeyword], color: "var(--syntax-keyword)" },
  { tag: [tags.string, tags.special(tags.string), tags.regexp], color: "var(--syntax-string)" },
  { tag: [tags.comment, tags.lineComment, tags.blockComment, tags.docComment], color: "var(--syntax-comment)", fontStyle: "italic" },
  { tag: [tags.number, tags.integer, tags.float], color: "var(--syntax-number)" },
  { tag: [tags.bool, tags.null, tags.atom], color: "var(--syntax-number)" },
  { tag: [tags.function(tags.variableName), tags.function(tags.propertyName), tags.definition(tags.function(tags.variableName))], color: "var(--syntax-function)" },
  { tag: [tags.typeName, tags.className, tags.namespace], color: "var(--syntax-type)" },
  { tag: [tags.propertyName, tags.attributeName], color: "var(--syntax-property)" },
  { tag: [tags.operator, tags.punctuation, tags.bracket], color: "var(--syntax-operator)" },
  { tag: [tags.variableName, tags.definition(tags.variableName)], color: "var(--syntax-variable)" },
  { tag: [tags.meta, tags.annotation], color: "var(--syntax-meta)" },
  { tag: [tags.heading, tags.link, tags.url], color: "var(--syntax-function)" },
  { tag: tags.escape, color: "var(--syntax-number)" },
  { tag: tags.invalid, color: "var(--sev-error)" },
]);

function themeExt(theme: Theme): Extension {
  return EditorView.theme(
    {
      "&": { backgroundColor: "transparent", color: "var(--text)", height: "100%" },
      "&.cm-focused": { outline: "none" },
      ".cm-content": { caretColor: "var(--accent-strong)" },
      ".cm-cursor, .cm-dropCursor": { borderLeftColor: "var(--accent-strong)" },
      ".cm-selectionBackground, &.cm-focused .cm-selectionBackground": {
        backgroundColor: "rgba(99, 102, 241, 0.28)",
      },
    },
    { dark: theme === "dark" },
  );
}

function findMatches(lines: string[], query: string, caseSensitive: boolean): SearchMatch[] {
  const q = query.trim();
  if (!q) return [];
  const needle = caseSensitive ? q : q.toLowerCase();
  const out: SearchMatch[] = [];
  const limit = 3000;
  for (let i = 0; i < lines.length && out.length < limit; i++) {
    const line = lines[i] ?? "";
    const hay = caseSensitive ? line : line.toLowerCase();
    let idx = 0;
    while (idx >= 0 && out.length < limit) {
      idx = hay.indexOf(needle, idx);
      if (idx < 0) break;
      out.push({ line: i + 1, from: idx, to: idx + q.length });
      idx += Math.max(1, q.length);
    }
  }
  return out;
}

export function CodeView({
  file,
  focusLine,
  highlights,
  activeHighlight,
  showRuler,
  theme,
  searchOpen,
  searchShortcut,
  onSearchClose,
  onCopyStatus,
  onOpenEditor,
}: {
  file: FileDTO;
  focusLine: number;
  highlights: HighlightRange[];
  activeHighlight: HighlightRange | null;
  showRuler: boolean;
  theme: Theme;
  searchOpen: boolean;
  searchShortcut: string;
  onSearchClose: () => void;
  onCopyStatus: (msg: string) => void;
  onOpenEditor: () => void;
}) {
  const hostRef = useRef<HTMLDivElement>(null);
  const rulerRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const themeCompartment = useRef(new Compartment());
  const langCompartment = useRef(new Compartment());
  const rulerUpdateRef = useRef<() => void>(() => {});
  const [query, setQuery] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [current, setCurrent] = useState(0);
  const [ticks, setTicks] = useState<RulerTick[]>([]);
  const [band, setBand] = useState<RulerBand | null>(null);
  const [menu, setMenu] = useState<{ x: number; y: number; hasSelection: boolean } | null>(null);

  const matches = useMemo(
    () => findMatches(file.lines, query, caseSensitive),
    [file.path, file.lines, query, caseSensitive],
  );

  useEffect(() => {
    setCurrent(0);
  }, [query, caseSensitive, file.path]);

  // 创建编辑器实例。
  useEffect(() => {
    if (!hostRef.current) return;
    const view = new EditorView({
      state: EditorState.create({
        doc: "",
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightActiveLine(),
          highlightSpecialChars(),
          drawSelection(),
          EditorState.readOnly.of(true),
          EditorView.editable.of(false),
          EditorView.contentAttributes.of({ spellcheck: "false", "aria-label": "源码内容" }),
          keymap.of(defaultKeymap),
          decoField,
          themeCompartment.current.of(themeExt(theme)),
          langCompartment.current.of([]),
          syntaxHighlighting(highlightStyle),
        ],
      }),
      parent: hostRef.current,
    });
    viewRef.current = view;
    const onScroll = () => rulerUpdateRef.current();
    view.scrollDOM.addEventListener("scroll", onScroll, { passive: true });
    const ro = new ResizeObserver(() => rulerUpdateRef.current());
    ro.observe(view.scrollDOM);
    return () => {
      view.scrollDOM.removeEventListener("scroll", onScroll);
      ro.disconnect();
      view.destroy();
      viewRef.current = null;
    };
  }, []);

  // 主题切换。
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({ effects: themeCompartment.current.reconfigure(themeExt(theme)) });
  }, [theme]);

  // 文件切换：重设文档，同时保留查找词并在新文件里重新查找。
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const state = EditorState.create({
      doc: file.lines.join("\n"),
      extensions: [
        lineNumbers(),
        highlightActiveLineGutter(),
        highlightActiveLine(),
        highlightSpecialChars(),
        drawSelection(),
        EditorState.readOnly.of(true),
        EditorView.editable.of(false),
        EditorView.contentAttributes.of({ spellcheck: "false", "aria-label": "源码内容" }),
        keymap.of(defaultKeymap),
        decoField,
        themeCompartment.current.of(themeExt(theme)),
        langCompartment.current.of([]),
        syntaxHighlighting(highlightStyle),
      ],
    });
    view.setState(state);
    view.dispatch({ effects: langCompartment.current.reconfigure([]) });
    let alive = true;
    (async () => {
      const desc =
        LanguageDescription.matchFilename(languages, file.displayPath || file.path) ??
        LanguageDescription.matchFilename(languages, file.path);
      if (!desc) return;
      try {
        const support = await desc.load();
        if (alive && viewRef.current === view) {
          view.dispatch({ effects: langCompartment.current.reconfigure(support) });
        }
      } catch {
        /* 语言包加载失败时保持纯文本 */
      }
    })();
    const raf = requestAnimationFrame(() => {
      const line = Math.max(1, Math.min(focusLine || 1, view.state.doc.lines));
      const pos = view.state.doc.line(line).from;
      view.dispatch({ effects: EditorView.scrollIntoView(pos, { y: "center" }) });
      rulerUpdateRef.current();
    });
    return () => {
      alive = false;
      cancelAnimationFrame(raf);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [file.path]);

  // 焦点行变化：滚到视口中央。
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    const raf = requestAnimationFrame(() => {
      const line = Math.max(1, Math.min(focusLine || 1, view.state.doc.lines));
      const pos = view.state.doc.line(line).from;
      view.dispatch({ effects: EditorView.scrollIntoView(pos, { y: "center" }) });
      rulerUpdateRef.current();
    });
    return () => cancelAnimationFrame(raf);
  }, [focusLine, file.path]);

  // 高亮 + 查找命中统一重建装饰。
  useEffect(() => {
    const view = viewRef.current;
    if (!view) return;
    view.dispatch({
      effects: setDeco.of({ highlights, matches, current }),
    });
  }, [highlights, matches, current]);

  // 当前命中滚动到视口中央。
  useEffect(() => {
    const view = viewRef.current;
    if (!view || matches.length === 0 || !query.trim()) return;
    const m = matches[Math.max(0, Math.min(current, matches.length - 1))];
    const line = view.state.doc.line(Math.max(1, Math.min(m.line, view.state.doc.lines)));
    const pos = line.from + Math.min(m.from, line.length);
    view.dispatch({ effects: EditorView.scrollIntoView(pos, { y: "center" }) });
  }, [current, matches, query]);

  // 打开查找条时聚焦输入框；关闭时清空查找词。
  useEffect(() => {
    if (searchOpen) {
      const timer = window.setTimeout(() => {
        searchRef.current?.focus();
        searchRef.current?.select();
      }, 40);
      return () => window.clearTimeout(timer);
    }
    setQuery("");
    setMenu(null);
  }, [searchOpen]);

  useEffect(() => {
    if (!menu) return;
    const close = () => setMenu(null);
    window.addEventListener("mousedown", close);
    window.addEventListener("scroll", close, true);
    return () => {
      window.removeEventListener("mousedown", close);
      window.removeEventListener("scroll", close, true);
    };
  }, [menu]);

  const updateRuler = useCallback(() => {
    const view = viewRef.current;
    const ruler = rulerRef.current;
    if (!showRuler || !view || !ruler) {
      setTicks([]);
      setBand(null);
      return;
    }
    const focus = activeHighlight ?? { startLine: focusLine, endLine: focusLine, startCol: 0, endCol: 0, kind: "sink" as const };
    const lineNo = Math.max(1, Math.min(focus.startLine || focusLine || 1, view.state.doc.lines));
    const line = view.state.doc.line(lineNo);
    let coords: { left: number; top: number } | null = null;
    try {
      coords = view.coordsAtPos(line.from);
    } catch {
      coords = null;
    }
    const rulerRect = ruler.getBoundingClientRect();
    if (!coords) {
      setTicks([]);
      setBand(null);
      return;
    }
    const charWidth = view.defaultCharacterWidth || 7.5;
    const textX = coords.left - rulerRect.left;
    const width = rulerRect.width;
    const nextTicks: RulerTick[] = [];
    const maxCol = Math.min(400, Math.floor((width - textX) / charWidth) + 1);
    for (let col = 10; col <= maxCol; col += 10) {
      nextTicks.push({ col, x: textX + (col - 1) * charWidth });
    }
    setTicks(nextTicks);
    if (focus.startCol && focus.startCol > 0) {
      const startCol = Math.max(1, focus.startCol);
      const endCol = Math.max(startCol, focus.endCol || startCol);
      const x1 = textX + (startCol - 1) * charWidth;
      const x2 = textX + endCol * charWidth;
      const x = Math.max(52, x1);
      const w = Math.max(3, Math.min(width - x - 4, x2 - x1));
      if (w > 2) {
        const label =
          endCol > startCol ? `列 ${startCol}-${endCol}` : `列 ${startCol}`;
        setBand({ x, width: w, label, kind: focus.kind });
      } else {
        setBand(null);
      }
    } else {
      setBand(null);
    }
  }, [activeHighlight, focusLine, showRuler]);

  rulerUpdateRef.current = updateRuler;

  useEffect(() => {
    const raf = requestAnimationFrame(updateRuler);
    return () => cancelAnimationFrame(raf);
  }, [updateRuler, file.path, highlights, theme]);

  const copy = async (text: string, lines: number) => {
    if (!text) return;
    try {
      await api.CopyText(text);
    } catch {
      try {
        await navigator.clipboard.writeText(text);
      } catch {
        /* ignore */
      }
    }
    onCopyStatus(lines > 1 ? `已复制 ${lines} 行代码到剪贴板` : "已复制代码到剪贴板");
  };

  const selectedText = () => {
    const view = viewRef.current;
    if (!view) return "";
    const sel = view.state.selection.main;
    if (sel.empty) return "";
    return view.state.sliceDoc(sel.from, sel.to);
  };

  const currentLineText = () => {
    const view = viewRef.current;
    if (!view) return "";
    const head = view.state.selection.main.head;
    return view.state.doc.lineAt(head).text;
  };

  const countLines = (text: string) => (text ? text.split("\n").length : 0);

  const closeSearch = () => {
    setQuery("");
    onSearchClose();
  };

  return (
    <div className="code-shell" onContextMenu={(e) => {
      e.preventDefault();
      const text = selectedText();
      setMenu({ x: e.clientX, y: e.clientY, hasSelection: !!text });
    }}>
      {showRuler && (
        <div className="column-ruler" ref={rulerRef} title="列标尺：每 10 列一个刻度，色带标出当前沉淀点/来源点的列范围">
          <div className="ruler-gutter">列</div>
          {ticks.map((t) => (
            <span key={t.col}>
              <span className="ruler-tick" style={{ left: t.x }} />
              <span className="ruler-label" style={{ left: t.x }}>
                {t.col}
              </span>
            </span>
          ))}
          {band && (
            <>
              <span className={cn("ruler-band", band.kind)} style={{ left: band.x, width: band.width }} />
              <span
                className={cn("ruler-band-label", band.kind)}
                style={{ left: Math.max(52, Math.min(band.x + band.width - 34, (rulerRef.current?.clientWidth ?? 800) - 58)) }}
              >
                {band.label}
              </span>
            </>
          )}
        </div>
      )}

      {searchOpen && (
        <div className="search-bar">
          <Search size={12} className="text-tertiary ml-1" />
          <input
            ref={searchRef}
            className="cm-search-input"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={`在文件中查找（${searchShortcut}）`}
            spellCheck={false}
            onKeyDown={(e) => {
              if (e.key === "Enter") {
                e.preventDefault();
                if (matches.length) {
                  setCurrent((c) => (e.shiftKey ? (c - 1 + matches.length) % matches.length : (c + 1) % matches.length));
                }
              } else if (e.key === "Escape") {
                e.preventDefault();
                closeSearch();
              }
            }}
          />
          <span className="text-[10px] text-tertiary whitespace-nowrap min-w-[56px] text-center">
            {query.trim()
              ? matches.length
                ? `第 ${current + 1} / ${matches.length} 处`
                : "无匹配"
              : "输入关键词"}
          </span>
          <button
            className={cn("btn btn-icon btn-sm", caseSensitive && "btn-soft")}
            onClick={() => setCaseSensitive((v) => !v)}
            title="区分大小写"
          >
            <CaseSensitive size={13} />
          </button>
          <button
            className="btn btn-icon btn-sm"
            onClick={() => {
              if (matches.length) setCurrent((c) => (c - 1 + matches.length) % matches.length);
            }}
            disabled={!matches.length}
            title="上一处（Shift+Enter）"
          >
            <ArrowUp size={13} />
          </button>
          <button
            className="btn btn-icon btn-sm"
            onClick={() => {
              if (matches.length) setCurrent((c) => (c + 1) % matches.length);
            }}
            disabled={!matches.length}
            title="下一处（Enter）"
          >
            <ArrowDown size={13} />
          </button>
          <button className="btn btn-icon btn-sm" onClick={closeSearch} title="关闭（Esc）">
            <X size={13} />
          </button>
        </div>
      )}

      <div className="cm-host" ref={hostRef} />

      {menu && (
        <div
          className="code-context-menu"
          style={{ left: Math.min(menu.x, window.innerWidth - 210), top: Math.min(menu.y, window.innerHeight - 190) }}
          onMouseDown={(e) => e.stopPropagation()}
        >
          <button
            className="menu-item"
            onClick={() => {
              const text = menu.hasSelection ? selectedText() : currentLineText();
              void copy(text, countLines(text));
              setMenu(null);
            }}
          >
            <Clipboard size={13} />
            复制选中代码
            <span className="ml-auto text-[9px] text-tertiary">
              {menu.hasSelection ? "选区" : "当前行"}
            </span>
          </button>
          <button
            className="menu-item"
            onClick={() => {
              const text = currentLineText();
              void copy(text, 1);
              setMenu(null);
            }}
          >
            <Code2 size={13} />
            复制当前行
          </button>
          <button
            className="menu-item"
            onClick={() => {
              const view = viewRef.current;
              if (view) {
                view.dispatch({ selection: { anchor: 0, head: view.state.doc.length } });
                view.focus();
              }
              setMenu(null);
            }}
          >
            <TextSelect size={13} />
            全选代码
            <span className="ml-auto kbd">⌘A</span>
          </button>
          <div className="menu-sep" />
          <button
            className="menu-item"
            onClick={() => {
              onOpenEditor();
              setMenu(null);
            }}
          >
            <ExternalLink size={13} />
            用编辑器打开
          </button>
          <div className="menu-sep" />
          <button
            className="menu-item"
            onClick={() => {
              const text = selectedText() || currentLineText();
              if (text) {
                navigator.clipboard?.writeText(text).catch(() => {});
                onCopyStatus("已复制");
              }
              setMenu(null);
            }}
          >
            <Check size={13} />
            复制并关闭
          </button>
        </div>
      )}
    </div>
  );
}
