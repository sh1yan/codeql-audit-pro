import clsx, { type ClassValue } from "clsx";

export function cn(...inputs: ClassValue[]) {
  return clsx(inputs);
}

export interface SeverityMeta {
  key: string;
  label: string;
  short: string;
  text: string;
  bg: string;
  border: string;
  dot: string;
  bar: string;
}

const SEVERITY: Record<string, SeverityMeta> = {
  error: {
    key: "error",
    label: "严重",
    short: "严重",
    text: "text-sev-error",
    bg: "bg-sev-error-soft",
    border: "border-sev-error/30",
    dot: "bg-sev-error",
    bar: "bg-sev-error",
  },
  warning: {
    key: "warning",
    label: "警告",
    short: "警告",
    text: "text-sev-warning",
    bg: "bg-sev-warning-soft",
    border: "border-sev-warning/30",
    dot: "bg-sev-warning",
    bar: "bg-sev-warning",
  },
  note: {
    key: "note",
    label: "提示",
    short: "提示",
    text: "text-sev-note",
    bg: "bg-sev-note-soft",
    border: "border-sev-note/30",
    dot: "bg-sev-note",
    bar: "bg-sev-note",
  },
  recommendation: {
    key: "recommendation",
    label: "建议",
    short: "建议",
    text: "text-sev-rec",
    bg: "bg-sev-rec-soft",
    border: "border-sev-rec/30",
    dot: "bg-sev-rec",
    bar: "bg-sev-rec",
  },
};

export function severityMeta(severity: string): SeverityMeta {
  return SEVERITY[severity?.toLowerCase()] ?? SEVERITY.note;
}

export interface VerdictMeta {
  label: string;
  text: string;
  bg: string;
  border: string;
  dot: string;
  bar: string;
}

const VERDICT: Record<string, VerdictMeta> = {
  false_positive: {
    label: "误报",
    text: "text-ai-fp",
    bg: "bg-ai-fp-soft",
    border: "border-ai-fp/30",
    dot: "bg-ai-fp",
    bar: "bg-ai-fp",
  },
  true_positive: {
    label: "真实漏洞",
    text: "text-ai-tp",
    bg: "bg-ai-tp-soft",
    border: "border-ai-tp/30",
    dot: "bg-ai-tp",
    bar: "bg-ai-tp",
  },
  uncertain: {
    label: "存疑",
    text: "text-ai-uncertain",
    bg: "bg-ai-uncertain-soft",
    border: "border-ai-uncertain/30",
    dot: "bg-ai-uncertain",
    bar: "bg-ai-uncertain",
  },
};

export function verdictMeta(verdict: string | null | undefined): VerdictMeta | null {
  if (!verdict) return null;
  return VERDICT[verdict] ?? null;
}

export function fmtDateTime(value: string): string {
  if (!value) return "";
  const d = new Date(value);
  if (Number.isNaN(d.getTime())) return value;
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

export function shortPath(path: string, max = 56): string {
  if (!path) return "";
  if (path.length <= max) return path;
  const parts = path.split(/[\\/]/);
  if (parts.length <= 2) return "…" + path.slice(-max);
  let out = parts[parts.length - 1];
  for (let i = parts.length - 2; i >= 0; i--) {
    const candidate = parts[i] + "/" + out;
    if (candidate.length > max - 1) break;
    out = candidate;
  }
  return "…/" + out;
}

export function middleEllipsis(path: string, head = 30, tail = 28): string {
  if (!path || path.length <= head + tail + 1) return path;
  return `${path.slice(0, head)}…${path.slice(-tail)}`;
}

export function fileDir(path: string): string {
  const i = Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\"));
  return i > 0 ? path.slice(0, i) : path;
}

export function fileBase(path: string): string {
  const i = Math.max(path.lastIndexOf("/"), path.lastIndexOf("\\"));
  return i >= 0 ? path.slice(i + 1) : path;
}

export function pct(part: number, total: number): number {
  if (!total) return 0;
  return Math.round((part / total) * 100);
}

export function plural(n: number, unit = "条"): string {
  return `${n} ${unit}`;
}

export function messageText(segments: { kind: string; text: string }[]): string {
  return segments.map((s) => s.text).join("");
}

export function confidenceText(value: number): string {
  if (!value || value <= 0) return "";
  return `${Math.round(value * 100)}%`;
}
