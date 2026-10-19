import { useEffect, useMemo, useRef, useState } from "react";
import {
  CircleStop,
  Download,
  ExternalLink,
  Play,
  Settings2,
  Sparkles,
  Wand2,
  TriangleAlert,
  ShieldCheck,
} from "lucide-react";
import type {
  BatchProgressDTO,
  BatchSummaryDTO,
  FindingDTO,
  ProjectDTO,
  SettingsDTO,
} from "../types";
import { api, onEvent } from "../lib/api";
import { Modal, ProgressBar, Spinner, WarningBox } from "./ui";
import { cn, severityMeta, verdictMeta } from "../lib/format";

type Scope = "filtered" | "pending" | "all";

interface ResultRow {
  key: string;
  rule: string;
  file: string;
  verdict: string;
  verdictZh: string;
  reason: string;
}

export function BatchModal({
  open,
  onClose,
  project,
  filtered,
  settings,
  onProjectRefresh,
  onSelectFinding,
  onOpenSettings,
  notify,
}: {
  open: boolean;
  onClose: () => void;
  project: ProjectDTO | null;
  filtered: FindingDTO[];
  settings: SettingsDTO | null;
  onProjectRefresh: () => Promise<void>;
  onSelectFinding: (key: string) => void;
  onOpenSettings: () => void;
  notify: (kind: "info" | "success" | "error", message: string) => void;
}) {
  const pending = useMemo(() => project?.findings.filter((f) => !f.ai) ?? [], [project]);
  const [scope, setScope] = useState<Scope>("pending");
  const [running, setRunning] = useState(false);
  const [done, setDone] = useState(0);
  const [total, setTotal] = useState(0);
  const [fp, setFp] = useState(0);
  const [tp, setTp] = useState(0);
  const [un, setUn] = useState(0);
  const [failed, setFailed] = useState(0);
  const [lastError, setLastError] = useState("");
  const [summary, setSummary] = useState<BatchSummaryDTO | null>(null);
  const [rows, setRows] = useState<ResultRow[]>([]);
  const [error, setError] = useState("");
  const runningRef = useRef(false);

  const aiReady =
    !!settings?.aiVerifyEnabled &&
    !!settings?.llm.baseUrl?.trim() &&
    !!settings?.llm.model?.trim();

  const targets = useMemo(() => {
    if (!project) return [] as FindingDTO[];
    if (scope === "all") return project.findings;
    if (scope === "pending") return project.findings.filter((f) => !f.ai);
    return filtered;
  }, [scope, project, filtered]);

  useEffect(() => {
    if (!open) return;
    setScope(pending.length > 0 ? "pending" : "filtered");
    setRunning(false);
    setDone(0);
    setTotal(0);
    setFp(0);
    setTp(0);
    setUn(0);
    setFailed(0);
    setLastError("");
    setSummary(null);
    setRows([]);
    setError("");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  useEffect(() => {
    if (!open) return;
    const offProgress = onEvent<BatchProgressDTO>("batch:progress", (p) => {
      setDone(p.done);
      setTotal(p.total);
      setFp(p.falsePositive);
      setTp(p.truePositive);
      setUn(p.uncertain);
      setFailed(p.failed);
      setLastError(p.lastError || "");
      if (p.findingKey && p.ai) {
        const f = project?.findings.find((x) => x.key === p.findingKey);
        if (f) {
          setRows((prev) => {
            const next = prev.filter((r) => r.key !== p.findingKey);
            next.unshift({
              key: p.findingKey!,
              rule: f.ruleDisplay,
              file: f.sink.file,
              verdict: p.ai!.verdict,
              verdictZh: p.ai!.verdictZh,
              reason: p.ai!.reason,
            });
            return next.slice(0, 300);
          });
        }
      }
    });
    const offDone = onEvent<BatchSummaryDTO>("batch:done", (s) => {
      setRunning(false);
      runningRef.current = false;
      setSummary(s);
      notify(s.stopped ? "info" : "success", s.message);
      void onProjectRefresh();
    });
    return () => {
      offProgress();
      offDone();
    };
  }, [open, project, notify, onProjectRefresh]);

  const start = async () => {
    if (!aiReady) {
      notify("error", "AI 误报验证尚未配置：请先在设置里启用并填写接口地址与模型。");
      return;
    }
    if (targets.length === 0) {
      notify("info", "当前范围内没有需要验证的漏洞。");
      return;
    }
    setError("");
    setSummary(null);
    setRows([]);
    setDone(0);
    setFp(0);
    setTp(0);
    setUn(0);
    setFailed(0);
    setLastError("");
    try {
      const res = await api.StartBatchVerify(targets.map((f) => f.key));
      setTotal(res.total);
      setRunning(true);
      runningRef.current = true;
    } catch (err) {
      setError(String(err));
    }
  };

  const stop = async () => {
    try {
      await api.CancelBatchVerify();
      notify("info", "正在停止…（已在途的请求会先返回）");
    } catch (err) {
      setError(String(err));
    }
  };

  const close = () => {
    if (running) {
      void api.CancelBatchVerify();
      notify("info", "已请求停止批量验证，结论会实时保存。");
    }
    onClose();
  };

  const exportResults = async () => {
    try {
      const res = await api.ExportVerdicts();
      if (res) notify("success", `已导出 ${res.count} 条 AI 判定到：\n${res.path}`);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const progress = total > 0 ? done / total : 0;

  return (
    <Modal
      open={open}
      onClose={close}
      closeOnBackdrop={!running}
      title="批量 AI 误报验证"
      subtitle="三路并发调用大模型逐条判定，结论实时写回列表并持久化"
      icon={Wand2}
      size="lg"
      footer={
        <>
          <button className="btn btn-outline" onClick={close}>
            {running ? "停止并关闭" : "关闭"}
          </button>
          <div className="flex-1" />
          <button className="btn btn-outline" onClick={exportResults} disabled={!project?.findings.some((f) => f.ai)}>
            <Download size={12} />
            导出验证结果
          </button>
          {running ? (
            <button className="btn btn-danger" onClick={stop}>
              <CircleStop size={13} />
              停止
            </button>
          ) : (
            <button className="btn btn-primary" onClick={start} disabled={!aiReady || targets.length === 0}>
              <Play size={13} />
              开始验证（{targets.length} 条）
            </button>
          )}
        </>
      }
    >
      {!aiReady && (
        <div className="mb-3">
          <WarningBox>
            <div className="flex items-center gap-2">
              <span className="flex-1">
                AI 误报验证尚未启用或未配置完整。请先启用开关，并填写接口地址与模型。
              </span>
              <button className="btn btn-sm btn-soft" onClick={onOpenSettings}>
                <Settings2 size={11} /> 去设置
              </button>
            </div>
          </WarningBox>
        </div>
      )}
      {error && (
        <div className="mb-3">
          <WarningBox error>{error}</WarningBox>
        </div>
      )}

      <div className="scope-grid mb-3">
        <button
          className={cn("scope-card", scope === "filtered" && "active")}
          onClick={() => setScope("filtered")}
          disabled={running}
        >
          <b>{filtered.length}</b>
          <span>当前筛选结果</span>
          <span className="text-[9px] opacity-70 mt-1">跟随左栏规则 / 文件 / 级别 / 未读 / AI 判定筛选</span>
        </button>
        <button
          className={cn("scope-card", scope === "pending" && "active")}
          onClick={() => setScope("pending")}
          disabled={running}
        >
          <b>{pending.length}</b>
          <span>全部未验证</span>
          <span className="text-[9px] opacity-70 mt-1">跳过已有 AI 结论的条目，适合批量补全</span>
        </button>
        <button
          className={cn("scope-card", scope === "all" && "active")}
          onClick={() => setScope("all")}
          disabled={running}
        >
          <b>{project?.findings.length ?? 0}</b>
          <span>全部重新验证</span>
          <span className="text-[9px] opacity-70 mt-1">会覆盖已有结论，适合换模型后重跑</span>
        </button>
      </div>

      <div className="warning-box mb-3">
        <TriangleAlert size={14} className="mt-0.5 flex-none" />
        <div className="flex-1">
          将对 <b>{targets.length}</b> 条漏洞调用大模型。每条会发送规则名、规则描述、漏洞详情
          {settings?.aiSendCode ? "以及沉淀点/来源点附近的源码片段" : "（隐私模式：不发送源码片段）"}。
          批量验证会产生真实调用费用，请确认范围后再开始。
        </div>
      </div>

      <div className="detail-section">
        <div className="detail-section-head">
          <Sparkles size={11} className="text-accent" />
          进度
          <span className="ml-auto font-mono text-[9px] normal-case tracking-normal text-tertiary">
            {done} / {total || targets.length}
          </span>
        </div>
        <div className="p-3 flex flex-col gap-2">
          <ProgressBar value={progress} indeterminate={running && total === 0} />
          <div className="grid grid-cols-4 gap-2 mt-1">
            <Count label="误报" value={fp} tone="fp" />
            <Count label="确认" value={tp} tone="tp" />
            <Count label="存疑" value={un} tone="un" />
            <Count label="失败" value={failed} tone="failed" />
          </div>
          {lastError && <div className="text-[9.5px] text-sev-error break-all">最近失败：{lastError}</div>}
          {summary && (
            <div className="text-[10.5px] text-secondary flex items-center gap-1.5">
              <ShieldCheck size={11} className="text-ai-fp" />
              {summary.message}
            </div>
          )}
          {running && (
            <div className="text-[9.5px] text-tertiary flex items-center gap-1.5">
              <Spinner size={10} /> 正在逐条调用大模型，结果会实时写回主界面…
            </div>
          )}
        </div>
      </div>

      <div className="detail-section mt-3">
        <div className="detail-section-head">
          <ExternalLink size={11} className="text-accent" />
          验证结果
          <span className="ml-auto text-[9px] normal-case tracking-normal text-tertiary">
            点击一条可跳回主界面复核
          </span>
        </div>
        <div style={{ maxHeight: 220, overflow: "auto" }}>
          {rows.length === 0 ? (
            <div className="empty-pane" style={{ minHeight: 100 }}>
              <div className="empty-desc">还没有结果。开始验证后，每完成一条会实时出现在这里。</div>
            </div>
          ) : (
            rows.map((r) => {
              const v = verdictMeta(r.verdict);
              const sev = severityMeta("note");
              return (
                <button
                  key={r.key}
                  className="batch-result w-full bg-transparent border-x-0 border-t-0 cursor-pointer text-left hover:bg-surface-3"
                  onClick={() => {
                    onSelectFinding(r.key);
                    onClose();
                  }}
                >
                  <span className={cn("badge", v?.bg, v?.text, v?.border)}>{r.verdictZh}</span>
                  <span className="min-w-0 flex-1">
                    <span className="block text-[10.5px] text-primary truncate">{r.rule}</span>
                    <span className="block text-[9px] text-tertiary font-mono truncate">{r.file}</span>
                  </span>
                  <span className={cn("text-[9px]", sev.text)}>{r.reason ? "查看理由" : ""}</span>
                </button>
              );
            })
          )}
        </div>
      </div>
    </Modal>
  );
}

function Count({
  label,
  value,
  tone,
}: {
  label: string;
  value: number;
  tone: "fp" | "tp" | "un" | "failed";
}) {
  const color =
    tone === "fp"
      ? "text-ai-fp"
      : tone === "tp"
        ? "text-ai-tp"
        : tone === "un"
          ? "text-ai-uncertain"
          : "text-sev-error";
  return (
    <div className="stat-cell" style={{ padding: "6px 8px" }}>
      <b className={color}>{value}</b>
      <span>{label}</span>
    </div>
  );
}
