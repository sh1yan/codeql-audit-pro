import {
  ChevronDown,
  ChevronUp,
  Download,
  FileSpreadsheet,
  FileText,
  FolderOpen,
  FolderTree,
  History,
  Home,
  Languages,
  LayoutGrid,
  Library,
  Loader2,
  LogOut,
  Moon,
  Route,
  Search,
  Settings2,
  ShieldCheck,
  Sparkles,
  Sun,
  Wand2,
} from "lucide-react";
import type { Lang, ProjectDTO, Theme } from "../types";
import { Dropdown, MenuAction, MenuDivider, MenuLabel, Tooltip } from "./ui";
import { cn } from "../lib/format";

const LANG_LABEL: Record<Lang, string> = {
  zh: "中文",
  en: "英文",
  both: "中英对照",
};

// 顶栏精简开关：这些入口在别处都有等价入口，默认不展示。
//   历史项目 / 退出项目 / 补全翻译 → 「功能」菜单里仍然可用
//   查找代码 → 快捷键（默认 Cmd+F）与代码区的「查找」仍然可用
//   设置 → 右上角齿轮按钮仍然可用
// 需要重新展示时，把对应开关改成 true 即可。
const TOPBAR_VISIBILITY = {
  history: false,
  exitProject: false,
  find: false,
  translate: false,
  settingsMenuItem: false,
};

export function TopBar({
  platform,
  onHome,
  project,
  theme,
  lang,
  searchShortcut,
  infoCollapsed,
  onOpenCSV,
  onOpenSrc,
  onExportCSV,
  onExportReport,
  onExportVerdicts,
  onTranslate,
  translating,
  translationMissing,
  onHistory,
  onExitProject,
  onFind,
  onSettings,
  onLibrary,
  onMapping,
  onBatch,
  onToggleTheme,
  onLangChange,
  onToggleInfo,
  onOpenConfigDir,
}: {
  platform: string;
  onHome: () => void;
  project: ProjectDTO | null;
  theme: Theme;
  lang: Lang;
  searchShortcut: string;
  infoCollapsed: boolean;
  onOpenCSV: () => void;
  onOpenSrc: () => void;
  onExportCSV: () => void;
  onExportReport: () => void;
  onExportVerdicts: () => void;
  onTranslate: () => void;
  translating: boolean;
  translationMissing: number;
  onHistory: () => void;
  onExitProject: () => void;
  onFind: () => void;
  onSettings: () => void;
  onLibrary: () => void;
  onMapping: () => void;
  onBatch: () => void;
  onToggleTheme: () => void;
  onLangChange: (lang: Lang) => void;
  onToggleInfo: () => void;
  onOpenConfigDir: () => void;
}) {
  return (
    <header className={cn("topbar", platform === "darwin" && "topbar--mac")}>
      <button className="btn no-drag" onClick={onHome} title="返回工具选择">
        <Home size={14} />
        <span className="hide-narrow">首页</span>
      </button>
      {TOPBAR_VISIBILITY.history && (
        <button
          className="btn btn-icon no-drag"
          onClick={onHistory}
          title="历史审计项目（查看 / 删除）"
        >
          <History size={14} />
        </button>
      )}
      {TOPBAR_VISIBILITY.exitProject && (
        <button
          className="btn no-drag"
          onClick={onExitProject}
          disabled={!project}
          title="退出当前项目，回到审计工作台选择界面"
        >
          <LogOut size={14} />
          <span className="hide-narrow">退出项目</span>
        </button>
      )}
      <div className="brand">
        <div className="brand-mark">
          <ShieldCheck size={17} strokeWidth={2.1} />
        </div>
        <div className="flex flex-col leading-none">
          <span className="brand-title">CodeQLAuditPro</span>
          <span className="brand-sub text-[8.5px] text-tertiary tracking-[0.08em] mt-1">
            CODEQL AUDIT WORKSPACE
          </span>
        </div>
        <span className="brand-tag">Pro</span>
      </div>

      <div className="topbar-sep" />

      <button className="btn btn-primary no-drag" onClick={onOpenCSV} title="打开 CodeQL 结果文件（CSV / SARIF v2.1.0）">
        <FolderOpen size={14} />
        打开结果文件
      </button>
      <button className="btn no-drag" onClick={onOpenSrc} disabled={!project} title="选择被审计的源码根目录">
        <FolderTree size={14} />
        <span className="hide-narrow">源码目录</span>
      </button>

      <div className="topbar-sep" />

      <Dropdown
        trigger={({ onClick }) => (
          <button className="btn" onClick={onClick} disabled={!project} title="导出">
            <Download size={14} />
            <span className="hide-narrow">导出</span>
            <ChevronDown size={11} className="opacity-60" />
          </button>
        )}
      >
        <MenuLabel>导出</MenuLabel>
        <MenuAction icon={FileSpreadsheet} onClick={onExportCSV}>
          中文 CSV
          <span className="text-[9px] text-tertiary ml-1">当前筛选</span>
        </MenuAction>
        <MenuAction icon={FileText} onClick={onExportReport}>
          审计报告（32 列 · 含 SARIF 字段）
        </MenuAction>
        <MenuDivider />
        <MenuAction icon={Sparkles} onClick={onExportVerdicts}>
          AI 验证结果 CSV
        </MenuAction>
      </Dropdown>

      {TOPBAR_VISIBILITY.find && (
        <button className="btn no-drag" onClick={onFind} disabled={!project} title={`查找代码（${searchShortcut}）`}>
          <Search size={14} />
          <span className="hide-narrow">查找代码</span>
          <span className="kbd ml-1 hide-narrow">{searchShortcut}</span>
        </button>
      )}
      <button className="btn no-drag" onClick={onBatch} disabled={!project} title="批量 AI 验证">
        <Wand2 size={14} />
        <span className="hide-narrow">批量 AI 验证</span>
      </button>
      {TOPBAR_VISIBILITY.translate && (
        <button
          className="btn no-drag"
          onClick={onTranslate}
          disabled={!project || translating}
          title="立即翻译当前项目中缺失的规则名 / 描述 / 漏洞详情，并刷新界面"
        >
          {translating ? <Loader2 size={14} className="spin" /> : <Languages size={14} />}
          <span className="hide-narrow">{translating ? "翻译中…" : "补全翻译"}</span>
          {translationMissing > 0 && !translating && (
            <span
              className="badge bg-sev-warning-soft text-sev-warning border-sev-warning/25"
              title={`${translationMissing} 条内容仍缺中文`}
            >
              {translationMissing}
            </span>
          )}
        </button>
      )}

      <div className="topbar-sep" />

      <Dropdown
        trigger={({ onClick }) => (
          <button
            className="btn"
            onClick={onClick}
            title="工作区功能：历史项目、翻译、路径映射、数据目录与项目切换"
          >
            <LayoutGrid size={14} />
            <span className="hide-narrow">功能</span>
            <ChevronDown size={11} className="opacity-60" />
          </button>
        )}
      >
        <MenuLabel>工作区功能</MenuLabel>
        <MenuAction icon={History} onClick={onHistory}>
          历史项目
          <span className="text-[9px] text-tertiary ml-1">查看 / 删除</span>
        </MenuAction>
        <MenuAction icon={Languages} onClick={onTranslate} disabled={!project || translating}>
          补全翻译
          {translationMissing > 0 ? (
            <span className="text-[9px] text-tertiary ml-1">{translationMissing} 条待翻译</span>
          ) : null}
        </MenuAction>
        <MenuAction icon={Library} onClick={onLibrary}>
          翻译库
        </MenuAction>
        <MenuAction icon={Route} onClick={onMapping}>
          路径映射
        </MenuAction>
        {TOPBAR_VISIBILITY.settingsMenuItem && (
          <MenuAction icon={Settings2} onClick={onSettings}>
            设置
          </MenuAction>
        )}
        <MenuDivider />
        <MenuAction icon={FolderOpen} onClick={onOpenConfigDir}>
          打开数据目录
        </MenuAction>
        <MenuAction icon={LogOut} onClick={onExitProject} disabled={!project} danger>
          退出当前项目
        </MenuAction>
      </Dropdown>

      <button className="btn btn-icon no-drag" onClick={onSettings} title="设置">
        <Settings2 size={15} />
      </button>

      <div className="flex-1" />

      <Tooltip tip={infoCollapsed ? "展开项目信息" : "收起项目信息"}>
        <button className="btn btn-icon no-drag" onClick={onToggleInfo} disabled={!project} title={infoCollapsed ? "展开项目信息" : "收起项目信息"}>
          {infoCollapsed ? <ChevronDown size={15} /> : <ChevronUp size={15} />}
        </button>
      </Tooltip>

      <Tooltip tip={theme === "dark" ? "切换到浅色主题" : "切换到深色主题"}>
        <button className="btn btn-icon no-drag" onClick={onToggleTheme} title={theme === "dark" ? "切换到浅色主题" : "切换到深色主题"}>
          {theme === "dark" ? <Sun size={15} /> : <Moon size={15} />}
        </button>
      </Tooltip>

      <Dropdown
        trigger={({ onClick }) => (
          <button className="btn no-drag" onClick={onClick} title={`详情语言：${LANG_LABEL[lang]}`}>
            <Languages size={14} />
            <span className="hide-narrow">{LANG_LABEL[lang]}</span>
            <ChevronDown size={11} className="opacity-60" />
          </button>
        )}
      >
        <MenuLabel>详情页语言</MenuLabel>
        <MenuAction onClick={() => onLangChange("zh")}>中文</MenuAction>
        <MenuAction onClick={() => onLangChange("en")}>英文</MenuAction>
        <MenuAction onClick={() => onLangChange("both")}>中英对照</MenuAction>
      </Dropdown>

    </header>
  );
}

export function ProjectStrip({
  project,
  collapsed,
  onOpenSrc,
  onMapping,
}: {
  project: ProjectDTO | null;
  collapsed: boolean;
  onOpenSrc: () => void;
  onMapping: () => void;
}) {
  if (!project || collapsed) return null;
  const st = project.stats;
  // 后端旧版本可能把空解析提示序列化成 null，直接读 .length 会让整页渲染崩掉。
  const warnings = project.warnings ?? [];
  const resolvedPct = st.total ? Math.round((st.resolved / st.total) * 100) : 0;
  return (
    <div className="project-strip">
      <div className="flex items-center gap-1.5 min-w-0">
        <FileSpreadsheet size={12} className="text-accent flex-none" />
        <span className="text-tertiary flex-none">结果</span>
        <span className="path" title={project.csvPath}>
          {project.csvPath}
        </span>
        <span className="chip chip-active flex-none font-mono" title={project.toolName ? `${project.toolName} ${project.toolVersion}` : undefined}>
          {project.formatLabel || "CSV"}
        </span>
      </div>
      <div className="w-px h-3.5 bg-[var(--border)] flex-none" />
      <button
        className="flex items-center gap-1.5 min-w-0 bg-transparent border-0 p-0 cursor-pointer text-secondary hover:text-primary"
        onClick={onOpenSrc}
        title="更换源码目录"
      >
        <FolderTree size={12} className="flex-none text-cyan" />
        <span className="text-tertiary flex-none">源码</span>
        <span className="path" title={project.srcRoot || "未设置"}>
          {project.srcRoot || "（未设置，源码跳转不可用）"}
        </span>
      </button>
      <div className="flex-1" />
      <span
        className={cn(
          "chip",
          resolvedPct === 100 ? "chip-active" : "",
        )}
        title="已定位到本地源码的比例"
      >
        <span className="w-1.5 h-1.5 rounded-full bg-current" />
        已定位 {st.resolved}/{st.total}
      </span>
      {st.withCodeFlow > 0 && (
        <span className="chip" title="带完整 SARIF codeFlow 的告警数量">
          污点路径 {st.withCodeFlow}
        </span>
      )}
      {st.cweKinds > 0 && (
        <span className="chip" title="去重后的 CWE 种类数">
          CWE {st.cweKinds}
        </span>
      )}
      {st.missing > 0 && (
        <button className="chip" onClick={onMapping} title="打开路径映射">
          {st.missing} 条未定位
        </button>
      )}
      {warnings.length > 0 && (
        <span className="chip" title={warnings.join("\n")}>
          解析提示 {warnings.length}
        </span>
      )}
      <span className="text-[10px] text-tertiary whitespace-nowrap">
        载入于 {new Date(project.loadedAt).toLocaleTimeString("zh-CN", { hour12: false })}
      </span>
    </div>
  );
}
