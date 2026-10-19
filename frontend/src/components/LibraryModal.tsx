import { useEffect, useMemo, useState } from "react";
import {
  BookOpen,
  Check,
  Languages,
  PencilLine,
  Search,
  Sparkles,
  Tag,
  UserCheck,
} from "lucide-react";
import type { LibraryDTO, LibraryEntryDTO } from "../types";
import { api } from "../lib/api";
import { Modal, Spinner, WarningBox } from "./ui";
import { cn, fmtDateTime, shortPath } from "../lib/format";

const KINDS = [
  { value: "", label: "全部" },
  { value: "rule", label: "规则名称" },
  { value: "description", label: "规则描述" },
  { value: "message", label: "漏洞详情" },
  { value: "label", label: "证据链标签" },
];

export function LibraryModal({
  open,
  onClose,
  notify,
}: {
  open: boolean;
  onClose: () => void;
  notify: (kind: "info" | "success" | "error", message: string) => void;
}) {
  const [kind, setKind] = useState("");
  const [query, setQuery] = useState("");
  const [lib, setLib] = useState<LibraryDTO | null>(null);
  const [loading, setLoading] = useState(false);
  const [selected, setSelected] = useState<LibraryEntryDTO | null>(null);
  const [zh, setZh] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const reload = async (k = kind, q = query) => {
    setLoading(true);
    setError("");
    try {
      const res = await api.GetLibrary(k, q);
      setLib(res);
      if (selected && res.entries.some((e) => e.id === selected.id)) {
        // keep current selection
      } else if (res.entries.length > 0) {
        setSelected(res.entries[0]);
        setZh(res.entries[0].sourceZh);
      } else {
        setSelected(null);
        setZh("");
      }
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!open) return;
    setSelected(null);
    setZh("");
    void reload("", "");
    setKind("");
    setQuery("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const timer = window.setTimeout(() => void reload(kind, query), 220);
    return () => window.clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kind, query, open]);

  const save = async () => {
    if (!selected) return;
    if (!zh.trim()) {
      notify("info", "译文不能为空。");
      return;
    }
    setSaving(true);
    try {
      await api.SaveLibraryEntry(selected.kind, selected.sourceEn, zh);
      notify("success", "译文已保存；之后遇到同样内容会直接复用。");
      setSelected({ ...selected, sourceZh: zh, origin: "user", originZh: "人工校正" });
      await reload(kind, query);
    } catch (err) {
      setError(String(err));
    } finally {
      setSaving(false);
    }
  };

  const originIcon =
    selected?.origin === "builtin" ? Sparkles : selected?.origin === "llm" ? Languages : UserCheck;

  const stats = useMemo(() => {
    if (!lib) return null;
    return { total: lib.total, shown: lib.shown };
  }, [lib]);

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="翻译库"
      subtitle="中英对照沉淀在本地 SQLite，越用越全；支持人工纠错回写"
      icon={BookOpen}
      size="xl"
      footer={
        <>
          <div className="text-[10px] text-tertiary mr-auto">
            {stats ? `共 ${stats.total} 条 · 当前显示 ${stats.shown} 条（按命中次数排序）` : ""}
          </div>
          <button className="btn btn-outline" onClick={onClose}>
            关闭
          </button>
        </>
      }
    >
      {error && (
        <div className="mb-3">
          <WarningBox error>{error}</WarningBox>
        </div>
      )}
      <div className="flex items-center gap-2 mb-3">
        <div className="flex items-center gap-1">
          {KINDS.map((k) => (
            <button
              key={k.value}
              className={cn("chip", kind === k.value && "chip-active")}
              onClick={() => setKind(k.value)}
            >
              {k.label}
            </button>
          ))}
        </div>
        <div className="flex-1" />
        <div className="relative w-64">
          <Search size={12} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-tertiary" />
          <input
            className="input pl-8"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="搜索原文或译文"
            spellCheck={false}
          />
        </div>
      </div>

      <div className="library-layout">
        <div className="library-list">
          {loading && !lib ? (
            <div className="flex items-center justify-center gap-2 py-12 text-[11px] text-tertiary">
              <Spinner size={13} /> 正在读取翻译库…
            </div>
          ) : !lib || lib.entries.length === 0 ? (
            <div className="empty-pane" style={{ minHeight: 220 }}>
              <div className="empty-icon">
                <BookOpen size={18} />
              </div>
              <div className="empty-title">没有匹配的译文</div>
              <div className="empty-desc">换个关键词，或换一个类别看看。</div>
            </div>
          ) : (
            lib.entries.map((e) => {
              const Icon = e.origin === "builtin" ? Sparkles : e.origin === "llm" ? Languages : UserCheck;
              return (
                <button
                  key={e.id}
                  className={cn("library-item", selected?.id === e.id && "active")}
                  onClick={() => {
                    setSelected(e);
                    setZh(e.sourceZh);
                  }}
                >
                  <div className="flex items-center gap-1.5">
                    <span className="badge bg-surface-3 text-tertiary border-subtle">
                      <Icon size={8} />
                      {e.kindZh}
                    </span>
                    <span className="badge bg-surface-3 text-tertiary border-subtle">
                      {e.originZh}
                    </span>
                    <span className="text-[9px] text-tertiary ml-auto">命中 {e.hits}</span>
                  </div>
                  <div className="library-en mt-1" title={e.sourceEn}>
                    {e.sourceEn}
                  </div>
                  <div className="library-zh" title={e.sourceZh}>
                    {e.sourceZh}
                  </div>
                </button>
              );
            })
          )}
        </div>

        <div className="rounded-lg border border-subtle bg-surface-2 p-3 flex flex-col gap-2 min-h-0">
          {selected ? (
            <>
              <div className="flex items-center gap-1.5">
                <Tag size={11} className="text-accent" />
                <span className="text-[10px] font-semibold text-primary">{selected.kindZh}</span>
                <span className="badge bg-surface-3 text-tertiary border-subtle ml-auto">
                  {selected.originZh}
                </span>
              </div>
              <div className="text-[9.5px] text-tertiary">英文原文</div>
              <div className="font-mono text-[10.5px] text-secondary bg-surface rounded-md border border-subtle p-2 max-h-32 overflow-auto whitespace-pre-wrap break-words select-text">
                {selected.sourceEn}
              </div>
              <div className="text-[9.5px] text-tertiary mt-1">中文译文（可修改）</div>
              <textarea
                className="textarea flex-1"
                value={zh}
                onChange={(e) => setZh(e.target.value)}
                spellCheck={false}
              />
              <div className="flex items-center gap-1.5 text-[9px] text-tertiary">
                <PencilLine size={10} />
                提示：漏洞详情模板里的 {"{{LINK}}"} 占位符要保留，否则证据链会退化成文末列表。
              </div>
              <div className="text-[9px] text-tertiary">最后更新：{fmtDateTime(selected.updatedAt)}</div>
              <div className="flex gap-2 mt-auto">
                <button className="btn btn-primary btn-sm flex-1" onClick={save} disabled={saving}>
                  {saving ? <Spinner size={11} /> : <Check size={12} />}
                  保存译文
                </button>
                <button
                  className="btn btn-outline btn-sm"
                  onClick={() => {
                    setZh(selected.sourceZh);
                  }}
                >
                  还原
                </button>
              </div>
            </>
          ) : (
            <div className="flex-1 flex flex-col items-center justify-center text-center gap-2 py-8">
              <div className="empty-icon" style={{ width: 38, height: 38 }}>
                <BookOpen size={16} />
              </div>
              <div className="empty-title">选择一条译文</div>
              <div className="empty-desc">在左侧点击任意条目，即可查看原文并修改中文译文。</div>
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
}
