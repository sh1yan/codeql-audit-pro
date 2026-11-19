/* eslint-disable */
// 浏览器开发模式下的 mock 后端。
//
// 只有在 DEV 且没有 Wails 运行时时才会被动态加载，生产包不会执行这里。
// 数据基于 .demo/results.csv 的形态手工构造，用于视觉自检与交互演示。

import type {
  AIDTO,
  AppApi,
  AssistantCommandsDTO,
  AssistantConfigDTO,
  AssistantHistoryDTO,
  AssistantStateDTO,
  AssistantStatusDTO,
  BatchProgressDTO,
  BatchStateDTO,
  BatchSummaryDTO,
  BootstrapDTO,
  CodeFlowDTO,
  FileDTO,
  FindingDTO,
  FlowStepDTO,
  LibraryDTO,
  LocationDTO,
  MappingDTO,
  ProjectDTO,
  ProjectHistoryDTO,
  RelatedLocationDTO,
  RuleInfoDTO,
  SegmentDTO,
  SettingsDTO,
  StatsDTO,
  TranslateProjectDTO,
} from "../types";
import { emitLocal } from "../lib/api";

const ROOT = "/Users/shiyan/Downloads/VibeCoding/.demo/src";
const CSV_PATH = "/Users/shiyan/Downloads/VibeCoding/.demo/pyhackme.sarif";

const SOURCE_FILES: Record<string, string[]> = {
  "app/common/sql.py": `import sqlite3

def query(uid):
    conn = sqlite3.connect('app.db')
    cur = conn.cursor()
    sql = "SELECT * FROM users WHERE id = '" + uid + "'"
    cur.execute(sql)
    return cur.fetchall()


def query_safe(uid):
    conn = sqlite3.connect('app.db')
    cur = conn.cursor()
    cur.execute("SELECT * FROM users WHERE id = ?", (uid,))
    return cur.fetchall()


def health():
    return {"ok": True}
`.split("\n"),
  "app/views/user.py": `from flask import request, render_template_string, make_response

def profile():
    uid = request.args.get('id')
    return render_template_string('<h1>' + uid + '</h1>')


def report():
    try:
        raise RuntimeError('boom')
    except Exception as exc:
        return str(exc)


def render_page():
    name = request.args.get('name', 'guest')
    return '<div>Hello ' + name + '</div>'
`.split("\n"),
  "app/views/admin.py": `from flask import request
import os
import hashlib

def run():
    cmd = request.args.get('cmd')
    os.system(cmd)
    return 'ok'


def digest(value):
    return hashlib.md5(value.encode()).hexdigest()


def login():
    password = "admin123"
    return password == request.args.get("password")
`.split("\n"),
  "app/views/upload.py": `from flask import request
import os

def save():
    name = request.args.get('name')
    open('/tmp/' + name, 'w').write(request.data)
    return 'saved'


def store_secret():
    secret = request.args.get('secret')
    with open('/tmp/secret.txt', 'w') as fh:
        fh.write(secret)
    return 'stored'


def clean():
    return 'ok'
`.split("\n"),
};

const abs = (rel: string) => `${ROOT}/${rel}`;

interface RuleDef {
  name: string;
  zh: string;
  description: string;
  descriptionZh: string;
  severity: string;
  messageZh: string;
  messageEn: string;
}

const RULES: RuleDef[] = [
  {
    name: "py/sql-injection",
    zh: "SQL 注入",
    description: "SQL query built from user-controlled sources",
    descriptionZh: "把用户可控的数据拼接进 SQL 查询，可能导致 SQL 注入。",
    severity: "error",
    messageZh: "这条查询依赖来自(1)的数据，未经参数化处理。",
    messageEn: "This query depends on a user-provided value.",
  },
  {
    name: "py/command-line-injection",
    zh: "命令注入",
    description: "Command line built from user-controlled sources",
    descriptionZh: "把用户可控的数据拼接进系统命令，可能执行任意命令。",
    severity: "error",
    messageZh: "命令字符串由(1)拼接而成，可能执行任意系统命令。",
    messageEn: "This command depends on a user-provided value.",
  },
  {
    name: "py/path-injection",
    zh: "路径注入",
    description: "Uncontrolled data used in path expression",
    descriptionZh: "未净化的用户输入参与文件路径拼接，可读写任意文件。",
    severity: "error",
    messageZh: "文件路径来自(1)，未经规范化校验。",
    messageEn: "The path depends on a user-provided value.",
  },
  {
    name: "py/stack-trace-exposure",
    zh: "堆栈信息泄露",
    description: "Stack trace information is exposed to the user",
    descriptionZh: "异常堆栈被直接返回给用户，可能泄露内部实现细节。",
    severity: "warning",
    messageZh: "异常堆栈被直接返回给用户，可能泄露内部实现细节。",
    messageEn: "Leaking information from an exception.",
  },
  {
    name: "py/reflective-xss",
    zh: "反射型 XSS",
    description: "Reflected cross-site scripting",
    descriptionZh: "用户输入被直接回显到页面，缺少 HTML 转义。",
    severity: "warning",
    messageZh: "用户输入被直接回显到页面，缺少转义。",
    messageEn: "Cross-site scripting vulnerability.",
  },
  {
    name: "py/clear-text-storage",
    zh: "明文存储敏感信息",
    description: "Cleartext storage of sensitive information",
    descriptionZh: "敏感数据以明文形式写入存储介质。",
    severity: "warning",
    messageZh: "敏感数据以明文形式写入存储，建议加密后再落盘。",
    messageEn: "Sensitive data is stored unencrypted.",
  },
  {
    name: "py/weak-crypto",
    zh: "弱加密算法",
    description: "Use of a weak cryptographic algorithm",
    descriptionZh: "使用了 MD5 等不具备抗碰撞能力的哈希算法。",
    severity: "note",
    messageZh: "使用了 MD5 等不具备抗碰撞能力的哈希算法。",
    messageEn: "MD5 is not collision resistant.",
  },
  {
    name: "py/hardcoded-credentials",
    zh: "硬编码凭据",
    description: "Hard-coded credentials",
    descriptionZh: "源码中硬编码了账号、口令或密钥。",
    severity: "warning",
    messageZh: "源码中硬编码了凭据，建议改由环境变量或密钥管理服务提供。",
    messageEn: "Credentials should not be hard-coded.",
  },
  {
    name: "py/recommendation/inefficient",
    zh: "低效字符串拼接",
    description: "Inefficient string concatenation in loop",
    descriptionZh: "循环中反复拼接字符串，建议使用 join() 或列表收集。",
    severity: "recommendation",
    messageZh: "循环中反复拼接字符串，建议使用 join()。",
    messageEn: "Consider using join().",
  },
  {
    name: "js/unsafe-deserialization",
    zh: "不安全的反序列化",
    description: "Deserialization of untrusted data",
    descriptionZh: "反序列化不可信数据可能导致远程代码执行。",
    severity: "error",
    messageZh: "反序列化来自(1)的对象，可能触发远程代码执行。",
    messageEn: "Deserialization of untrusted data.",
  },
  {
    name: "java/log-injection",
    zh: "日志注入",
    description: "Log entries created from user input",
    descriptionZh: "未经净化的用户输入被写入日志，可能伪造日志行。",
    severity: "warning",
    messageZh: "未经净化的(1)写入日志，可能伪造日志行。",
    messageEn: "Log entries created from user input.",
  },
  {
    name: "go/ssrf",
    zh: "服务端请求伪造",
    description: "Server-side request forgery",
    descriptionZh: "请求目标 URL 来自用户输入，可能导致 SSRF。",
    severity: "error",
    messageZh: "请求目标来自(1)，可能导致服务端请求伪造。",
    messageEn: "The request target depends on a user-provided value.",
  },
];

const SOURCE_FILES_LIST = Object.keys(SOURCE_FILES);

function makeLocation(
  file: string,
  line: number,
  col: number,
  label = "",
  labelZh = "",
  unresolved = false,
): LocationDTO {
  const endCol = col + 5 + (line % 4);
  const path = unresolved ? "" : abs(file);
  return {
    file,
    resolvedPath: path,
    startLine: line,
    startCol: col,
    endLine: line,
    endCol,
    label,
    labelZh,
    resolved: !unresolved,
    lineRange: String(line),
    columnRange: `${col}-${endCol}`,
    lineCol: `${line}:${col}`,
    displayPath: file,
    baseName: file.split("/").pop() || file,
  };
}

function linkForFile(file: string, line: number, col: number, unresolved = false) {
  return {
    label: "user-provided value",
    labelZh: "用户可控输入",
    rawUri: `relative:///${file}:${line}:${col}:${line}:${col + 6}`,
    file,
    startLine: line,
    startCol: col,
    endLine: line,
    endCol: col + 6,
    resolved: !unresolved,
  };
}

function resolveTemplate(template: string, linkText: string): SegmentDTO[] {
  const out: SegmentDTO[] = [];
  const marker = template.indexOf("(1)");
  if (marker < 0) return [{ kind: "text", text: template }];
  const before = template.slice(0, marker);
  const after = template.slice(marker + 3);
  out.push({ kind: "text", text: before });
  out.push({ kind: "link", text: linkText });
  if (after) out.push({ kind: "text", text: after });
  return out;
}

function statsOf(findings: FindingDTO[]): StatsDTO {
  const bySeverity: Record<string, number> = {};
  let resolved = 0;
  let unread = 0;
  let verified = 0;
  let falsePositive = 0;
  let truePositive = 0;
  let uncertain = 0;
  let pending = 0;
  let translationMissing = 0;
  let sources = 0;
  let withCodeFlow = 0;
  const rules = new Set<string>();
  const files = new Set<string>();
  const cwes = new Set<string>();
  for (const f of findings) {
    bySeverity[f.severity] = (bySeverity[f.severity] || 0) + 1;
    if (f.sink.resolved) resolved++;
    if (!f.reviewed) unread++;
    if (f.ai) {
      verified++;
      if (f.ai.verdict === "false_positive") falsePositive++;
      else if (f.ai.verdict === "true_positive") truePositive++;
      else if (f.ai.verdict === "uncertain") uncertain++;
    }
    if (f.translatePending) pending++;
    if (
      f.translatePending ||
      !f.ruleNameZh ||
      f.ruleNameZh === f.ruleName ||
      !f.messageZhTemplate
    ) {
      translationMissing++;
    }
    sources += f.sources.length;
    rules.add(f.ruleName);
    files.add(f.sink.file);
    if (f.codeFlows.length > 0) withCodeFlow++;
    for (const cwe of f.cwes) cwes.add(cwe);
  }
  return {
    total: findings.length,
    ruleKinds: rules.size,
    fileKinds: files.size,
    resolved,
    missing: findings.length - resolved,
    unread,
    pending,
    translationMissing,
    verified,
    falsePositive,
    truePositive,
    uncertain,
    translationBuiltin: 141,
    translationStore: 0,
    translationLLM: 0,
    bySeverity,
    totalSourceLocation: sources,
    withCodeFlow,
    cweKinds: cwes.size,
  };
}

function buildFindings(): FindingDTO[] {
  const list: FindingDTO[] = [];
  for (let i = 0; i < 47; i++) {
    const rule = RULES[i % RULES.length];
    const file = SOURCE_FILES_LIST[i % SOURCE_FILES_LIST.length];
    const lines = SOURCE_FILES[file];
    const line = 3 + ((i * 3) % Math.max(1, lines.length - 3));
    const col = 5 + ((i * 7) % 22);
    const sourceFile = SOURCE_FILES_LIST[(i + 2) % SOURCE_FILES_LIST.length];
    const sourceLines = SOURCE_FILES[sourceFile];
    const sourceLine = 3 + ((i * 5) % Math.max(1, sourceLines.length - 3));
    const sourceCol = 9 + ((i * 3) % 12);
    const unresolved = i === 17 || i === 31 || i === 42;
    const sinkFile = unresolved ? "app/legacy/handler.py" : file;
    const sink = makeLocation(sinkFile, unresolved ? 6 + (i % 4) : line, col, "", "", unresolved);
    const source = makeLocation(sourceFile, sourceLine, sourceCol, "user-provided value", "用户可控输入");
    const link = linkForFile(sourceFile, sourceLine, sourceCol);
    const linkText = "用户可控输入";
    const messageZh = resolveTemplate(rule.messageZh, linkText);
    const messageEn: SegmentDTO[] = [{ kind: "text", text: rule.messageEn }];
    if (rule.messageZh.includes("(1)")) {
      messageEn.push({ kind: "link", text: "user-provided value", link });
    }

    let ai: AIDTO | null = null;
    if (i === 0 || i % 9 === 0) {
      ai = {
        verdict: "false_positive",
        verdictZh: "误报",
        reason: "该位置使用了参数化查询，用户输入不会改变 SQL 结构，判定为误报。",
        confidence: 0.87,
        model: "deepseek-chat",
        at: "2026-10-10T15:33:00+08:00",
        short: "AI 误报",
      };
    } else if (i % 13 === 0) {
      ai = {
        verdict: "true_positive",
        verdictZh: "真实漏洞",
        reason: "输入直接拼接进命令字符串且没有白名单校验，攻击者可执行任意命令。",
        confidence: 0.92,
        model: "deepseek-chat",
        at: "2026-10-10T15:40:00+08:00",
        short: "AI 确认",
      };
    } else if (i % 11 === 0) {
      ai = {
        verdict: "uncertain",
        verdictZh: "存疑",
        reason: "上下文中存在一个未识别的校验函数，无法确认是否足以阻断攻击。",
        confidence: 0.55,
        model: "deepseek-chat",
        at: "2026-10-10T15:45:00+08:00",
        short: "AI 存疑",
      };
    }

    const blankSource = { ...link, resolved: false };
    const cweByRule: Record<string, string[]> = {
      "py/sql-injection": ["CWE-89"],
      "py/command-line-injection": ["CWE-78", "CWE-88"],
      "py/path-injection": ["CWE-22"],
      "py/stack-trace-exposure": ["CWE-209", "CWE-497"],
      "py/reflective-xss": ["CWE-79"],
      "py/weak-crypto": ["CWE-327"],
    };
    const cwes = cweByRule[rule.name] ?? ["CWE-20"];
    const flowSteps: FlowStepDTO[] = [
      {
        index: 1,
        location: source,
        message: "user-provided value",
        nestingLevel: 0,
        kinds: [],
      },
      {
        index: 2,
        location: makeLocation(file, Math.max(1, line - 1), 5, "ControlFlowNode for value", "值的控制流节点"),
        message: "ControlFlowNode for value",
        nestingLevel: 0,
        kinds: ["intermediate"],
      },
      {
        index: 3,
        location: sink,
        message: "sink",
        nestingLevel: 0,
        kinds: ["sink"],
      },
    ];
    const primaryFingerprint = `primaryLocationLineHash=mock${i + 1}:1|primaryLocationStartColumnFingerprint=${col}`;
    const codeFlows: CodeFlowDTO[] =
      i % 4 === 0
        ? [{ index: 1, message: "", steps: flowSteps }]
        : [];
    const relatedLocations: RelatedLocationDTO[] = [
      { id: 1, location: source, message: "user-provided value" },
    ];
    const ruleInfo: RuleInfoDTO = {
      id: rule.name,
      name: rule.name,
      shortDescription: rule.description,
      fullDescription: rule.descriptionZh,
      helpUri: `https://codeql.github.com/codeql-query-help/python/${rule.name.replace("/", "-")}/`,
      defaultLevel: rule.severity,
      securitySeverity: "8.8",
      precision: "high",
      kind: "path-problem",
      problemSeverity: rule.severity,
      tags: ["security", `external/cwe/${cwes[0].toLowerCase()}`],
      cwes,
    };
    list.push({
      key: `mock-finding-${i + 1}`,
      index: i + 1,
      ruleName: rule.name,
      ruleNameZh: rule.zh,
      ruleDisplay: rule.zh,
      description: rule.description,
      descriptionZh: rule.descriptionZh,
      descriptionDisplay: rule.descriptionZh,
      severity: rule.severity,
      severityZh:
        rule.severity === "error"
          ? "严重"
          : rule.severity === "warning"
            ? "警告"
            : rule.severity === "recommendation"
              ? "建议"
              : "提示",
      sink,
      sources: [source, { ...source, file: sourceFile, label: "control flow", labelZh: "控制流" }],
      messageEn,
      messageZh: i === 17 || i === 34 ? [] : messageZh,
      messageRaw: `${rule.messageEn}\n\n[[user-provided value|relative:///${sourceFile}:${sourceLine}:${sourceCol}:${sourceLine}:${sourceCol + 6}]]`,
      messageKey: `${rule.name}: ${rule.messageEn}`,
      messageZhTemplate: rule.messageZh,
      translateSource: i === 17 || i === 34 ? "none" : "builtin",
      translateSourceZh: i === 17 || i === 34 ? "未翻译（保持英文）" : "内置词典",
      translatePending: i === 17 || i === 34,
      reviewed: i !== 0 && i % 7 === 0,
      ai,
      format: "sarif",
      formatLabel: "SARIF v2.1.0",
      toolName: "CodeQL",
      toolVersion: "2.26.3",
      cwes,
      tags: ruleInfo.tags,
      fingerprints: [
        { name: "primaryLocationLineHash", value: `mock${i + 1}:1` },
        { name: "primaryLocationStartColumnFingerprint", value: String(col) },
      ],
      primaryFingerprint,
      ruleInfo,
      codeFlows,
      relatedLocations,
    });
    void blankSource;
  }
  return list;
}

function buildProject(): ProjectDTO {
  const findings = buildFindings();
  const stats = statsOf(findings);
  const ruleCounts = new Map<string, { label: string; count: number }>();
  const fileCounts = new Map<string, number>();
  const unresolvedMap = new Map<string, number>();
  for (const f of findings) {
    const r = ruleCounts.get(f.ruleName);
    ruleCounts.set(f.ruleName, { label: f.ruleDisplay, count: (r?.count || 0) + 1 });
    fileCounts.set(f.sink.file, (fileCounts.get(f.sink.file) || 0) + 1);
    if (!f.sink.resolved) unresolvedMap.set(f.sink.file, (unresolvedMap.get(f.sink.file) || 0) + 1);
  }
  return {
    csvPath: CSV_PATH,
    csvName: "pyhackme.sarif",
    format: "sarif",
    formatLabel: "SARIF v2.1.0",
    toolName: "CodeQL",
    toolVersion: "2.26.3",
    srcRoot: ROOT,
    findings,
    stats,
    rules: [...ruleCounts.entries()]
      .sort((a, b) => b[1].count - a[1].count)
      .map(([value, info]) => ({ value, label: info.label, count: info.count })),
    files: [...fileCounts.entries()]
      .sort((a, b) => b[1] - a[1])
      .map(([value, count]) => ({ value, label: value, count })),
    severities: [
      { value: "error", label: "严重", count: stats.bySeverity.error || 0 },
      { value: "warning", label: "警告", count: stats.bySeverity.warning || 0 },
      { value: "note", label: "提示", count: stats.bySeverity.note || 0 },
      { value: "recommendation", label: "建议", count: stats.bySeverity.recommendation || 0 },
    ].filter((s) => s.count > 0),
    warnings: [],
    unresolved: [...unresolvedMap.entries()].map(([file, count]) => ({
      file,
      count,
      severities: ["error"],
    })),
    prefixRules: [],
    overrides: {},
    loadedAt: new Date().toISOString(),
    translationDb: "/Users/shiyan/.CodeQLAuditPro/translations.db",
  };
}

let project = buildProject();
let settings: SettingsDTO = {
  llm: {
    enabled: true,
    baseUrl: "https://api.deepseek.com/v1",
    apiKey: "",
    model: "deepseek-chat",
    timeoutSeconds: 60,
    templateOnly: true,
  },
  aiVerifyEnabled: true,
  aiSendCode: true,
  editor: {
    cmd: "/Applications/Visual Studio Code.app/Contents/Resources/app/bin/code",
    args: "-g {file}:{line}:{col}",
  },
  displayLang: "zh",
  showColumnRuler: true,
  codeSearchShortcut: "Cmd+F",
  translateDb: "/Users/shiyan/.CodeQLAuditPro/translations.db",
  historyDb: "/Users/shiyan/.CodeQLAuditPro/history.db",
  dataDir: "/Users/shiyan/.CodeQLAuditPro",
  configPath: "/Users/shiyan/.CodeQLAuditPro/config.json",
  windowWidth: 1480,
  windowHeight: 930,
  splitLeft: 0.24,
  splitDetail: 0.5,
  topInfoCollapsed: false,
};

let batchCancelled = false;
// mock 的批量验证状态：让「关掉窗口再打开」也能读回进度（与真实后端语义一致）。
let batchState: BatchStateDTO = {
  running: false,
  stopping: false,
  done: 0,
  total: 0,
  falsePositive: 0,
  truePositive: 0,
  uncertain: 0,
  failed: 0,
};

function recompute() {
  project.stats = statsOf(project.findings);
}

function find(key: string): FindingDTO | undefined {
  return project.findings.find((f) => f.key === key);
}

function delay(ms: number) {
  return new Promise((r) => setTimeout(r, ms));
}

function mappingOf(): MappingDTO {
  return {
    unresolved: project.unresolved,
    prefixRules: project.prefixRules,
    overrides: project.overrides,
    totalFiles: project.stats.fileKinds,
    totalFindings: project.stats.total,
    resolved: project.stats.resolved,
    stats: project.stats,
  };
}

const LIBRARY = [
  {
    id: 1,
    kind: "rule",
    kindZh: "规则名称",
    sourceEn: "py/sql-injection",
    sourceZh: "SQL 注入",
    origin: "builtin",
    originZh: "内置词典",
    hits: 47,
    updatedAt: "2026-10-10T10:00:00Z",
  },
  {
    id: 2,
    kind: "rule",
    kindZh: "规则名称",
    sourceEn: "py/command-line-injection",
    sourceZh: "命令注入",
    origin: "builtin",
    originZh: "内置词典",
    hits: 32,
    updatedAt: "2026-10-10T10:00:00Z",
  },
  {
    id: 3,
    kind: "description",
    kindZh: "规则描述",
    sourceEn: "SQL query built from user-controlled sources",
    sourceZh: "把用户可控的数据拼接进 SQL 查询，可能导致 SQL 注入。",
    origin: "builtin",
    originZh: "内置词典",
    hits: 44,
    updatedAt: "2026-10-10T10:00:00Z",
  },
  {
    id: 4,
    kind: "message",
    kindZh: "漏洞详情",
    sourceEn: "This query depends on a user-provided value.",
    sourceZh: "这条查询依赖来自{{LINK}}的数据，未经参数化处理。",
    origin: "builtin",
    originZh: "内置词典",
    hits: 31,
    updatedAt: "2026-10-10T10:00:00Z",
  },
  {
    id: 5,
    kind: "label",
    kindZh: "证据链标签",
    sourceEn: "user-provided value",
    sourceZh: "用户可控输入",
    origin: "builtin",
    originZh: "内置词典",
    hits: 120,
    updatedAt: "2026-10-10T10:00:00Z",
  },
  {
    id: 6,
    kind: "label",
    kindZh: "证据链标签",
    sourceEn: "control flow",
    sourceZh: "控制流",
    origin: "builtin",
    originZh: "内置词典",
    hits: 88,
    updatedAt: "2026-10-10T10:00:00Z",
  },
];

function projectFileFor(csvPath: string): string {
  const rel = csvPath.replace(/^\/+/, "");
  return abs(rel);
}

// ------------------------------ CodeQL 助手 mock ------------------------------

const assistantConfig: AssistantConfigDTO = {
  name: "",
  binaryPath: "/opt/homebrew/bin/codeql",
  queryPath: "codeql/python-queries:codeql-suites/python-security-extended.qls",
  databasePath: "/Users/shiyan/codeql-db/pyhackme",
  sourceRoot: ROOT,
  language: "python",
  outputFormat: "sarifv2.1.0",
  outputFile: "/Users/shiyan/Downloads/codeql-results.sarif",
  ramLimit: 4096,
  threads: 4,
  extraArgs: "",
};

const assistantProfiles = ["python-security", "java-maven"];
const assistantHistory: AssistantHistoryDTO[] = [
  {
    timestamp: new Date(Date.now() - 3600_000).toISOString(),
    action: "analyze",
    status: "已完成",
    outputFile: "/Users/shiyan/Downloads/codeql-results.csv",
    durationMs: 92_400,
    language: "python",
    databasePath: "/Users/shiyan/codeql-db/pyhackme",
    queryPath: "codeql/python-queries:codeql-suites/python-security-extended.qls",
  },
];

let mockAssistantRunning = false;
let mockAssistantAction = "";
let mockAssistantStatus = "idle";
let mockAssistantStatusLabel = "就绪";
let mockAssistantStartedAt = "";
let mockAssistantElapsed = 0;
let mockAssistantLogPath = "";
let mockAssistantTimer: number | undefined;

function mockAssistantStatusDTO(): AssistantStatusDTO {
  return {
    running: mockAssistantRunning,
    status: mockAssistantStatus,
    statusLabel: mockAssistantStatusLabel,
    action: mockAssistantAction,
    startedAt: mockAssistantStartedAt,
    elapsedMs: mockAssistantElapsed,
    logPath: mockAssistantLogPath,
  };
}

function mockAssistantState(): AssistantStateDTO {
  return {
    config: { ...assistantConfig },
    defaultConfig: { ...assistantConfig, binaryPath: "", databasePath: "", sourceRoot: "" },
    profiles: [...assistantProfiles],
    history: [...assistantHistory],
    running: mockAssistantRunning,
    status: mockAssistantStatus,
    statusLabel: mockAssistantStatusLabel,
    action: mockAssistantAction,
    startedAt: mockAssistantStartedAt,
    elapsedMs: mockAssistantElapsed,
    logPath: mockAssistantLogPath,
    quote: "漏洞不会自己说话，但查询可以替它开口。",
    languages: ["java", "python", "javascript", "typescript", "cpp", "csharp", "go", "ruby", "swift", "kotlin"],
    formats: ["sarifv2.1.0", "csv"],
    dataDir: "/Users/shiyan/.CodeQLAuditPro/assistant",
    platform: "darwin",
  };
}

function splitMockArgs(input: string): string[] {
  const args: string[] = [];
  let current = "";
  let inQuote = false;
  for (const ch of input) {
    if (ch === '"') {
      inQuote = !inQuote;
      continue;
    }
    if (!inQuote && /\s/.test(ch)) {
      if (current) {
        args.push(current);
        current = "";
      }
      continue;
    }
    current += ch;
  }
  if (current) args.push(current);
  return args;
}

function mockFormat(format: string): string {
  return (format || "").toLowerCase() === "csv" ? "csv" : "sarifv2.1.0";
}

function mockCommand(binary: string, args: string[]): string {
  return [binary, ...args]
    .map((arg) => (arg === "" ? "''" : /\s/.test(arg) ? `"${arg}"` : arg))
    .join(" ");
}

function mockAssistantPreview(cfg: AssistantConfigDTO): AssistantCommandsDTO {
  const binary = cfg.binaryPath || "codeql";
  const extra = splitMockArgs(cfg.extraArgs || "");

  const createArgs = [
    "database",
    "create",
    cfg.databasePath || "<数据库目录>",
    `--language=${cfg.language || "java"}`,
    `--source-root=${cfg.sourceRoot || "<源码根目录>"}`,
  ];
  if (cfg.threads > 0) createArgs.push(`--threads=${cfg.threads}`);
  if (cfg.ramLimit > 0) createArgs.push(`--ram=${cfg.ramLimit}`);
  createArgs.push("--overwrite", ...extra);

  const analyzeArgs = [
    "database",
    "analyze",
    cfg.databasePath || "<数据库目录>",
    cfg.queryPath || "<查询 / 套件>",
    `--format=${mockFormat(cfg.outputFormat)}`,
    `--output=${cfg.outputFile || "<输出文件>"}`,
  ];
  if (cfg.threads > 0) analyzeArgs.push(`--threads=${cfg.threads}`);
  if (cfg.ramLimit > 0) analyzeArgs.push(`--ram=${cfg.ramLimit}`);
  analyzeArgs.push(...extra);

  return {
    create: mockCommand(binary, createArgs),
    analyze: mockCommand(binary, analyzeArgs),
  };
}

function mockAssistantRun(action: "create" | "analyze") {
  if (mockAssistantRunning) throw new Error("已有任务正在运行，请先停止当前任务");
  mockAssistantRunning = true;
  mockAssistantAction = action;
  mockAssistantStatus = "running";
  mockAssistantStatusLabel = "运行中…";
  mockAssistantStartedAt = new Date().toISOString();
  mockAssistantElapsed = 0;
  mockAssistantLogPath = "/Users/shiyan/.CodeQLAuditPro/assistant/logs/20261019-120000.log";
  emitLocal("assistant:output", { reset: true, lines: [] });
  emitLocal("assistant:status", mockAssistantStatusDTO());

  const preview = mockAssistantPreview(assistantConfig);
  const script =
    action === "create"
      ? [
          `$ ${preview.create}`,
          "---",
          "Initializing database at /Users/shiyan/codeql-db/pyhackme.",
          "Running command: python",
          "Running query: python",
          "Database creation complete.",
          "---",
          "数据库创建成功完成。",
        ]
      : [
          `$ ${preview.analyze}`,
          "---",
          "Running queries.",
          " [1/4] py/sql-injection",
          " [2/4] py/command-line-injection",
          " [3/4] py/path-injection",
          " [4/4] py/reflected-xss",
          "Wrote results to /Users/shiyan/Downloads/codeql-results.csv",
          "---",
          "分析成功完成。",
        ];

  let index = 0;
  mockAssistantTimer = window.setInterval(() => {
    if (index >= script.length) {
      window.clearInterval(mockAssistantTimer);
      mockAssistantRunning = false;
      mockAssistantStatus = "completed";
      mockAssistantStatusLabel = "已完成";
      mockAssistantElapsed = action === "create" ? 18_400 : 92_400;
      assistantHistory.unshift({
        timestamp: new Date().toISOString(),
        action,
        status: "已完成",
        outputFile: action === "create" ? assistantConfig.databasePath : assistantConfig.outputFile,
        durationMs: mockAssistantElapsed,
        language: assistantConfig.language,
        databasePath: assistantConfig.databasePath,
        queryPath: assistantConfig.queryPath,
      });
      emitLocal("assistant:status", mockAssistantStatusDTO());
      emitLocal("assistant:done", {
        action,
        status: "completed",
        statusLabel: "已完成",
        outputFile: action === "create" ? assistantConfig.databasePath : assistantConfig.outputFile,
        durationMs: mockAssistantElapsed,
        error: "",
      });
      return;
    }
    emitLocal("assistant:output", {
      reset: false,
      lines: [{ text: script[index], error: script[index].includes("失败") }],
    });
    index += 1;
    mockAssistantElapsed += 900;
  }, 220);
  return Promise.resolve();
}

let mockProjectHistory: ProjectHistoryDTO[] = [
  {
    csvPath: CSV_PATH,
    csvName: "pyhackme.sarif",
    srcRoot: ROOT,
    lastOpened: new Date().toISOString(),
    totalFindings: 47,
    resolvedFindings: 44,
    reviewedCount: 12,
    verdictCount: 9,
    format: "sarif",
    formatLabel: "SARIF v2.1.0",
    toolName: "CodeQL",
    resultExists: true,
    resultSize: 12840000,
    sourceExists: true,
    active: true,
  },
  {
    csvPath: "/Users/shiyan/Downloads/pyhackme.csv",
    csvName: "pyhackme.csv",
    srcRoot: "/Users/shiyan/Downloads/待做笔记的参考材料/pyhackme",
    lastOpened: "2026-10-19T18:37:31+08:00",
    totalFindings: 31,
    resolvedFindings: 31,
    reviewedCount: 31,
    verdictCount: 4,
    format: "csv",
    formatLabel: "CSV",
    toolName: "CodeQL",
    resultExists: true,
    resultSize: 86000,
    sourceExists: true,
    active: false,
  },
];

export const mockApi: AppApi = {
  async Bootstrap(): Promise<BootstrapDTO> {
    return {
      settings,
      lastProject: CSV_PATH,
      lastSrcRoot: ROOT,
      autoLoad: false,
      platform: "darwin",
      version: "1.2.0",
      defaultSearchShortcut: "Cmd+F",
      defaultEditorArgs: "-g {file}:{line}:{col}",
      dataDir: settings.dataDir,
      configPath: settings.configPath,
    };
  },
  async GetSettings() {
    return settings;
  },
  async SaveSettings(s: SettingsDTO) {
    settings = { ...s };
    return settings;
  },
  async SaveUIPreferences() {},
  async SetNativeTheme() {},
  async ChooseResultsFile() {
    return CSV_PATH;
  },
  async ChooseCSVFile() {
    return CSV_PATH;
  },
  async ChooseDirectory() {
    return ROOT;
  },
  async LoadProject() {
    emitLocal("project:progress", { phase: "parse", done: 0, total: 1, message: "正在解析 SARIF v2.1.0 …" });
    await delay(220);
    emitLocal("project:progress", {
      phase: "load",
      done: 12,
      total: 47,
      message: "正在翻译与定位 12/47 …",
    });
    await delay(260);
    project = buildProject();
    emitLocal("project:progress", {
      phase: "load",
      done: 47,
      total: 47,
      message: "正在翻译与定位 47/47 …",
    });
    await delay(180);
    emitLocal("project:progress", { phase: "done", done: 1, total: 1, message: "载入完成" });
    return structuredClone(project);
  },
  async GetProject() {
    return structuredClone(project);
  },
  async CloseProject() {
    mockProjectHistory = mockProjectHistory.map((h) => ({ ...h, active: false }));
  },
  async GetProjectHistory() {
    return structuredClone(mockProjectHistory);
  },
  async DeleteProjectHistory(csvPath: string) {
    mockProjectHistory = mockProjectHistory.filter((h) => h.csvPath !== csvPath);
    return structuredClone(mockProjectHistory);
  },
  async ClearProjectHistory() {
    mockProjectHistory = [];
    return [];
  },
  async TranslateProject(): Promise<TranslateProjectDTO> {
    const targets = project.findings.filter(
      (f) =>
        f.translatePending ||
        !f.ruleNameZh ||
        f.ruleNameZh === f.ruleName ||
        !f.messageZhTemplate,
    );
    emitLocal("translate:progress", {
      done: 0,
      total: targets.length,
      message: `正在补全翻译 0/${targets.length} …`,
    });
    await delay(320);
    for (const f of targets) {
      f.translatePending = false;
      f.translateSource = "llm";
      f.translateSourceZh = "大模型";
      if (!f.ruleNameZh || f.ruleNameZh === f.ruleName) {
        f.ruleNameZh = f.ruleDisplay;
      }
      if (!f.messageZhTemplate) {
        f.messageZhTemplate = f.messageRaw || f.ruleDisplay;
      }
    }
    recompute();
    emitLocal("translate:progress", {
      done: targets.length,
      total: targets.length,
      message: `正在补全翻译 ${targets.length}/${targets.length} …`,
    });
    return {
      project: structuredClone(project),
      total: targets.length,
      translated: targets.length,
      remaining: 0,
      message: `翻译完成：本次补全 ${targets.length} 条，当前项目已全部中文化。`,
    };
  },
  async SetSrcRoot() {
    return structuredClone(project);
  },
  async GetSourceFile(csvPath: string, resolvedPath: string): Promise<FileDTO> {
    const path = resolvedPath || projectFileFor(csvPath);
    const rel = path.startsWith(ROOT + "/") ? path.slice(ROOT.length + 1) : csvPath;
    const lines = SOURCE_FILES[rel];
    if (!lines) throw new Error(`未定位：${csvPath}\n可在「路径映射」中手工指定本地文件`);
    return {
      path,
      displayPath: csvPath,
      encoding: "UTF-8",
      lines,
      totalLines: lines.length,
    };
  },
  async ResolvePath(csvPath: string) {
    const rel = csvPath.replace(/^\/+/, "");
    if (!SOURCE_FILES[rel]) throw new Error(`未定位：${csvPath}`);
    return abs(rel);
  },
  async SetReviewed(keys: string[], reviewed: boolean) {
    const set = new Set(keys);
    for (const f of project.findings) {
      if (set.has(f.key)) f.reviewed = reviewed;
    }
    recompute();
    return project.stats;
  },
  async ResetReviewed() {
    for (const f of project.findings) f.reviewed = false;
    recompute();
    return project.stats;
  },
  async GetMapping() {
    return mappingOf();
  },
  async AddPrefixRule(from: string, to: string) {
    project.prefixRules.push({ from, to });
    return mappingOf();
  },
  async RemovePrefixRule(index: number) {
    project.prefixRules.splice(index, 1);
    return mappingOf();
  },
  async ChooseOverrideFile(csvPath: string) {
    const unresolved = project.findings.filter((f) => !f.sink.resolved);
    for (const f of unresolved) {
      f.sink.resolved = true;
      f.sink.resolvedPath = abs("app/common/sql.py");
    }
    project.unresolved = [];
    project.overrides[csvPath] = abs("app/common/sql.py");
    recompute();
    return mappingOf();
  },
  async RemoveOverride(csvPath: string) {
    delete project.overrides[csvPath];
    return mappingOf();
  },
  async OpenInEditor(path: string, line: number) {
    return `code -g ${path}:${line}:1（模拟）`;
  },
  async RevealInFinder() {},
  async CopyText(text: string) {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      /* ignore */
    }
  },
  async OpenExternalURL(url: string) {
    window.open(url, "_blank");
  },
  async VerifyFinding(key: string) {
    await delay(1200);
    const f = find(key);
    if (!f) throw new Error("找不到这条漏洞");
    const ai: AIDTO = {
      verdict: "false_positive",
      verdictZh: "误报",
      reason: "沉淀点使用了参数化查询或已有的转义函数，用户输入无法改变语句结构，判定为误报。",
      confidence: 0.88,
      model: settings.llm.model || "deepseek-chat",
      at: new Date().toISOString(),
      short: "AI 误报",
    };
    f.ai = ai;
    recompute();
    return { findingKey: key, ai, stats: project.stats };
  },
  async ClearVerdict(key: string) {
    const f = find(key);
    if (f) f.ai = null;
    recompute();
    return project.stats;
  },
  async StartBatchVerify(keys: string[]) {
    batchCancelled = false;
    batchState = {
      running: true,
      stopping: false,
      done: 0,
      total: keys.length,
      falsePositive: 0,
      truePositive: 0,
      uncertain: 0,
      failed: 0,
    };
    const targets = keys.length ? project.findings.filter((f) => keys.includes(f.key)) : project.findings;
    const total = targets.length;
    void (async () => {
      let done = 0;
      let fp = 0;
      let tp = 0;
      let un = 0;
      let failed = 0;
      for (const f of targets) {
        if (batchCancelled) break;
        await delay(320);
        const verdict = done % 5 === 0 ? "false_positive" : done % 7 === 0 ? "true_positive" : "uncertain";
        const ai: AIDTO = {
          verdict,
          verdictZh: verdict === "false_positive" ? "误报" : verdict === "true_positive" ? "真实漏洞" : "存疑",
          reason:
            verdict === "false_positive"
              ? "输入经过参数化处理，未改变查询结构。"
              : verdict === "true_positive"
                ? "输入未经净化直接进入危险函数。"
                : "上下文不足，建议人工复核。",
          confidence: 0.72 + (done % 3) * 0.08,
          model: settings.llm.model || "deepseek-chat",
          at: new Date().toISOString(),
          short: verdict === "false_positive" ? "AI 误报" : verdict === "true_positive" ? "AI 确认" : "AI 存疑",
        };
        f.ai = ai;
        done++;
        if (verdict === "false_positive") fp++;
        else if (verdict === "true_positive") tp++;
        else un++;
        recompute();
        const progress: BatchProgressDTO = {
          done,
          total,
          falsePositive: fp,
          truePositive: tp,
          uncertain: un,
          failed,
          findingKey: f.key,
          ai,
          stopping: batchCancelled,
        };
        batchState = {
          running: true,
          stopping: batchCancelled,
          done,
          total,
          falsePositive: fp,
          truePositive: tp,
          uncertain: un,
          failed,
        };
        emitLocal("batch:progress", progress);
      }
      const summary: BatchSummaryDTO = {
        done,
        total,
        falsePositive: fp,
        truePositive: tp,
        uncertain: un,
        failed,
        stopped: batchCancelled,
        message: batchCancelled
          ? `已停止：共处理 ${done} / ${total} 条。`
          : `完成：共 ${done} 条，误报 ${fp}，确认 ${tp}，存疑 ${un}。`,
      };
      batchState = {
        ...batchState,
        running: false,
        stopping: false,
        done,
        total,
        falsePositive: fp,
        truePositive: tp,
        uncertain: un,
        failed,
      };
      emitLocal("batch:done", summary);
    })();
    return { total };
  },
  async CancelBatchVerify() {
    batchCancelled = true;
    batchState = { ...batchState, stopping: true };
    emitLocal("batch:stopping", { done: batchState.done, total: batchState.total });
  },
  async BatchState() {
    return { ...batchState };
  },
  async TestConnection(s) {
    await delay(700);
    return {
      endpoint: `${s.baseUrl.replace(/\/$/, "")}/chat/completions`,
      model: s.model,
      reply: "OK",
    };
  },
  async GetLibrary(kind: string, query: string): Promise<LibraryDTO> {
    const q = query.toLowerCase();
    const entries = LIBRARY.filter(
      (e) =>
        (!kind || e.kind === kind) &&
        (!q || e.sourceEn.toLowerCase().includes(q) || e.sourceZh.toLowerCase().includes(q)),
    );
    return { total: LIBRARY.length, shown: entries.length, entries: structuredClone(entries) };
  },
  async SaveLibraryEntry(kind: string, en: string, zh: string) {
    const entry = LIBRARY.find((e) => e.kind === kind && e.sourceEn === en);
    if (entry) {
      entry.sourceZh = zh;
      entry.origin = "user";
      entry.originZh = "人工校正";
    }
  },
  async CorrectMessage(key: string, template: string) {
    const f = find(key);
    if (!f) throw new Error("找不到这条漏洞");
    f.messageZhTemplate = template;
    const link = f.sources[0];
    f.messageZh = [
      { kind: "text", text: template.replace(/\{\{LINK\}\}/g, "").slice(0, 60) },
      { kind: "link", text: "用户可控输入", link: linkForFile(link.file, link.startLine, link.startCol) },
    ];
    f.translatePending = false;
    f.translateSource = "store";
    f.translateSourceZh = "翻译库";
    recompute();
    return { finding: structuredClone(f), stats: structuredClone(project.stats) };
  },
  async ExportChineseCSV() {
    return { path: "/Users/shiyan/Downloads/results_中文.csv", count: project.findings.length, kind: "中文 CSV" };
  },
  async ExportAuditReport() {
    return { path: "/Users/shiyan/Downloads/results_审计报告.csv", count: project.findings.length, kind: "审计报告" };
  },
  async AssistantBootstrap(): Promise<AssistantStateDTO> {
    return mockAssistantState();
  },
  async AssistantGetState(): Promise<AssistantStateDTO> {
    return mockAssistantState();
  },
  async AssistantSaveConfig(cfg: AssistantConfigDTO): Promise<void> {
    Object.assign(assistantConfig, cfg);
  },
  async AssistantSaveProfile(name: string, cfg: AssistantConfigDTO): Promise<void> {
    if (!assistantProfiles.includes(name)) assistantProfiles.push(name);
    Object.assign(assistantConfig, cfg, { name });
  },
  async AssistantLoadProfile(name: string): Promise<AssistantConfigDTO> {
    if (!assistantProfiles.includes(name)) throw new Error("配置不存在");
    return { ...assistantConfig, name };
  },
  async AssistantDeleteProfile(name: string): Promise<void> {
    const idx = assistantProfiles.indexOf(name);
    if (idx >= 0) assistantProfiles.splice(idx, 1);
  },
  async AssistantListProfiles(): Promise<string[]> {
    return [...assistantProfiles];
  },
  async AssistantGetHistory(): Promise<AssistantHistoryDTO[]> {
    return [...assistantHistory];
  },
  async AssistantClearHistory(): Promise<void> {
    assistantHistory.length = 0;
  },
  async AssistantRandomQuote(): Promise<string> {
    return "你查的不是代码，是很多人睡不着的理由。";
  },
  async AssistantRunCreate(cfg: AssistantConfigDTO): Promise<void> {
    Object.assign(assistantConfig, cfg);
    return mockAssistantRun("create");
  },
  async AssistantRunAnalyze(cfg: AssistantConfigDTO): Promise<void> {
    Object.assign(assistantConfig, cfg);
    return mockAssistantRun("analyze");
  },
  async AssistantStop(): Promise<void> {
    if (mockAssistantTimer) window.clearInterval(mockAssistantTimer);
    mockAssistantRunning = false;
    mockAssistantStatus = "stopped";
    mockAssistantStatusLabel = "已停止";
    emitLocal("assistant:status", mockAssistantStatusDTO());
    emitLocal("assistant:done", {
      action: mockAssistantAction,
      status: "stopped",
      statusLabel: "已停止",
      outputFile: assistantConfig.outputFile,
      durationMs: mockAssistantElapsed,
      error: "",
    });
  },
  async AssistantPreviewCommands(cfg: AssistantConfigDTO): Promise<AssistantCommandsDTO> {
    return mockAssistantPreview(cfg);
  },
  async AssistantChooseBinary(): Promise<string> {
    return "/opt/homebrew/bin/codeql";
  },
  async AssistantChooseQueryFile(): Promise<string> {
    return "/Users/shiyan/queries/security.qls";
  },
  async AssistantChooseQueryDir(): Promise<string> {
    return "/Users/shiyan/queries";
  },
  async AssistantChooseDirectory(_title: string, startDir: string): Promise<string> {
    return startDir || "/Users/shiyan/codeql-db";
  },
  async AssistantChooseOutputFile(defaultName: string): Promise<string> {
    return `/Users/shiyan/Downloads/${defaultName}`;
  },
  async AssistantOpenLog(): Promise<void> {},
  async AssistantOpenPath(_path: string): Promise<void> {},
  async AssistantOpenDataDir(): Promise<void> {},
  async ExportVerdicts() {
    const count = project.findings.filter((f) => f.ai).length;
    return { path: "/Users/shiyan/Downloads/results_AI验证结果.csv", count, kind: "AI 验证结果" };
  },
};
