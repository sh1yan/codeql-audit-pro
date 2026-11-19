import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertTriangle,
  Loader2,
  RefreshCw,
  ShieldCheck,
  Sparkles,
} from "lucide-react";

import type {
  ActiveLocation,
  BatchProgressDTO,
  BatchSummaryDTO,
  BootstrapDTO,
  FileDTO,
  FindingDTO,
  Lang,
  LinkDTO,
  LocationDTO,
  NavPoint,
  ProjectDTO,
  ProjectHistoryDTO,
  SettingsDTO,
  StatsDTO,
  Theme,
  Verdict,
} from "./types";
import { api, onEvent } from "./lib/api";
import { EMPTY_FILTER, Sidebar, type FindingsFilter } from "./components/Sidebar";
import { TopBar, ProjectStrip } from "./components/TopBar";
import { CodePanel } from "./components/CodePanel";
import { DetailPanel } from "./components/DetailPanel";
import { StatusBar } from "./components/StatusBar";
import { StartScreen } from "./components/StartScreen";
import { Hub } from "./components/Hub";
import { AssistantWorkspace } from "./components/AssistantWorkspace";
import { Modal, ToastStack, type ToastItem, type ToastKind } from "./components/ui";
import { SettingsModal } from "./components/SettingsModal";
import { MappingModal } from "./components/MappingModal";
import { LibraryModal } from "./components/LibraryModal";
import { BatchModal } from "./components/BatchModal";
import { CorrectionModal } from "./components/CorrectionModal";
import { ProjectHistoryModal } from "./components/ProjectHistoryModal";
import { messageText, cn } from "./lib/format";

type ModalKind =
  | "settings"
  | "mapping"
  | "library"
  | "batch"
  | "correction"
  | "history"
  | null;
type Tone = "info" | "success" | "error";

type ConfirmState = {
  title: string;
  message: string;
  confirmLabel: string;
  danger?: boolean;
  onConfirm: () => void | Promise<void>;
};

const clamp = (v: number, min: number, max: number) => Math.min(max, Math.max(min, v));

function initialTheme(): Theme {
  const param = new URLSearchParams(window.location.search).get("theme");
  if (param === "light" || param === "dark") return param;
  const saved = localStorage.getItem("cqa-theme");
  if (saved === "light" || saved === "dark") return saved;
  return window.matchMedia?.("(prefers-color-scheme: light)").matches ? "light" : "dark";
}

function initialInfoCollapsed(): boolean {
  return localStorage.getItem("cqa-info-collapsed") === "1";
}

// 开发自检时可以带 ?tool=assistant 直接进入某个工作台；正式启动始终从首页开始。
function initialTool(): "hub" | "audit" | "assistant" {
  const value = new URLSearchParams(window.location.search).get("tool");
  return value === "audit" || value === "assistant" ? value : "hub";
}

function shortcutMatches(e: KeyboardEvent, spec: string): boolean {
  const parts = spec
    .split("+")
    .map((s) => s.trim().toLowerCase())
    .filter(Boolean);
  const key = parts.pop();
  if (!key) return false;
  const mods = new Set(parts);
  const cmd =
    mods.has("cmd") || mods.has("command") || mods.has("super") || mods.has("meta") || mods.has("win");
  const ctrl = mods.has("ctrl") || mods.has("control");
  const alt = mods.has("alt") || mods.has("option") || mods.has("opt");
  const shift = mods.has("shift");
  if (e.metaKey !== cmd) return false;
  if (e.ctrlKey !== ctrl) return false;
  if (e.altKey !== alt) return false;
  if (e.shiftKey !== shift) return false;
  const eventKey = e.key.toLowerCase();
  if (key.length === 1) return eventKey === key;
  if (/^f([1-9]|1[0-2])$/.test(key)) return eventKey === key;
  const symbol: Record<string, string> = {
    slash: "/",
    backslash: "\\",
    minus: "-",
    dash: "-",
    equal: "=",
    period: ".",
    dot: ".",
    comma: ",",
    semicolon: ";",
    leftbracket: "[",
    rightbracket: "]",
    space: " ",
  };
  return eventKey === (symbol[key] ?? key);
}

export default function App() {
  const [boot, setBoot] = useState<BootstrapDTO | null>(null);
  const [bootLoading, setBootLoading] = useState(true);
  const [bootError, setBootError] = useState("");
  const [settings, setSettings] = useState<SettingsDTO | null>(null);
  const [project, setProject] = useState<ProjectDTO | null>(null);
  const [projectHistory, setProjectHistory] = useState<ProjectHistoryDTO[]>([]);
  const [translating, setTranslating] = useState(false);
  // 批量 AI 验证的全局状态：顶栏常驻显示进度 + 强制停止入口，
  // 即使批量窗口被关掉，用户也能随时中止整批任务。
  const [batch, setBatch] = useState({ running: false, stopping: false, done: 0, total: 0 });
  const [translateProgress, setTranslateProgress] = useState({ done: 0, total: 0 });
  const [confirm, setConfirm] = useState<ConfirmState | null>(null);
  const [selectedKey, setSelectedKey] = useState<string | null>(null);
  const [active, setActive] = useState<ActiveLocation | null>(null);
  const [file, setFile] = useState<FileDTO | null>(null);
  const [fileLoading, setFileLoading] = useState(false);
  const [fileError, setFileError] = useState("");
  const [history, setHistory] = useState<NavPoint[]>([]);
  const [filter, setFilter] = useState<FindingsFilter>(EMPTY_FILTER);
  const [theme, setTheme] = useState<Theme>(initialTheme);
  const [tool, setTool] = useState<"hub" | "audit" | "assistant">(initialTool);
  const [assistantRunning, setAssistantRunning] = useState(false);
  const [assistantStatusLabel, setAssistantStatusLabel] = useState("就绪");
  const [externalAuditRequest, setExternalAuditRequest] = useState<{
    csvPath: string;
    srcRoot: string;
  } | null>(null);
  const [lang, setLang] = useState<Lang>("zh");
  const [showRuler, setShowRuler] = useState(true);
  const [searchOpen, setSearchOpen] = useState(false);
  const [infoCollapsed, setInfoCollapsed] = useState(initialInfoCollapsed);
  const [leftRatio, setLeftRatio] = useState(0.245);
  const [rightRatio, setRightRatio] = useState(0.38);
  const [loading, setLoading] = useState<{
    active: boolean;
    message: string;
    done: number;
    total: number;
  }>({ active: false, message: "", done: 0, total: 0 });
  const [status, setStatus] = useState<{ message: string; tone: Tone; busy: boolean }>({
    message: "就绪",
    tone: "info",
    busy: false,
  });
  const [toasts, setToasts] = useState<ToastItem[]>([]);
  const [modal, setModal] = useState<ModalKind>(() => {
    if (!import.meta.env.DEV) return null;
    const value = new URLSearchParams(window.location.search).get("modal");
    return value === "settings" || value === "mapping" || value === "library" || value === "batch"
      ? value
      : null;
  });
  const [verifyingKey, setVerifyingKey] = useState<string | null>(null);
  const [windowSize, setWindowSize] = useState({ w: window.innerWidth, h: window.innerHeight });

  const workspaceRef = useRef<HTMLDivElement>(null);
  const fileRef = useRef<FileDTO | null>(null);
  const activeRef = useRef<ActiveLocation | null>(null);
  const selectedKeyRef = useRef<string | null>(null);
  const lastSelectedRef = useRef<string | null>(null);
  const reviewedRef = useRef<Set<string>>(new Set());
  const toastIdRef = useRef(0);
  const loadingRef = useRef(loading);

  fileRef.current = file;
  activeRef.current = active;
  selectedKeyRef.current = selectedKey;
  loadingRef.current = loading;

  const notify = useCallback((kind: ToastKind, message: string) => {
    const id = ++toastIdRef.current;
    setToasts((prev) => [...prev.slice(-4), { id, kind, message }]);
    window.setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id));
    }, kind === "error" ? 7000 : 4200);
  }, []);

  const setStatMessage = useCallback((message: string, tone: Tone = "info", busy = false) => {
    setStatus({ message, tone, busy });
  }, []);

  const patchFinding = useCallback((key: string, patch: Partial<FindingDTO>) => {
    setProject((p) =>
      p ? { ...p, findings: p.findings.map((f) => (f.key === key ? { ...f, ...patch } : f)) } : p,
    );
  }, []);

  const applyStats = useCallback((stats: StatsDTO) => {
    setProject((p) => (p ? { ...p, stats } : p));
  }, []);

  const openLocation = useCallback(
    async (
      finding: FindingDTO,
      loc: LocationDTO,
      kind: ActiveLocation["kind"],
      index: number,
      pushHistory: boolean,
      meta?: Pick<ActiveLocation, "flowIndex" | "stepIndex">,
    ) => {
      if (pushHistory) {
        const currentFile = fileRef.current;
        const currentActive = activeRef.current;
        if (currentFile && currentActive) {
          setHistory((h) => [
            ...h,
            {
              csvPath: currentFile.displayPath || currentFile.path,
              resolvedPath: currentFile.path,
              line: currentActive.loc.startLine,
              active: currentActive,
            },
          ]);
        }
      }
      setFileLoading(true);
      setFileError("");
      try {
        const next = await api.GetSourceFile(loc.file, loc.resolvedPath);
        const resolvedLoc: LocationDTO = {
          ...loc,
          resolvedPath: next.path,
          resolved: true,
          displayPath: loc.displayPath || loc.file,
        };
        setFile(next);
        setActive({ kind, index, loc: resolvedLoc, ...meta });
        setStatMessage(
          `已定位 ${finding.ruleDisplay} · ${loc.displayPath}:${loc.startLine}`,
          "info",
          false,
        );
      } catch (err) {
        setFileError(String(err));
        setActive({ kind, index, loc, ...meta });
        setStatMessage(`未定位：${loc.file}（可在路径映射里手工指定）`, "error", false);
      } finally {
        setFileLoading(false);
      }
    },
    [setStatMessage],
  );

  const markReviewed = useCallback(
    async (key: string) => {
      if (!key || reviewedRef.current.has(key)) return;
      reviewedRef.current.add(key);
      try {
        const stats = await api.SetReviewed([key], true);
        patchFinding(key, { reviewed: true });
        applyStats(stats);
      } catch {
        reviewedRef.current.delete(key);
      }
    },
    [applyStats, patchFinding],
  );

  const refreshHistory = useCallback(async () => {
    try {
      setProjectHistory(await api.GetProjectHistory());
    } catch {
      // 历史库暂时不可用时不影响正常审计。
    }
  }, []);

  const loadProject = useCallback(
    async (csvPath: string, srcRoot: string, notifyUser = true) => {
      setLoading({ active: true, message: "正在解析、翻译并定位源码…", done: 0, total: 0 });
      setStatMessage("正在载入项目…", "info", true);
      try {
        const p = await api.LoadProject(csvPath, srcRoot);
        lastSelectedRef.current = null;
        reviewedRef.current = new Set(p.findings.filter((f) => f.reviewed).map((f) => f.key));
        setProject(p);
        setFile(null);
        setFileError("");
        setHistory([]);
        setSearchOpen(false);
        setFilter(EMPTY_FILTER);
        const first = p.findings[0] ?? null;
        setSelectedKey(first?.key ?? null);
        setActive(first ? { kind: "sink", index: 0, loc: first.sink } : null);
        setStatMessage(
          `已载入 ${p.findings.length} 条漏洞 · 已定位 ${p.stats.resolved}/${p.stats.total}`,
          "success",
          false,
        );
        if (notifyUser) {
          notify("success", `已载入 ${p.csvName}：${p.findings.length} 条漏洞`);
        }
        const warnings = p.warnings ?? [];
        if (warnings.length > 0) {
          notify("info", `解析提示：\n${warnings.slice(0, 5).join("\n")}`);
        }
        void refreshHistory();
      } catch (err) {
        setStatMessage("载入失败", "error", false);
        notify("error", String(err));
      } finally {
        setLoading({ active: false, message: "", done: 0, total: 0 });
      }
    },
    [notify, setStatMessage, refreshHistory],
  );

  // 运行器分析出结果后，可以直接带着结果和源码目录跳到审计工作台。
  useEffect(() => {
    if (!externalAuditRequest) return;
    const request = externalAuditRequest;
    setExternalAuditRequest(null);
    setTool("audit");
    void loadProject(
      request.csvPath,
      request.srcRoot || boot?.lastSrcRoot || project?.srcRoot || "",
    );
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [externalAuditRequest]);

  const resetWorkspace = useCallback(() => {
    setProject(null);
    setSelectedKey(null);
    setActive(null);
    setFile(null);
    setFileError("");
    setHistory([]);
    setFilter(EMPTY_FILTER);
    setSearchOpen(false);
    reviewedRef.current.clear();
    lastSelectedRef.current = null;
  }, []);

  const closeCurrentProject = () => {
    if (!project) return;
    setConfirm({
      title: "退出当前项目",
      message:
        "退出后回到审计工作台的选择界面。\n\n已读标记、AI 判定与路径映射都会保留在「历史项目」里。",
      confirmLabel: "退出项目",
      onConfirm: async () => {
        try {
          await api.CloseProject();
          resetWorkspace();
          setStatMessage("已退出当前项目", "info", false);
          notify("success", "已退出当前项目，可从历史项目继续审计。");
          void refreshHistory();
        } catch (err) {
          notify("error", String(err));
        }
      },
    });
  };

  const openHistoryProject = async (item: ProjectHistoryDTO) => {
    if (!item.resultExists) {
      notify("error", `结果文件不存在：\n${item.csvPath}\n\n可以删除这条历史记录，或把文件放回原位置。`);
      return;
    }
    setTool("audit");
    setModal(null);
    await loadProject(item.csvPath, item.srcRoot);
  };

  const performDeleteHistory = async (item: ProjectHistoryDTO, active: boolean) => {
    try {
      const list = await api.DeleteProjectHistory(item.csvPath);
      setProjectHistory(list);
      if (active) resetWorkspace();
      if (boot?.lastProject && boot.lastProject.toLowerCase() === item.csvPath.toLowerCase()) {
        setBoot((b) => (b ? { ...b, lastProject: "", lastSrcRoot: "" } : b));
      }
      notify("success", `已删除历史项目：${item.csvName}`);
      setStatMessage("已删除历史项目", "info", false);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const deleteHistoryProject = (item: ProjectHistoryDTO) => {
    const active = !!(
      project && project.csvPath.toLowerCase() === item.csvPath.toLowerCase()
    );
    setConfirm({
      title: "删除历史项目",
      message:
        `删除「${item.csvName}」？\n\n` +
        (active ? "当前项目会被关闭并回到选择界面。\n" : "") +
        "只会删除已读标记、AI 判定与路径映射记录，不会删除结果文件、源码或翻译库。",
      confirmLabel: "删除",
      danger: true,
      onConfirm: () => performDeleteHistory(item, active),
    });
  };

  const performClearHistory = async () => {
    try {
      const list = await api.ClearProjectHistory();
      setProjectHistory(list);
      resetWorkspace();
      setBoot((b) => (b ? { ...b, lastProject: "", lastSrcRoot: "" } : b));
      notify("success", "已清空历史项目");
      setStatMessage("已清空历史项目", "info", false);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const clearProjectHistory = () => {
    if (projectHistory.length === 0) return;
    setConfirm({
      title: "清空历史项目",
      message:
        `清空全部 ${projectHistory.length} 个历史项目？\n\n` +
        "只会删除已读标记、AI 判定与路径映射记录，不会删除结果文件、源码或翻译库。",
      confirmLabel: "清空全部",
      danger: true,
      onConfirm: performClearHistory,
    });
  };

  const translateProject = async () => {
    if (!project || translating) return;
    setTranslating(true);
    setTranslateProgress({ done: 0, total: project.stats.translationMissing || 0 });
    setStatMessage("正在补全翻译…", "info", true);
    try {
      const res = await api.TranslateProject();
      if (res.project) setProject(res.project);
      setTranslateProgress({ done: res.total, total: res.total });
      const text =
        res.remaining > 0 && res.lastError
          ? `${res.message}\n最近一次错误：${res.lastError}`
          : res.message;
      notify(res.remaining > 0 ? "info" : "success", text);
      setStatMessage(res.message, res.remaining > 0 ? "info" : "success", false);
      void refreshHistory();
    } catch (err) {
      notify("error", String(err));
      setStatMessage("补全翻译失败", "error", false);
    } finally {
      setTranslating(false);
    }
  };

  const refreshProject = useCallback(async () => {
    const p = await api.GetProject();
    if (!p) return;
    setProject(p);
    const key = selectedKeyRef.current;
    const act = activeRef.current;
    const f = p.findings.find((x) => x.key === key);
    if (!f || !act) return;
    let loc: LocationDTO | null = null;
    if (act.kind === "sink") loc = f.sink;
    else if (act.kind === "source") loc = f.sources[act.index] ?? null;
    else if (act.kind === "flow") {
      const flow = f.codeFlows.find((cf) => cf.index === act.flowIndex);
      loc = flow?.steps.find((s) => s.index === act.stepIndex)?.location ?? act.loc;
    } else loc = act.loc;
    if (!loc) return;
    setActive({ ...act, loc });
    if (loc.resolved && loc.resolvedPath && loc.resolvedPath !== fileRef.current?.path) {
      await openLocation(f, loc, act.kind, act.index, false);
    }
  }, [openLocation]);

  // 启动：只读取 bootstrap 与历史项目元数据，不自动解析上一次的结果文件。
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const b = await api.Bootstrap();
        if (cancelled) return;
        setBoot(b);
        setSettings(b.settings);
        setLang(b.settings.displayLang || "zh");
        setShowRuler(b.settings.showColumnRuler);
        setInfoCollapsed(b.settings.topInfoCollapsed);
        const detail = clamp(b.settings.splitDetail || 0.5, 0.25, 0.75);
        const right = clamp((1 - (b.settings.splitLeft || 0.245)) * (1 - detail), 0.22, 0.55);
        const left = clamp(
          b.settings.splitLeft || 0.245,
          0.16,
          Math.min(0.42, 1 - right - 0.3),
        );
        setRightRatio(right);
        setLeftRatio(left);
        if (b.warning) notify("info", b.warning);
        // 历史项目列表只读取 history.db 的轻量元数据，不解析结果文件。
        // 不阻塞首屏：先让选择界面出来，列表随后补上。
        void refreshHistory();
        // 只有命令行显式传入 -csv 时才自动载入；普通启动停留在选择界面，
        // 由用户从历史项目里挑一个，避免每次启动都解析几百 MB 的结果文件。
        const skipAuto =
          import.meta.env.DEV && new URLSearchParams(window.location.search).get("empty") === "1";
        if (b.autoLoad && b.lastProject && !skipAuto) {
          setTool("audit");
          await loadProject(b.lastProject, b.lastSrcRoot, false);
        }
      } catch (err) {
        if (!cancelled) setBootError(String(err));
      } finally {
        if (!cancelled) setBootLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // 主题。
  useEffect(() => {
    document.documentElement.dataset.theme = theme;
    localStorage.setItem("cqa-theme", theme);
    if (boot) {
      api.SetNativeTheme(theme).catch(() => {});
    }
  }, [theme, boot]);

  // 选中漏洞变化：重置到沉淀点、清空历史、自动标记已读。
  useEffect(() => {
    if (!project || !selectedKey) {
      lastSelectedRef.current = null;
      return;
    }
    const current = project.findings.find((f) => f.key === selectedKey);
    if (!current) return;
    if (lastSelectedRef.current === current.key) return;
    lastSelectedRef.current = current.key;
    setHistory([]);
    setSearchOpen(false);
    setActive({ kind: "sink", index: 0, loc: current.sink });
    void openLocation(current, current.sink, "sink", 0, false);
    void markReviewed(current.key);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedKey, project?.csvPath, project?.loadedAt]);

  // 进度事件。
  useEffect(() => {
    const off = onEvent<{ phase: string; done: number; total: number; message: string }>(
      "project:progress",
      (p) => {
        setLoading((prev) => ({
          active: true,
          message: p.message || prev.message,
          done: p.done,
          total: p.total,
        }));
        setStatMessage(p.message || "正在载入项目…", "info", true);
      },
    );
    return off;
  }, [setStatMessage]);

  // 补全翻译进度。
  useEffect(() => {
    const off = onEvent<{ done: number; total: number; message: string }>(
      "translate:progress",
      (p) => {
        setTranslateProgress({ done: p.done, total: p.total });
        setStatMessage(p.message || `正在补全翻译 ${p.done}/${p.total}`, "info", true);
      },
    );
    return off;
  }, [setStatMessage]);

  // 批量验证：实时回填 AI 结论，结束后刷新项目。
  useEffect(() => {
    const offProgress = onEvent<BatchProgressDTO>("batch:progress", (p) => {
      setBatch({
        running: true,
        stopping: !!p.stopping,
        done: p.done,
        total: p.total,
      });
      if (!p.findingKey || !p.ai) return;
      setProject((prev) => {
        if (!prev) return prev;
        const findings = prev.findings.map((f) =>
          f.key === p.findingKey ? { ...f, ai: p.ai! } : f,
        );
        const verified = p.falsePositive + p.truePositive + p.uncertain;
        return {
          ...prev,
          findings,
          stats: {
            ...prev.stats,
            verified,
            falsePositive: p.falsePositive,
            truePositive: p.truePositive,
            uncertain: p.uncertain,
          },
        };
      });
    });
    const offStopping = onEvent<{ done: number; total: number }>("batch:stopping", (p) => {
      setBatch((prev) => ({ ...prev, running: true, stopping: true, done: p.done ?? prev.done, total: p.total || prev.total }));
    });
    const offDone = onEvent<BatchSummaryDTO>("batch:done", (s) => {
      setBatch({ running: false, stopping: false, done: s.done, total: s.total });
      setStatMessage(s.message, s.stopped ? "info" : "success", false);
      void refreshProject();
    });
    return () => {
      offProgress();
      offStopping();
      offDone();
    };
  }, [refreshProject, setStatMessage]);

  // 界面重新载入 / 切换项目后，同步一次后端真实的批量状态，
  // 避免「后端还在跑、界面却以为没开始」。
  useEffect(() => {
    let alive = true;
    void api
      .BatchState()
      .then((st) => {
        if (!alive) return;
        setBatch({
          running: !!st?.running,
          stopping: !!st?.stopping,
          done: st?.done ?? 0,
          total: st?.total ?? 0,
        });
      })
      .catch(() => {});
    return () => {
      alive = false;
    };
  }, [project?.csvPath, project?.loadedAt]);

  const stopBatch = async () => {
    setBatch((prev) => ({ ...prev, stopping: true }));
    try {
      await api.CancelBatchVerify();
      notify("info", "已请求强制停止批量 AI 验证：剩余条目不会再调用大模型。");
    } catch (err) {
      notify("error", String(err));
    }
  };

  // 文件拖放。
  useEffect(() => {
    const off = onEvent<{ paths: string[] }>("file:dropped", (payload) => {
      if (tool !== "audit") return;
      const paths = payload?.paths ?? [];
      if (paths.length === 0) return;
      const result = paths.find((p) => /\.(csv|sarif|json)$/i.test(p));
      if (result) {
        void loadProject(result, boot?.lastSrcRoot || project?.srcRoot || "");
        return;
      }
      if (project) {
        void setSrcRoot(paths[0]);
      } else {
        notify("info", "请先打开 CodeQL 结果文件（CSV / SARIF），再拖入源码目录。");
      }
    });
    return off;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [boot?.lastSrcRoot, project?.csvPath, project?.srcRoot, loadProject, notify, tool]);

  // 窗口尺寸 + 偏好落盘（防抖）。
  useEffect(() => {
    const onResize = () =>
      setWindowSize({ w: window.innerWidth, h: window.innerHeight });
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);

  useEffect(() => {
    if (!boot) return;
    const timer = window.setTimeout(() => {
      const middle = 1 - leftRatio - rightRatio;
      const splitDetail = leftRatio < 0.999 ? middle / (1 - leftRatio) : 0.5;
      api
        .SaveUIPreferences({
          displayLang: lang,
          showColumnRuler: showRuler,
          splitLeft: leftRatio,
          splitDetail: clamp(splitDetail, 0.25, 0.75),
          windowWidth: windowSize.w,
          windowHeight: windowSize.h,
          topInfoCollapsed: infoCollapsed,
        })
        .catch(() => {});
    }, 450);
    return () => window.clearTimeout(timer);
  }, [boot, lang, showRuler, leftRatio, rightRatio, windowSize.w, windowSize.h, infoCollapsed]);

  // 快捷键。
  const searchShortcut =
    settings?.codeSearchShortcut?.trim() || boot?.defaultSearchShortcut || "Cmd+F";
  useEffect(() => {
    if (tool !== "audit") return;
    const onKey = (e: KeyboardEvent) => {
      if (modal !== null) return;
      if (shortcutMatches(e, searchShortcut)) {
        e.preventDefault();
        if (project) {
          setSearchOpen(true);
        }
        return;
      }
      if (e.key === "Escape" && modal === null && searchOpen) {
        setSearchOpen(false);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [searchShortcut, project, modal, searchOpen, tool]);

  const selected = useMemo(
    () => project?.findings.find((f) => f.key === selectedKey) ?? null,
    [project, selectedKey],
  );

  const filtered = useMemo(() => {
    if (!project) return [] as FindingDTO[];
    const kw = filter.keyword.trim().toLowerCase();
    return project.findings.filter((f) => {
      if (filter.severity && f.severity !== filter.severity) return false;
      if (filter.rule && f.ruleName !== filter.rule) return false;
      if (filter.file && f.sink.file !== filter.file) return false;
      if (filter.onlyUnread && f.reviewed) return false;
      if (filter.onlyUnresolved && f.sink.resolved) return false;
      if (filter.verdict) {
        if (filter.verdict === "__none__") {
          if (f.ai) return false;
        } else if (f.ai?.verdict !== (filter.verdict as Verdict)) {
          return false;
        }
      }
      if (kw) {
        const hay = [
          f.ruleDisplay,
          f.ruleName,
          f.descriptionDisplay,
          f.description,
          f.sink.file,
          f.messageRaw,
          messageText(f.messageZh),
          messageText(f.messageEn),
          ...(f.cwes ?? []),
          ...(f.tags ?? []),
          ...(f.fingerprints ?? []).map((fp) => `${fp.name} ${fp.value}`),
          ...(f.codeFlows ?? []).flatMap((cf) =>
            cf.steps.map((s) => `${s.message} ${s.location.labelZh}`),
          ),
        ]
          .join("\n")
          .toLowerCase();
        if (!hay.includes(kw)) return false;
      }
      return true;
    });
  }, [project, filter]);

  const focusLine = active?.loc.startLine || selected?.sink.startLine || 1;

  // ------------------------------ 动作 ------------------------------

  const openCSV = async () => {
    try {
      const path = await api.ChooseResultsFile();
      if (path) await loadProject(path, boot?.lastSrcRoot || project?.srcRoot || "");
    } catch (err) {
      notify("error", String(err));
    }
  };

  const setSrcRoot = async (root: string) => {
    if (!project) {
      notify("info", "请先打开 CodeQL 结果文件。");
      return;
    }
    setLoading({ active: true, message: "正在重新定位源码…", done: 0, total: 0 });
    setStatMessage("正在更换源码目录并重新定位…", "info", true);
    try {
      const p = await api.SetSrcRoot(root);
      setProject(p);
      lastSelectedRef.current = null;
      const f = p.findings.find((x) => x.key === selectedKey) ?? p.findings[0];
      if (f) {
        setSelectedKey(f.key);
        setActive({ kind: "sink", index: 0, loc: f.sink });
        setHistory([]);
        await openLocation(f, f.sink, "sink", 0, false);
      }
      setStatMessage(
        `源码目录已更新 · 已定位 ${p.stats.resolved}/${p.stats.total}`,
        "success",
        false,
      );
      notify("success", "源码目录已更新，已重新定位全部漏洞。");
      void refreshHistory();
    } catch (err) {
      notify("error", String(err));
      setStatMessage("更换源码目录失败", "error", false);
    } finally {
      setLoading({ active: false, message: "", done: 0, total: 0 });
    }
  };

  const chooseSrcRoot = async () => {
    if (!project) {
      notify("info", "请先打开 CodeQL 结果文件，再选择源码目录。");
      return;
    }
    try {
      const root = await api.ChooseDirectory("选择被审计的源码目录");
      if (root) await setSrcRoot(root);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const jumpTo = (kind: "sink" | "source", index: number) => {
    if (!selected) return;
    let loc: LocationDTO | null = null;
    if (kind === "sink") loc = selected.sink;
    else loc = selected.sources[index] ?? null;
    if (!loc) return;
    void openLocation(selected, loc, kind, index, true);
  };

  const jumpToFlowStep = (flowIndex: number, stepIndex: number) => {
    if (!selected) return;
    const flow = selected.codeFlows.find((cf) => cf.index === flowIndex);
    const step = flow?.steps.find((s) => s.index === stepIndex);
    if (!step) return;
    void openLocation(selected, step.location, "flow", step.index, true, {
      flowIndex,
      stepIndex,
    });
  };

  const openExternal = (url: string) => {
    if (!url) return;
    api.OpenExternalURL(url).catch((err) => notify("error", String(err)));
  };

  const jumpToLink = (link: LinkDTO) => {
    if (!selected) return;
    const idx = selected.sources.findIndex(
      (s) =>
        s.file === link.file &&
        s.startLine === link.startLine &&
        s.startCol === link.startCol,
    );
    if (idx >= 0) {
      void openLocation(selected, selected.sources[idx], "source", idx, true);
      return;
    }
    // SARIF relatedLocations：直接使用后端已经解析好的位置（含本地路径）。
    const related = (selected.relatedLocations ?? []).find(
      (r) =>
        r.location.file === link.file &&
        r.location.startLine === link.startLine &&
        r.location.startCol === link.startCol,
    );
    if (related) {
      void openLocation(selected, related.location, "link", -1, true);
      return;
    }
    const loc: LocationDTO = {
      file: link.file,
      resolvedPath: "",
      startLine: link.startLine,
      startCol: link.startCol,
      endLine: link.endLine,
      endCol: link.endCol,
      label: link.label,
      labelZh: link.labelZh,
      resolved: false,
      lineRange: String(link.startLine),
      columnRange: link.startCol ? `${link.startCol}-${link.endCol}` : "",
      lineCol: `${link.startLine}:${link.startCol}`,
      displayPath: link.file,
      baseName: link.file.split("/").pop() || link.file,
    };
    void openLocation(selected, loc, "link", -1, true);
  };

  const goBack = async () => {
    if (history.length === 0) {
      setStatMessage("没有可返回的跳转记录。", "info", false);
      return;
    }
    const point = history[history.length - 1];
    setHistory((h) => h.slice(0, -1));
    setFileLoading(true);
    setFileError("");
    try {
      const f = await api.GetSourceFile(point.csvPath, point.resolvedPath);
      setFile(f);
      setActive(point.active);
      setStatMessage(`已返回 ${point.csvPath}:${point.line}`, "info", false);
    } catch (err) {
      setFileError(String(err));
      setStatMessage("返回失败：原文件已不可读", "error", false);
    } finally {
      setFileLoading(false);
    }
  };

  const verify = async () => {
    if (!selected) return;
    setVerifyingKey(selected.key);
    setStatMessage("正在做 AI 误报验证…", "info", true);
    try {
      const res = await api.VerifyFinding(selected.key);
      patchFinding(selected.key, { ai: res.ai });
      applyStats(res.stats);
      setStatMessage(`AI 判定：${res.ai?.verdictZh ?? "已完成"}`, "success", false);
      notify("success", `AI 判定：${res.ai?.verdictZh ?? "已完成"}`);
    } catch (err) {
      notify("error", String(err));
      setStatMessage("AI 验证失败", "error", false);
    } finally {
      setVerifyingKey(null);
    }
  };

  const clearVerdict = async () => {
    if (!selected) return;
    try {
      const stats = await api.ClearVerdict(selected.key);
      patchFinding(selected.key, { ai: null });
      applyStats(stats);
      notify("success", "已清除该条 AI 判定结果");
      setStatMessage("已清除 AI 判定结果", "info", false);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const copyChinese = async () => {
    if (!selected) return;
    const flowText = (selected.codeFlows ?? [])
      .map((cf) => {
        const steps = cf.steps
          .map(
            (s) =>
              `  ${s.index}. ${s.location.displayPath}:${s.location.startLine}` +
              (s.message ? `  ${s.message}` : ""),
          )
          .join("\n");
        return `路径 ${cf.index}（${cf.steps.length} 步）\n${steps}`;
      })
      .join("\n\n");
    const text = [
      selected.ruleDisplay,
      selected.ruleName,
      `严重级别：${selected.severityZh}`,
      selected.cwes?.length ? `CWE：${selected.cwes.join(", ")}` : "",
      `位置：${selected.sink.displayPath}:${selected.sink.startLine}:${selected.sink.startCol}`,
      "",
      selected.descriptionDisplay,
      "",
      messageText(selected.messageZh) || messageText(selected.messageEn),
      flowText ? `\n污点传播路径：\n${flowText}` : "",
    ]
      .filter(Boolean)
      .join("\n");
    try {
      await api.CopyText(text);
      notify("success", "已复制当前漏洞的中文详情到剪贴板");
      setStatMessage("已复制中文详情", "success", false);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const openInEditor = async () => {
    const path = fileRef.current?.path || selected?.sink.resolvedPath || "";
    if (!path) {
      notify("info", "当前没有可打开的源码文件。");
      return;
    }
    const line = activeRef.current?.loc.startLine || selected?.sink.startLine || 1;
    const col = activeRef.current?.loc.startCol || selected?.sink.startCol || 1;
    try {
      const desc = await api.OpenInEditor(path, line, col);
      notify("success", `已用编辑器打开：${desc}`);
      setStatMessage("已用编辑器打开源码", "success", false);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const reveal = async () => {
    const path = fileRef.current?.path;
    if (!path) return;
    try {
      await api.RevealInFinder(path);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const exportCurrent = async (kind: "csv" | "report" | "verdicts") => {
    if (!project) return;
    const keys = filtered.map((f) => f.key);
    setStatMessage("正在导出…", "info", true);
    try {
      const res =
        kind === "csv"
          ? await api.ExportChineseCSV(keys)
          : kind === "report"
            ? await api.ExportAuditReport(keys)
            : await api.ExportVerdicts();
      if (res) {
        notify("success", `已导出 ${res.count} 条 ${res.kind} 到：\n${res.path}`);
        setStatMessage(`已导出 ${res.kind}`, "success", false);
      } else {
        setStatMessage("已取消导出", "info", false);
      }
    } catch (err) {
      notify("error", String(err));
      setStatMessage("导出失败", "error", false);
    }
  };

  const markAllRead = async () => {
    if (!project) return;
    const keys = filtered.map((f) => f.key);
    if (keys.length === 0) return;
    try {
      const stats = await api.SetReviewed(keys, true);
      setProject((p) =>
        p
          ? {
              ...p,
              findings: p.findings.map((f) => (keys.includes(f.key) ? { ...f, reviewed: true } : f)),
              stats,
            }
          : p,
      );
      notify("success", `已把 ${keys.length} 条标记为已读`);
      setStatMessage(`已标记 ${keys.length} 条为已读`, "success", false);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const resetRead = async () => {
    if (!project) return;
    try {
      const stats = await api.ResetReviewed();
      reviewedRef.current.clear();
      setProject((p) =>
        p ? { ...p, findings: p.findings.map((f) => ({ ...f, reviewed: false })), stats } : p,
      );
      notify("success", "已重置全部已读标记");
      setStatMessage("已重置已读标记", "info", false);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const selectFinding = (f: FindingDTO) => {
    setSelectedKey(f.key);
    if (lastSelectedRef.current === f.key) {
      // 重复点击同一行：手动重新定位到沉淀点。
      setActive({ kind: "sink", index: 0, loc: f.sink });
      void openLocation(f, f.sink, "sink", 0, false);
    }
  };

  const changeLang = (next: Lang) => {
    setLang(next);
    setStatMessage(next === "zh" ? "详情语言：中文" : next === "en" ? "详情语言：英文" : "详情语言：中英对照");
  };

  const toggleTheme = () => setTheme((t) => (t === "dark" ? "light" : "dark"));

  const toggleInfo = () => {
    setInfoCollapsed((v) => {
      localStorage.setItem("cqa-info-collapsed", v ? "0" : "1");
      return !v;
    });
  };

  const toggleRuler = () => setShowRuler((v) => !v);

  const onSettingsSaved = (s: SettingsDTO) => {
    setSettings(s);
    setBoot((b) => (b ? { ...b, settings: s } : b));
    setLang(s.displayLang);
    setShowRuler(s.showColumnRuler);
    setStatMessage("设置已保存", "success", false);
  };

  // 分隔条拖动。
  const startResize = (side: "left" | "right") => (e: React.MouseEvent) => {
    e.preventDefault();
    const workspace = workspaceRef.current;
    if (!workspace) return;
    const total = workspace.getBoundingClientRect().width;
    const startX = e.clientX;
    const startLeft = leftRatio;
    const startRight = rightRatio;
    document.body.classList.add("resizing");
    const onMove = (ev: MouseEvent) => {
      const dx = (ev.clientX - startX) / Math.max(1, total);
      if (side === "left") {
        setLeftRatio(clamp(startLeft + dx, 0.16, Math.min(0.42, 1 - startRight - 0.3)));
      } else {
        setRightRatio(clamp(startRight - dx, 0.22, Math.min(0.55, 1 - startLeft - 0.3)));
      }
    };
    const onUp = () => {
      document.body.classList.remove("resizing");
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  };

  const resetSplit = () => {
    setLeftRatio(0.245);
    setRightRatio(0.38);
  };

  const openMappingFromCode = () => setModal("mapping");

  const batchRefresh = useCallback(async () => {
    await refreshProject();
  }, [refreshProject]);

  // ------------------------------ 渲染 ------------------------------

  if (bootLoading) {
    return (
      <div className="app-shell">
        <div className="start-screen">
          <div className="start-card">
            <div className="start-logo">
              <ShieldCheck size={32} strokeWidth={1.9} />
            </div>
            <div className="start-title" style={{ fontSize: 22 }}>
              CodeQL 漏洞审计
            </div>
            <div className="mt-3 flex items-center justify-center gap-2 text-[12px] text-secondary">
              <Loader2 size={14} className="spin text-accent" />
              正在启动审计工作台…
            </div>
          </div>
        </div>
        <ToastStack items={toasts} onClose={(id) => setToasts((t) => t.filter((x) => x.id !== id))} />
      </div>
    );
  }

  if (bootError) {
    return (
      <div className="app-shell">
        <div className="start-screen">
          <div className="start-card">
            <div className="start-logo" style={{ background: "linear-gradient(135deg,#f87171,#dc2626)" }}>
              <AlertTriangle size={30} />
            </div>
            <div className="start-title" style={{ fontSize: 22 }}>
              后端未就绪
            </div>
            <p className="start-desc">{bootError}</p>
            <button className="btn btn-primary" onClick={() => window.location.reload()}>
              <RefreshCw size={14} /> 重新加载
            </button>
          </div>
        </div>
        <ToastStack items={toasts} onClose={(id) => setToasts((t) => t.filter((x) => x.id !== id))} />
      </div>
    );
  }

  return (
    <div className="app-shell pro-shell">
      <div className="tool-layer" style={{ display: tool === "hub" ? "flex" : "none" }}>
        <Hub
          version={boot?.version || "1.2.0"}
          platform={boot?.platform || "darwin"}
          theme={theme}
          lastProject={project?.csvPath || boot?.lastProject || ""}
          assistantRunning={assistantRunning}
          assistantStatus={assistantStatusLabel}
          onToggleTheme={toggleTheme}
          onOpenAudit={() => setTool("audit")}
          onOpenAssistant={() => setTool("assistant")}
        />
      </div>

      <div className="tool-layer" style={{ display: tool === "audit" ? "flex" : "none" }}>
      <TopBar
        platform={boot?.platform || "darwin"}
        onHome={() => setTool("hub")}
        project={project}
        theme={theme}
        lang={lang}
        searchShortcut={searchShortcut}
        infoCollapsed={infoCollapsed}
        onOpenCSV={() => void openCSV()}
        onOpenSrc={() => void chooseSrcRoot()}
        onExportCSV={() => void exportCurrent("csv")}
        onExportReport={() => void exportCurrent("report")}
        onExportVerdicts={() => void exportCurrent("verdicts")}
        onTranslate={() => void translateProject()}
        translating={translating}
        translationMissing={project?.stats.translationMissing || 0}
        onHistory={() => {
          void refreshHistory();
          setModal("history");
        }}
        onExitProject={() => void closeCurrentProject()}
        onFind={() => {
          if (project) setSearchOpen(true);
        }}
        onSettings={() => setModal("settings")}
        onLibrary={() => setModal("library")}
        onMapping={() => setModal("mapping")}
        onBatch={() => setModal("batch")}
        batchRunning={batch.running}
        batchStopping={batch.stopping}
        batchDone={batch.done}
        batchTotal={batch.total}
        onStopBatch={() => void stopBatch()}
        onToggleTheme={toggleTheme}
        onLangChange={changeLang}
        onToggleInfo={toggleInfo}
        onOpenConfigDir={() => {
          if (settings?.dataDir) api.RevealInFinder(settings.dataDir).catch((e) => notify("error", String(e)));
        }}
      />
      <ProjectStrip
        project={project}
        collapsed={infoCollapsed}
        onOpenSrc={() => void chooseSrcRoot()}
        onMapping={() => setModal("mapping")}
      />

      {!project ? (
        <StartScreen
          boot={boot}
          history={projectHistory}
          onOpenCSV={() => void openCSV()}
          onOpenSrc={() => void chooseSrcRoot()}
          onOpenProject={(item) => void openHistoryProject(item)}
          onDeleteProject={(item) => void deleteHistoryProject(item)}
          onOpenHistory={() => {
            void refreshHistory();
            setModal("history");
          }}
          loading={loading.active}
        />
      ) : (
        <div className="workspace" ref={workspaceRef}>
          <div
            className="flex min-h-0 min-w-0"
            style={{ flex: `0 0 ${leftRatio * 100}%`, minWidth: 240, maxWidth: 460 }}
          >
            <Sidebar
              project={project}
              findings={filtered}
              selectedKey={selectedKey}
              filter={filter}
              onFilterChange={setFilter}
              onSelect={selectFinding}
              onMarkAllRead={() => void markAllRead()}
              onResetRead={() => void resetRead()}
              onOpenMapping={() => setModal("mapping")}
            />
          </div>

          <div
            className="resizer"
            onMouseDown={startResize("left")}
            onDoubleClick={resetSplit}
            title="拖动调整左栏宽度（双击恢复默认）"
          />

          <div className="flex-1 min-w-0 flex">
            <CodePanel
              finding={selected}
              file={file}
              fileError={fileError}
              fileLoading={fileLoading}
              active={active}
              focusLine={focusLine}
              historyLength={history.length}
              showRuler={showRuler}
              theme={theme}
              searchOpen={searchOpen}
              searchShortcut={searchShortcut}
              onBack={() => void goBack()}
              onOpenEditor={() => void openInEditor()}
              onReveal={() => void reveal()}
              onToggleRuler={toggleRuler}
              onToggleSearch={() => setSearchOpen((v) => !v)}
              onOpenMapping={openMappingFromCode}
              onCopyStatus={(msg) => {
                notify("success", msg);
                setStatMessage(msg, "success", false);
              }}
            />
          </div>

          <div
            className="resizer"
            onMouseDown={startResize("right")}
            onDoubleClick={resetSplit}
            title="拖动调整右栏宽度（双击恢复默认）"
          />

          <div
            className="flex min-h-0 min-w-0"
            style={{ flex: `0 0 ${rightRatio * 100}%`, minWidth: 300, maxWidth: 560 }}
          >
            <DetailPanel
              finding={selected}
              lang={lang}
              active={active}
              verifying={verifyingKey === selected?.key}
              aiReady={
                !!settings?.aiVerifyEnabled &&
                !!settings?.llm.baseUrl?.trim() &&
                !!settings?.llm.model?.trim()
              }
              onVerify={() => void verify()}
              onClearVerdict={() => void clearVerdict()}
              onCopy={() => void copyChinese()}
              onCorrect={() => setModal("correction")}
              onJump={jumpTo}
              onJumpFlow={jumpToFlowStep}
              onLinkClick={jumpToLink}
              onOpenEditor={() => void openInEditor()}
              onOpenMapping={() => setModal("mapping")}
              onOpenSettings={() => setModal("settings")}
              onOpenExternal={openExternal}
            />
          </div>

          {translating && (
            <div className="loading-overlay">
              <div className="loading-card">
                <div className="flex items-center gap-2">
                  <div className="brand-mark" style={{ width: 26, height: 26 }}>
                    <Sparkles size={13} />
                  </div>
                  <div className="flex-1">
                    <div className="text-[12.5px] font-semibold text-primary">正在补全翻译</div>
                    <div className="text-[10px] text-tertiary whitespace-pre-wrap">
                      正在把翻译库中没有的规则名 / 描述 / 漏洞详情交给大模型，完成后立即刷新界面。
                    </div>
                  </div>
                  <Loader2 size={15} className="spin text-accent" />
                </div>
                <div className="progress-track">
                  <div
                    className={cn(
                      "progress-fill",
                      translateProgress.total <= 0 && "progress-indeterminate",
                    )}
                    style={
                      translateProgress.total > 0
                        ? {
                            width: `${(translateProgress.done / translateProgress.total) * 100}%`,
                          }
                        : undefined
                    }
                  />
                </div>
                {translateProgress.total > 0 && (
                  <div className="text-[9.5px] text-tertiary text-right font-mono">
                    {translateProgress.done} / {translateProgress.total}
                  </div>
                )}
              </div>
            </div>
          )}
        </div>
      )}

      {loading.active && (
        <div className="loading-overlay" style={{ zIndex: 60 }}>
          <div className="loading-card">
            <div className="flex items-center gap-2">
              <div className="brand-mark" style={{ width: 26, height: 26 }}>
                <Sparkles size={13} />
              </div>
              <div className="flex-1">
                <div className="text-[12.5px] font-semibold text-primary">正在载入项目</div>
                <div className="text-[10px] text-tertiary whitespace-pre-wrap">
                  {loading.message || "正在解析、翻译并定位源码…"}
                </div>
              </div>
              <Loader2 size={15} className="spin text-accent" />
            </div>
            <div className="progress-track">
              <div
                className={cn("progress-fill", loading.total <= 0 && "progress-indeterminate")}
                style={
                  loading.total > 0
                    ? { width: `${(loading.done / loading.total) * 100}%` }
                    : undefined
                }
              />
            </div>
            {loading.total > 0 && (
              <div className="text-[9.5px] text-tertiary text-right font-mono">
                {loading.done} / {loading.total}
              </div>
            )}
          </div>
        </div>
      )}

      <StatusBar
        project={project}
        message={status.message}
        tone={status.tone}
        busy={status.busy}
        version={boot?.version || "1.2.0"}
        platform={boot?.platform || "darwin"}
      />

      {tool === "audit" && (
        <ToastStack items={toasts} onClose={(id) => setToasts((t) => t.filter((x) => x.id !== id))} />
      )}

      <Modal
        open={!!confirm}
        onClose={() => setConfirm(null)}
        title={confirm?.title || "确认操作"}
        subtitle="请确认后继续"
        icon={AlertTriangle}
        size="sm"
        footer={
          <>
            <button className="btn btn-outline" onClick={() => setConfirm(null)}>
              取消
            </button>
            <button
              className={confirm?.danger ? "btn btn-danger" : "btn btn-primary"}
              onClick={() => {
                const action = confirm;
                setConfirm(null);
                if (action) void action.onConfirm();
              }}
            >
              {confirm?.confirmLabel || "确定"}
            </button>
          </>
        }
      >
        <div className="text-[12px] text-secondary leading-relaxed whitespace-pre-wrap">
          {confirm?.message}
        </div>
      </Modal>

      {settings && (
        <>
          <SettingsModal
            open={modal === "settings"}
            onClose={() => setModal(null)}
            settings={settings}
            defaults={{
              searchShortcut: boot?.defaultSearchShortcut || "Cmd+F",
              editorArgs: boot?.defaultEditorArgs || "-g {file}:{line}:{col}",
            }}
            currentFile={file}
            currentLine={focusLine}
            version={boot?.version || "1.2.0"}
            platform={boot?.platform || "darwin"}
            onSaved={onSettingsSaved}
            notify={notify}
          />
          <MappingModal
            open={modal === "mapping"}
            onClose={() => setModal(null)}
            onProjectChanged={batchRefresh}
            notify={notify}
          />
          <LibraryModal
            open={modal === "library"}
            onClose={() => setModal(null)}
            notify={notify}
          />
          <BatchModal
            open={modal === "batch"}
            onClose={() => setModal(null)}
            project={project}
            filtered={filtered}
            settings={settings}
            onProjectRefresh={batchRefresh}
            onSelectFinding={(key) => {
              setFilter(EMPTY_FILTER);
              setSelectedKey(key);
              setModal(null);
            }}
            onOpenSettings={() => setModal("settings")}
            notify={notify}
          />
          <CorrectionModal
            open={modal === "correction"}
            finding={selected}
            onClose={() => setModal(null)}
            onSaved={(updated, stats) => {
              setProject((p) =>
                p
                  ? {
                      ...p,
                      findings: p.findings.map((f) => (f.key === updated.key ? updated : f)),
                      stats,
                    }
                  : p,
              );
              setStatMessage("译文已更新", "success", false);
            }}
            notify={notify}
          />
          <ProjectHistoryModal
            open={modal === "history"}
            onClose={() => setModal(null)}
            projects={projectHistory}
            activeCsvPath={project?.csvPath || ""}
            onOpen={(item) => void openHistoryProject(item)}
            onDelete={(item) => void deleteHistoryProject(item)}
            onClear={() => void clearProjectHistory()}
            onRefresh={() => void refreshHistory()}
          />
        </>
      )}
      </div>

      <div className="tool-layer" style={{ display: tool === "assistant" ? "flex" : "none" }}>
        <AssistantWorkspace
          active={tool === "assistant"}
          version={boot?.version || "1.2.0"}
          theme={theme}
          onToggleTheme={toggleTheme}
          onHome={() => setTool("hub")}
          onRunningChange={(running, label) => {
            setAssistantRunning(running);
            setAssistantStatusLabel(label);
          }}
          onOpenAudit={(csvPath, srcRoot) =>
            setExternalAuditRequest({ csvPath, srcRoot })
          }
        />
      </div>
    </div>
  );
}
