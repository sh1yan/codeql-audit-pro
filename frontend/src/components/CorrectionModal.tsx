import { useEffect, useState } from "react";
import { AlertTriangle, Check, Languages, RotateCcw, Save } from "lucide-react";
import type { FindingDTO, StatsDTO } from "../types";
import { api } from "../lib/api";
import { Modal, Spinner, WarningBox } from "./ui";

export function CorrectionModal({
  open,
  finding,
  onClose,
  onSaved,
  notify,
}: {
  open: boolean;
  finding: FindingDTO | null;
  onClose: () => void;
  onSaved: (finding: FindingDTO, stats: StatsDTO) => void;
  notify: (kind: "info" | "success" | "error", message: string) => void;
}) {
  const [zh, setZh] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (open && finding) {
      setZh(finding.messageZhTemplate || "");
      setError("");
    }
  }, [open, finding]);

  const save = async () => {
    if (!finding) return;
    if (!zh.trim()) {
      notify("info", "中文模板不能为空。");
      return;
    }
    setSaving(true);
    setError("");
    try {
      const res = await api.CorrectMessage(finding.key, zh);
      onSaved(res.finding, res.stats);
      notify("success", "译文已保存，之后遇到同样的消息会直接复用。");
      onClose();
    } catch (err) {
      setError(String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="纠正译文"
      subtitle="入库键是规范化后的英文模板；保存后立即重新翻译本条漏洞"
      icon={Languages}
      size="md"
      footer={
        <>
          <button className="btn btn-outline" onClick={onClose}>
            取消
          </button>
          <button className="btn btn-primary" onClick={save} disabled={saving || !finding}>
            {saving ? <Spinner size={12} /> : <Save size={12} />}
            保存译文
          </button>
        </>
      }
    >
      {error && (
        <div className="mb-3">
          <WarningBox error>{error}</WarningBox>
        </div>
      )}
      {finding && (
        <div className="flex flex-col gap-3">
          <div>
            <div className="flex items-center gap-1.5 mb-1.5">
              <AlertTriangle size={11} className="text-sev-warning" />
              <span className="text-[10.5px] text-secondary">
                请保留 <span className="font-mono text-accent">{"{{LINK}}"}</span>、
                <span className="font-mono text-accent">{"{{N}}"}</span>、
                <span className="font-mono text-accent">{"{{W}}"}</span>、
                <span className="font-mono text-accent">{"{{API}}"}</span> 等占位符，
                否则证据链位置链接会退化成文末列表。
              </span>
            </div>
          </div>

          <div>
            <div className="text-[9.5px] text-tertiary mb-1">英文模板（入库键）</div>
            <div className="font-mono text-[10.5px] text-secondary bg-surface-2 border border-subtle rounded-md p-2 max-h-32 overflow-auto whitespace-pre-wrap break-words select-text">
              {finding.messageKey || "（空）"}
            </div>
          </div>

          <div>
            <div className="text-[9.5px] text-tertiary mb-1">中文模板（可修改）</div>
            <textarea
              className="textarea"
              style={{ minHeight: 132 }}
              value={zh}
              onChange={(e) => setZh(e.target.value)}
              placeholder="输入中文译文，保留 {{LINK}} 等占位符"
              spellCheck={false}
            />
          </div>

          <div className="flex items-center gap-2">
            <button
              className="btn btn-outline btn-sm"
              onClick={() => setZh(finding.messageZhTemplate)}
            >
              <RotateCcw size={11} /> 还原
            </button>
            <div className="flex-1" />
            <span className="text-[9.5px] text-tertiary">
              <Check size={10} className="inline mr-1" />
              保存后写入 SQLite 翻译库
            </span>
          </div>

          <details className="raw-details detail-section">
            <summary>
              <span className="text-[10px] font-semibold">本次消息原文</span>
            </summary>
            <div className="raw-code">{finding.messageRaw || "（空）"}</div>
          </details>
        </div>
      )}
    </Modal>
  );
}
