import { useEffect, useState } from "react";
import {
  AlertTriangle,
  CheckCircle2,
  FileSearch,
  FolderSearch,
  Link2,
  Plus,
  RefreshCw,
  Route,
  Trash2,
  X,
} from "lucide-react";
import type { MappingDTO } from "../types";
import { api } from "../lib/api";
import { Modal, Spinner, WarningBox } from "./ui";
import { cn, pct, shortPath } from "../lib/format";

export function MappingModal({
  open,
  onClose,
  onProjectChanged,
  notify,
}: {
  open: boolean;
  onClose: () => void;
  onProjectChanged: () => Promise<void>;
  notify: (kind: "info" | "success" | "error", message: string) => void;
}) {
  const [mapping, setMapping] = useState<MappingDTO | null>(null);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState<string | null>(null);
  const [error, setError] = useState("");
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");

  const reload = async () => {
    setLoading(true);
    setError("");
    try {
      setMapping(await api.GetMapping());
    } catch (err) {
      setError(String(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (open) {
      void reload();
      setFrom("");
      setTo("");
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const afterChange = async (m: MappingDTO, msg: string) => {
    setMapping(m);
    notify("success", msg);
    await onProjectChanged();
  };

  const pickOverride = async (file: string) => {
    setBusy(file);
    try {
      const m = await api.ChooseOverrideFile(file);
      await afterChange(m, `已为 ${shortPath(file, 40)} 指定本地文件`);
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(null);
    }
  };

  const addRule = async () => {
    if (!from.trim() || !to.trim()) {
      notify("info", "请同时填写 CSV 路径前缀与本地目录。");
      return;
    }
    setBusy("add");
    try {
      const m = await api.AddPrefixRule(from.trim(), to.trim());
      setFrom("");
      setTo("");
      await afterChange(m, "已添加前缀规则并重新定位");
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(null);
    }
  };

  const removeRule = async (index: number) => {
    setBusy(`rule-${index}`);
    try {
      const m = await api.RemovePrefixRule(index);
      await afterChange(m, "已删除前缀规则");
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(null);
    }
  };

  const removeOverride = async (file: string) => {
    setBusy(`ov-${file}`);
    try {
      const m = await api.RemoveOverride(file);
      await afterChange(m, "已删除手工映射");
    } catch (err) {
      setError(String(err));
    } finally {
      setBusy(null);
    }
  };

  const resolvedPct = mapping ? pct(mapping.resolved, mapping.totalFindings) : 0;
  const overrides = mapping ? Object.entries(mapping.overrides) : [];

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="路径映射"
      subtitle="把 CodeQL 容器里的路径映射到本机源码，跳转才能真正落到文件上"
      icon={Route}
      size="lg"
      footer={
        <>
          <button className="btn btn-outline" onClick={() => void reload()} disabled={loading}>
            {loading ? <Spinner size={12} /> : <RefreshCw size={12} />}
            重新扫描
          </button>
          <div className="flex-1" />
          <button className="btn btn-primary" onClick={onClose}>
            完成
          </button>
        </>
      }
    >
      {error && (
        <div className="mb-3">
          <WarningBox error>{error}</WarningBox>
        </div>
      )}
      {loading && !mapping ? (
        <div className="flex items-center gap-2 text-[11px] text-secondary py-10 justify-center">
          <Spinner size={14} /> 正在读取路径映射信息…
        </div>
      ) : mapping ? (
        <div className="flex flex-col gap-4">
          <div className="grid grid-cols-3 gap-2">
            <div className="stat-cell" style={{ padding: "10px 12px" }}>
              <b>{mapping.totalFiles}</b>
              <span>涉及文件</span>
            </div>
            <div className="stat-cell" style={{ padding: "10px 12px" }}>
              <b className={mapping.unresolved.length ? "text-sev-warning" : "text-ai-fp"}>
                {mapping.unresolved.length}
              </b>
              <span>未定位文件</span>
            </div>
            <div className="stat-cell" style={{ padding: "10px 12px" }}>
              <b>{resolvedPct}%</b>
              <span>漏洞已定位比例</span>
            </div>
          </div>

          <div>
            <div className="flex items-center gap-2 mb-2">
              <FileSearch size={13} className="text-accent" />
              <span className="text-[11.5px] font-semibold text-primary">未定位的文件</span>
              <span className="text-[9.5px] text-tertiary">
                点击「选择文件」手工指定本地路径，永久生效
              </span>
            </div>
            {mapping.unresolved.length === 0 ? (
              <div className="flex items-center gap-2 rounded-lg border border-ai-fp/25 bg-ai-fp-soft px-3 py-4 text-[11px] text-ai-fp">
                <CheckCircle2 size={15} />
                所有沉淀点都已定位到本地源码，无需处理。
              </div>
            ) : (
              <div className="rounded-lg border border-subtle overflow-hidden max-h-52 overflow-y-auto">
                {mapping.unresolved.map((u) => (
                  <div
                    key={u.file}
                    className="flex items-center gap-2 px-3 py-2 border-b border-subtle last:border-b-0 hover:bg-surface-3"
                  >
                    <AlertTriangle size={12} className="text-sev-warning flex-none" />
                    <div className="min-w-0 flex-1">
                      <div className="font-mono text-[10.5px] text-primary truncate" title={u.file}>
                        {u.file}
                      </div>
                      <div className="text-[9px] text-tertiary">{u.count} 条漏洞依赖此文件</div>
                    </div>
                    <button
                      className="btn btn-sm btn-outline"
                      onClick={() => void pickOverride(u.file)}
                      disabled={busy === u.file}
                    >
                      {busy === u.file ? <Spinner size={11} /> : <FolderSearch size={11} />}
                      选择文件
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="divider" />

          <div>
            <div className="flex items-center gap-2 mb-2">
              <Link2 size={13} className="text-accent" />
              <span className="text-[11.5px] font-semibold text-primary">前缀规则</span>
              <span className="text-[9.5px] text-tertiary">
                把 CSV 路径前缀替换成本地目录，例如 app → /Users/me/code/pyhackme
              </span>
            </div>
            <div className="grid grid-cols-[1fr_1fr_auto] gap-2">
              <input
                className="input font-mono text-[11px]"
                value={from}
                onChange={(e) => setFrom(e.target.value)}
                placeholder="CSV 路径前缀，例如 app"
                spellCheck={false}
              />
              <input
                className="input font-mono text-[11px]"
                value={to}
                onChange={(e) => setTo(e.target.value)}
                placeholder="本地目录，例如 /Users/me/code/pyhackme"
                spellCheck={false}
              />
              <button className="btn btn-soft" onClick={() => void addRule()} disabled={busy === "add"}>
                {busy === "add" ? <Spinner size={12} /> : <Plus size={13} />}
                添加
              </button>
            </div>
            {mapping.prefixRules.length > 0 && (
              <div className="mt-2 rounded-lg border border-subtle overflow-hidden">
                {mapping.prefixRules.map((r, i) => (
                  <div
                    key={`${r.from}-${i}`}
                    className="flex items-center gap-2 px-3 py-2 border-b border-subtle last:border-b-0"
                  >
                    <span className="font-mono text-[10.5px] text-accent">{r.from}</span>
                    <span className="text-tertiary text-[10px]">→</span>
                    <span className="font-mono text-[10.5px] text-primary flex-1 truncate" title={r.to}>
                      {r.to}
                    </span>
                    <button
                      className="btn btn-icon btn-sm text-tertiary hover:text-sev-error"
                      onClick={() => void removeRule(i)}
                      disabled={busy === `rule-${i}`}
                      title="删除规则"
                    >
                      <Trash2 size={12} />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>

          {overrides.length > 0 && (
            <>
              <div className="divider" />
              <div>
                <div className="flex items-center gap-2 mb-2">
                  <FileSearch size={13} className="text-accent" />
                  <span className="text-[11.5px] font-semibold text-primary">手工映射</span>
                  <span className="text-[9.5px] text-tertiary">优先级最高，按 CSV 路径精确匹配</span>
                </div>
                <div className="rounded-lg border border-subtle overflow-hidden">
                  {overrides.map(([csv, local]) => (
                    <div key={csv} className="flex items-center gap-2 px-3 py-2 border-b border-subtle last:border-b-0">
                      <span className="font-mono text-[10px] text-secondary flex-1 truncate" title={csv}>
                        {csv}
                      </span>
                      <span className="text-tertiary text-[10px]">→</span>
                      <span className="font-mono text-[10px] text-primary flex-1 truncate" title={local}>
                        {local}
                      </span>
                      <button
                        className="btn btn-icon btn-sm text-tertiary hover:text-sev-error"
                        onClick={() => void removeOverride(csv)}
                        disabled={busy === `ov-${csv}`}
                        title="删除映射"
                      >
                        <X size={12} />
                      </button>
                    </div>
                  ))}
                </div>
              </div>
            </>
          )}

          <div className={cn("text-[10px] text-tertiary flex items-center gap-1.5")}>
            <AlertTriangle size={11} />
            索引源码目录时会跳过 .git、node_modules、vendor、__pycache__ 等噪声目录。
            修改映射后，主界面的代码跳转会立即使用新的结果。
          </div>
        </div>
      ) : null}
    </Modal>
  );
}
