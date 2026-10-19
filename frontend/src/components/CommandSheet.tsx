import { useEffect, useMemo, useState } from "react";
import {
  BookOpen,
  Check,
  Code2,
  Copy,
  Database,
  FolderTree,
  Info,
  PlayCircle,
  ShieldCheck,
  Sparkles,
  TerminalSquare,
} from "lucide-react";

import type { AssistantCommandsDTO, AssistantConfigDTO } from "../types";
import { api } from "../lib/api";
import { Modal, type ToastKind } from "./ui";
import { cn } from "../lib/format";

type SheetTab = "current" | "create" | "analyze" | "tips";

function initialSheetTab(): SheetTab {
  const value = new URLSearchParams(window.location.search).get("tab");
  return value === "create" || value === "analyze" || value === "tips" ? value : "current";
}

const TABS: { id: SheetTab; label: string; icon: typeof Code2 }[] = [
  { id: "current", label: "当前配置", icon: TerminalSquare },
  { id: "create", label: "创建数据库", icon: Database },
  { id: "analyze", label: "运行分析", icon: PlayCircle },
  { id: "tips", label: "小贴士", icon: Info },
];

const CREATE_EXAMPLES: { title: string; note?: string; command: string }[] = [
  {
    title: "Java · Maven",
    note: "编译型语言必须通过 --command 指定构建命令，CodeQL 会在构建过程中采集语义信息。",
    command: `codeql database create ~/codeql-db/java-demo \\
  --language=java \\
  --source-root=/path/to/java-project \\
  --command="mvn -B -DskipTests clean package" \\
  --threads=4 --ram=4096 --overwrite`,
  },
  {
    title: "Java · Gradle",
    command: `codeql database create ~/codeql-db/java-demo \\
  --language=java \\
  --source-root=/path/to/java-project \\
  --command="./gradlew --no-daemon clean compileJava" \\
  --threads=4 --ram=4096 --overwrite`,
  },
  {
    title: "Python",
    note: "解释型语言不需要 --command，CodeQL 会直接扫描源码。",
    command: `codeql database create ~/codeql-db/py-demo \\
  --language=python \\
  --source-root=/path/to/python-project \\
  --threads=4 --ram=4096 --overwrite`,
  },
  {
    title: "JavaScript / TypeScript",
    command: `codeql database create ~/codeql-db/js-demo \\
  --language=javascript \\
  --source-root=/path/to/web-project \\
  --threads=4 --ram=4096 --overwrite`,
  },
  {
    title: "Go",
    command: `codeql database create ~/codeql-db/go-demo \\
  --language=go \\
  --source-root=/path/to/go-project \\
  --threads=4 --ram=4096 --overwrite`,
  },
  {
    title: "C / C++",
    note: "需要可用的编译器；--command 应覆盖项目真实的编译过程。",
    command: `codeql database create ~/codeql-db/cpp-demo \\
  --language=cpp \\
  --source-root=/path/to/cpp-project \\
  --command="make -j4" \\
  --threads=4 --ram=4096 --overwrite`,
  },
  {
    title: "C#",
    command: `codeql database create ~/codeql-db/csharp-demo \\
  --language=csharp \\
  --source-root=/path/to/dotnet-project \\
  --command="dotnet build -c Release" \\
  --threads=4 --ram=4096 --overwrite`,
  },
];

const ANALYZE_EXAMPLES: { title: string; note?: string; command: string }[] = [
  {
    title: "官方安全套件 · SARIF v2.1.0",
    note: "security-extended 覆盖面更广；输出标准 SARIF v2.1.0，审计工作台可据此展示完整污点路径、CWE 与指纹。首次使用查询包时加 --download 让 CodeQL 自动下载。",
    command: `codeql database analyze ~/codeql-db/java-demo \\
  codeql/java-queries:codeql-suites/java-security-extended.qls \\
  --download \\
  --format=sarifv2.1.0 \\
  --output=~/java-results.sarif \\
  --threads=4 --ram=4096`,
  },
  {
    title: "输出 CSV · 兼容旧流程",
    note: "审计工作台同样支持 CSV。分析完成后回到首页，进入「审计工作台」打开这个文件即可。",
    command: `codeql database analyze ~/codeql-db/py-demo \\
  codeql/python-queries:codeql-suites/python-security-extended.qls \\
  --download \\
  --format=csv \\
  --output=~/py-results.csv \\
  --threads=4 --ram=4096`,
  },
  {
    title: "安全 + 代码质量套件",
    command: `codeql database analyze ~/codeql-db/js-demo \\
  codeql/javascript-queries:codeql-suites/javascript-security-and-quality.qls \\
  --download \\
  --format=sarifv2.1.0 \\
  --output=~/js-results.sarif \\
  --threads=4 --ram=4096`,
  },
  {
    title: "单个查询文件 / 自定义查询包",
    command: `# 单个 .ql 查询
codeql database analyze ~/codeql-db/go-demo \\
  /path/to/MyQuery.ql \\
  --format=csv --output=~/custom-results.csv

# 本地查询包目录
codeql database analyze ~/codeql-db/go-demo \\
  /path/to/my-query-pack \\
  --format=sarifv2.1.0 --output=~/pack-results.sarif`,
  },
  {
    title: "查看数据库信息",
    command: `codeql database info ~/codeql-db/java-demo

# 列出本机已安装的查询包
codeql pack ls ~/.codeql/packages`,
  },
];

export function CommandSheet({
  open,
  onClose,
  config,
  notify,
}: {
  open: boolean;
  onClose: () => void;
  config: AssistantConfigDTO;
  notify: (kind: ToastKind, message: string) => void;
}) {
  const [tab, setTab] = useState<SheetTab>(initialSheetTab);
  const [preview, setPreview] = useState<AssistantCommandsDTO | null>(null);
  const [copied, setCopied] = useState("");

  useEffect(() => {
    if (!open) return;
    setTab(initialSheetTab());
  }, [open]);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    const timer = window.setTimeout(() => {
      api
        .AssistantPreviewCommands(config)
        .then((next) => {
          if (!cancelled) setPreview(next);
        })
        .catch(() => {
          if (!cancelled) setPreview(null);
        });
    }, 180);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [open, config]);

  const copy = async (key: string, text: string) => {
    try {
      await api.CopyText(text);
      setCopied(key);
      window.setTimeout(() => setCopied((cur) => (cur === key ? "" : cur)), 1600);
      notify("success", "命令已复制到剪贴板");
    } catch (err) {
      notify("error", String(err));
    }
  };

  const title = useMemo(
    () => (
      <span className="flex items-center gap-2">
        命令示例
        <span className="badge" style={{ background: "var(--cyan-soft)", color: "var(--cyan)" }}>
          可独立执行
        </span>
      </span>
    ),
    [],
  );

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      subtitle="这里的所有命令都可以复制到系统终端里直接运行，不依赖 CodeQLAuditPro。当前配置页会按你填写的路径实时生成命令。"
      icon={BookOpen}
      size="xl"
      footer={
        <>
          <span className="text-[10px] text-tertiary mr-auto">
            提示：路径含空格时会自动加引号；Windows 建议在 Git Bash / WSL 中执行，PowerShell 需把行尾的 \ 换成反引号 `。
          </span>
          <button className="btn btn-outline" onClick={onClose}>
            关闭
          </button>
        </>
      }
    >
      <div className="command-sheet">
        <nav className="command-tabs">
          {TABS.map(({ id, label, icon: Icon }) => (
            <button
              key={id}
              className={cn("command-tab", tab === id && "active")}
              data-autofocus={tab === id ? true : undefined}
              onClick={() => setTab(id)}
            >
              <Icon size={13} />
              {label}
            </button>
          ))}
        </nav>

        {tab === "current" && (
          <section className="command-panel">
            <div className="command-intro">
              <ShieldCheck size={15} className="text-accent flex-none" />
              <div>
                <div className="text-[12px] font-semibold text-primary">与「创建数据库 / 开始分析」完全一致</div>
                <div className="text-[11px] text-secondary mt-0.5">
                  下面两条命令使用与运行器相同的参数组装逻辑。填好左侧配置后，复制到终端即可脱离界面执行。
                </div>
              </div>
            </div>
            <CodeBlock
              icon={Database}
              title="1. 创建 CodeQL 数据库"
              command={preview?.create || "正在生成…"}
              copyKey="current-create"
              copied={copied === "current-create"}
              onCopy={() => preview && copy("current-create", preview.create)}
            />
            <CodeBlock
              icon={PlayCircle}
              title="2. 运行查询并输出结果"
              command={preview?.analyze || "正在生成…"}
              copyKey="current-analyze"
              copied={copied === "current-analyze"}
              onCopy={() => preview && copy("current-analyze", preview.analyze)}
            />
            <div className="command-note">
              <Info size={13} className="flex-none text-cyan" />
              <span>
                如果「查询 / 套件」留空，CodeQL 会自动选择默认查询；推荐填写官方套件，例如
                <code>codeql/java-queries:codeql-suites/java-security-extended.qls</code>，
                这样规则范围更可控。
              </span>
            </div>
          </section>
        )}

        {tab === "create" && (
          <section className="command-panel">
            <ExampleGroup
              items={CREATE_EXAMPLES}
              prefix="create"
              copied={copied}
              onCopy={copy}
            />
            <div className="command-note">
              <FolderTree size={13} className="flex-none text-cyan" />
              <span>
                <code>--source-root</code> 指向源码根目录，<code>--database</code> 输出目录可以不存在，CodeQL 会自动创建。
                编译型语言（Java、C/C++、C#、Go 的编译模式等）必须提供 <code>--command</code>。
              </span>
            </div>
          </section>
        )}

        {tab === "analyze" && (
          <section className="command-panel">
            <ExampleGroup
              items={ANALYZE_EXAMPLES}
              prefix="analyze"
              copied={copied}
              onCopy={copy}
            />
            <div className="command-note">
              <Sparkles size={13} className="flex-none text-accent" />
              <span>
                常用套件：<code>security-extended</code>（安全规则更全）、
                <code>security-and-quality</code>（安全 + 代码质量）。输出格式只使用
                <code>sarifv2.1.0</code> 与 <code>csv</code>：把 <code>--format=csv</code> 的结果
                交给「审计工作台」，就能获得三栏可点、可跳、可读的审计体验。
              </span>
            </div>
          </section>
        )}

        {tab === "tips" && (
          <section className="command-panel">
            <div className="tip-grid">
              <TipCard title="先确认 CLI 可用" icon={TerminalSquare}>
                <code>codeql version</code>
                <br />
                <code>codeql resolve languages</code>
                <p>
                  如果终端能用、应用里找不到，通常是 GUI 没有继承终端 PATH。CodeQLAuditPro 启动时会自动读取
                  zsh/bash 登录环境，也可以在「CodeQL 二进制文件」里直接选择绝对路径。
                </p>
              </TipCard>
              <TipCard title="下载官方查询包" icon={Database}>
                <code>codeql pack download codeql/java-queries</code>
                <p>
                  首次分析时加 <code>--download</code> 也可以让 CodeQL 自动下载。查询包默认缓存在
                  <code>~/.codeql/packages</code>。
                </p>
              </TipCard>
              <TipCard title="选择输出格式" icon={Code2}>
                <p>
                  运行器只提供两种输出格式：
                  <br />• <code>sarifv2.1.0</code>：标准 SARIF，审计工作台可展示完整 codeFlow / CWE / 指纹，也供 GitHub Code Scanning、CI 使用
                  <br />• <code>csv</code>：兼容旧流程与轻量导出
                </p>
              </TipCard>
              <TipCard title="资源不够时" icon={Info}>
                <p>
                  大型项目可以调低 <code>--threads</code>、调高 <code>--ram</code>（单位 MB）。
                  把内存限额或线程数设为 <code>0</code>，生成的命令会省略对应的
                  <code>--ram</code> / <code>--threads</code>，交给 CodeQL 自己决定。
                  如果构建命令依赖特定 JDK / Node 版本，请先在终端里确认能构建成功，再回到应用里执行。
                </p>
              </TipCard>
              <TipCard title="分析结果怎么用" icon={ShieldCheck}>
                <p>
                  以 <code>--format=sarifv2.1.0</code> 或 <code>--format=csv</code> 输出后，回到首页进入「审计工作台」，
                  点「打开结果文件」选择该文件，再选择源码目录即可开始审计。已读状态和 AI 判定会按结果文件路径持久化。
                </p>
              </TipCard>
              <TipCard title="完全脱离本工具" icon={Sparkles}>
                <p>
                  本页面生成的命令不包含任何 CodeQLAuditPro 私有参数，复制到任意安装了 CodeQL CLI 的机器上都能执行。
                  应用只是帮你把参数拼好、把日志收好。
                </p>
              </TipCard>
            </div>
          </section>
        )}
      </div>
    </Modal>
  );
}

function ExampleGroup({
  items,
  prefix,
  copied,
  onCopy,
}: {
  items: { title: string; note?: string; command: string }[];
  prefix: string;
  copied: string;
  onCopy: (key: string, text: string) => void;
}) {
  return (
    <div className="example-list">
      {items.map((item, index) => {
        const key = `${prefix}-${index}`;
        return (
          <div key={key} className="example-item">
            <div className="example-head">
              <div className="flex items-center gap-2 min-w-0">
                <span className="example-index">{String(index + 1).padStart(2, "0")}</span>
                <span className="text-[12px] font-semibold text-primary truncate">{item.title}</span>
              </div>
              <button className="btn btn-sm" onClick={() => onCopy(key, item.command)}>
                {copied === key ? <Check size={12} /> : <Copy size={12} />}
                {copied === key ? "已复制" : "复制"}
              </button>
            </div>
            {item.note && <div className="example-note">{item.note}</div>}
            <pre className="code-block">{item.command}</pre>
          </div>
        );
      })}
    </div>
  );
}

function CodeBlock({
  icon: Icon,
  title,
  command,
  copyKey,
  copied,
  onCopy,
}: {
  icon: typeof Database;
  title: string;
  command: string;
  copyKey: string;
  copied: boolean;
  onCopy: () => void;
}) {
  return (
    <div className="command-block">
      <div className="command-block-head">
        <div className="flex items-center gap-2">
          <Icon size={14} className="text-accent" />
          <span className="text-[12px] font-semibold text-primary">{title}</span>
        </div>
        <button className="btn btn-sm btn-soft" onClick={onCopy} data-copy={copyKey}>
          {copied ? <Check size={12} /> : <Copy size={12} />}
          {copied ? "已复制" : "复制命令"}
        </button>
      </div>
      <pre className="code-block code-block--large">{command}</pre>
    </div>
  );
}

function TipCard({
  title,
  icon: Icon,
  children,
}: {
  title: string;
  icon: typeof Info;
  children: React.ReactNode;
}) {
  return (
    <div className="tip-card">
      <div className="flex items-center gap-2">
        <div className="tip-icon">
          <Icon size={13} />
        </div>
        <span className="text-[12px] font-semibold text-primary">{title}</span>
      </div>
      <div className="tip-body">{children}</div>
    </div>
  );
}
