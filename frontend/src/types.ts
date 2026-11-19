export type Severity = "error" | "warning" | "note" | "recommendation" | string;
export type Lang = "zh" | "en" | "both";
export type Theme = "dark" | "light";
export type Verdict = "false_positive" | "true_positive" | "uncertain" | "";

export interface LocationDTO {
  file: string;
  resolvedPath: string;
  startLine: number;
  startCol: number;
  endLine: number;
  endCol: number;
  label: string;
  labelZh: string;
  resolved: boolean;
  lineRange: string;
  columnRange: string;
  lineCol: string;
  displayPath: string;
  baseName: string;
}

export interface LinkDTO {
  label: string;
  labelZh: string;
  rawUri: string;
  file: string;
  startLine: number;
  startCol: number;
  endLine: number;
  endCol: number;
}

export interface SegmentDTO {
  kind: "text" | "link";
  text: string;
  link?: LinkDTO;
}

export interface AIDTO {
  verdict: Verdict;
  verdictZh: string;
  reason: string;
  confidence: number;
  model: string;
  at: string;
  short: string;
}

export interface FingerprintDTO {
  name: string;
  value: string;
}

export interface RuleInfoDTO {
  id: string;
  name: string;
  shortDescription: string;
  fullDescription: string;
  helpUri: string;
  defaultLevel: string;
  securitySeverity: string;
  precision: string;
  kind: string;
  problemSeverity: string;
  tags: string[];
  cwes: string[];
}

export interface FlowStepDTO {
  index: number;
  location: LocationDTO;
  message: string;
  nestingLevel: number;
  kinds: string[];
}

export interface CodeFlowDTO {
  index: number;
  message: string;
  steps: FlowStepDTO[];
}

export interface RelatedLocationDTO {
  id: number;
  location: LocationDTO;
  message: string;
}

export interface FindingDTO {
  key: string;
  index: number;
  ruleName: string;
  ruleNameZh: string;
  ruleDisplay: string;
  description: string;
  descriptionZh: string;
  descriptionDisplay: string;
  severity: Severity;
  severityZh: string;
  sink: LocationDTO;
  sources: LocationDTO[];
  messageEn: SegmentDTO[];
  messageZh: SegmentDTO[];
  messageRaw: string;
  messageKey: string;
  messageZhTemplate: string;
  translateSource: string;
  translateSourceZh: string;
  translatePending: boolean;
  reviewed: boolean;
  ai: AIDTO | null;
  format: string;
  formatLabel: string;
  toolName: string;
  toolVersion: string;
  cwes: string[];
  tags: string[];
  fingerprints: FingerprintDTO[];
  primaryFingerprint: string;
  ruleInfo: RuleInfoDTO | null;
  codeFlows: CodeFlowDTO[];
  relatedLocations: RelatedLocationDTO[];
}

export interface StatsDTO {
  total: number;
  ruleKinds: number;
  fileKinds: number;
  resolved: number;
  missing: number;
  unread: number;
  pending: number;
  translationMissing: number;
  verified: number;
  falsePositive: number;
  truePositive: number;
  uncertain: number;
  translationBuiltin: number;
  translationStore: number;
  translationLLM: number;
  bySeverity: Record<string, number>;
  totalSourceLocation: number;
  withCodeFlow: number;
  cweKinds: number;
}

export interface OptionDTO {
  value: string;
  label: string;
  count: number;
}

export interface UnresolvedDTO {
  file: string;
  count: number;
  severities: Severity[];
}

export interface PrefixRuleDTO {
  from: string;
  to: string;
}

export interface ProjectDTO {
  csvPath: string;
  csvName: string;
  format: string;
  formatLabel: string;
  toolName: string;
  toolVersion: string;
  srcRoot: string;
  findings: FindingDTO[];
  stats: StatsDTO;
  rules: OptionDTO[];
  files: OptionDTO[];
  severities: OptionDTO[];
  warnings: string[];
  unresolved: UnresolvedDTO[];
  prefixRules: PrefixRuleDTO[];
  overrides: Record<string, string>;
  loadedAt: string;
  translationDb: string;
}

export interface LLMSettingsDTO {
  enabled: boolean;
  baseUrl: string;
  apiKey: string;
  model: string;
  timeoutSeconds: number;
  templateOnly: boolean;
}

export interface EditorSettingsDTO {
  cmd: string;
  args: string;
}

export interface SettingsDTO {
  llm: LLMSettingsDTO;
  aiVerifyEnabled: boolean;
  aiSendCode: boolean;
  editor: EditorSettingsDTO;
  displayLang: Lang;
  showColumnRuler: boolean;
  codeSearchShortcut: string;
  translateDb: string;
  historyDb: string;
  dataDir: string;
  configPath: string;
  windowWidth: number;
  windowHeight: number;
  splitLeft: number;
  splitDetail: number;
  topInfoCollapsed: boolean;
}

export interface BootstrapDTO {
  settings: SettingsDTO;
  lastProject: string;
  lastSrcRoot: string;
  autoLoad: boolean;
  platform: string;
  version: string;
  defaultSearchShortcut: string;
  defaultEditorArgs: string;
  dataDir: string;
  configPath: string;
  warning?: string;
}

export interface ProjectHistoryDTO {
  csvPath: string;
  csvName: string;
  srcRoot: string;
  lastOpened: string;
  totalFindings: number;
  resolvedFindings: number;
  reviewedCount: number;
  verdictCount: number;
  format: string;
  formatLabel: string;
  toolName: string;
  resultExists: boolean;
  resultSize: number;
  sourceExists: boolean;
  active: boolean;
}

export interface TranslateProjectDTO {
  project: ProjectDTO | null;
  total: number;
  translated: number;
  remaining: number;
  message: string;
  lastError?: string;
}

export interface FileDTO {
  path: string;
  displayPath: string;
  encoding: string;
  lines: string[];
  totalLines: number;
}

export interface VerifyResponseDTO {
  findingKey: string;
  ai: AIDTO | null;
  stats: StatsDTO;
}

export interface BatchProgressDTO {
  done: number;
  total: number;
  falsePositive: number;
  truePositive: number;
  uncertain: number;
  failed: number;
  lastError?: string;
  findingKey?: string;
  ai?: AIDTO | null;
  /** 用户已请求强制停止，剩余条目不会再开始。 */
  stopping?: boolean;
}

export interface BatchSummaryDTO {
  done: number;
  total: number;
  falsePositive: number;
  truePositive: number;
  uncertain: number;
  failed: number;
  stopped: boolean;
  message: string;
}

export interface BatchStartDTO {
  total: number;
}

/** 批量验证的实时状态（窗口关掉再打开也能读回真实进度）。 */
export interface BatchStateDTO {
  running: boolean;
  stopping: boolean;
  done: number;
  total: number;
  falsePositive: number;
  truePositive: number;
  uncertain: number;
  failed: number;
  lastError?: string;
  startedAt?: string;
  elapsedSecond?: number;
}

export interface LibraryEntryDTO {
  id: number;
  kind: string;
  kindZh: string;
  sourceEn: string;
  sourceZh: string;
  origin: string;
  originZh: string;
  hits: number;
  updatedAt: string;
}

export interface LibraryDTO {
  total: number;
  shown: number;
  entries: LibraryEntryDTO[];
}

export interface MappingDTO {
  unresolved: UnresolvedDTO[];
  prefixRules: PrefixRuleDTO[];
  overrides: Record<string, string>;
  totalFiles: number;
  totalFindings: number;
  resolved: number;
  stats: StatsDTO;
}

export interface ExportResultDTO {
  path: string;
  count: number;
  kind: string;
}

export interface TestConnectionDTO {
  endpoint: string;
  model: string;
  reply: string;
}

export interface UIPrefsDTO {
  displayLang: Lang;
  showColumnRuler: boolean;
  splitLeft: number;
  splitDetail: number;
  windowWidth: number;
  windowHeight: number;
  topInfoCollapsed: boolean;
}

export interface AssistantConfigDTO {
  name: string;
  binaryPath: string;
  queryPath: string;
  databasePath: string;
  sourceRoot: string;
  language: string;
  outputFormat: string;
  outputFile: string;
  ramLimit: number;
  threads: number;
  extraArgs: string;
}

export interface AssistantHistoryDTO {
  timestamp: string;
  action: string;
  status: string;
  outputFile: string;
  durationMs: number;
  language: string;
  databasePath: string;
  queryPath: string;
}

export interface AssistantStateDTO {
  config: AssistantConfigDTO;
  defaultConfig: AssistantConfigDTO;
  profiles: string[];
  history: AssistantHistoryDTO[];
  running: boolean;
  status: string;
  statusLabel: string;
  action: string;
  startedAt: string;
  elapsedMs: number;
  logPath: string;
  quote: string;
  languages: string[];
  formats: string[];
  dataDir: string;
  platform: string;
}

export interface AssistantStatusDTO {
  running: boolean;
  status: string;
  statusLabel: string;
  action: string;
  startedAt: string;
  elapsedMs: number;
  logPath: string;
}

export interface AssistantOutputLineDTO {
  text: string;
  error: boolean;
}

export interface AssistantOutputDTO {
  lines: AssistantOutputLineDTO[];
  reset: boolean;
}

export interface AssistantDoneDTO {
  action: string;
  status: string;
  statusLabel: string;
  outputFile: string;
  durationMs: number;
  error: string;
}

export interface AssistantCommandsDTO {
  create: string;
  analyze: string;
}

export interface AppApi {
  Bootstrap(): Promise<BootstrapDTO>;
  GetSettings(): Promise<SettingsDTO>;
  SaveSettings(s: SettingsDTO): Promise<SettingsDTO>;
  SaveUIPreferences(p: UIPrefsDTO): Promise<void>;
  SetNativeTheme(theme: string): Promise<void>;
  ChooseResultsFile(): Promise<string>;
  ChooseCSVFile(): Promise<string>;
  ChooseDirectory(title: string): Promise<string>;
  LoadProject(csvPath: string, srcRoot: string): Promise<ProjectDTO>;
  GetProject(): Promise<ProjectDTO | null>;
  CloseProject(): Promise<void>;
  GetProjectHistory(): Promise<ProjectHistoryDTO[]>;
  DeleteProjectHistory(csvPath: string): Promise<ProjectHistoryDTO[]>;
  ClearProjectHistory(): Promise<ProjectHistoryDTO[]>;
  TranslateProject(): Promise<TranslateProjectDTO>;
  SetSrcRoot(root: string): Promise<ProjectDTO>;
  GetSourceFile(csvPath: string, resolvedPath: string): Promise<FileDTO>;
  ResolvePath(csvPath: string): Promise<string>;
  SetReviewed(keys: string[], reviewed: boolean): Promise<StatsDTO>;
  ResetReviewed(): Promise<StatsDTO>;
  GetMapping(): Promise<MappingDTO>;
  AddPrefixRule(from: string, to: string): Promise<MappingDTO>;
  RemovePrefixRule(index: number): Promise<MappingDTO>;
  ChooseOverrideFile(csvPath: string): Promise<MappingDTO>;
  RemoveOverride(csvPath: string): Promise<MappingDTO>;
  OpenInEditor(path: string, line: number, col: number): Promise<string>;
  RevealInFinder(path: string): Promise<void>;
  CopyText(text: string): Promise<void>;
  OpenExternalURL(url: string): Promise<void>;
  VerifyFinding(key: string): Promise<VerifyResponseDTO>;
  ClearVerdict(key: string): Promise<StatsDTO>;
  StartBatchVerify(keys: string[]): Promise<BatchStartDTO>;
  CancelBatchVerify(): Promise<void>;
  BatchState(): Promise<BatchStateDTO>;
  TestConnection(s: LLMSettingsDTO): Promise<TestConnectionDTO>;
  GetLibrary(kind: string, query: string): Promise<LibraryDTO>;
  SaveLibraryEntry(kind: string, en: string, zh: string): Promise<void>;
  CorrectMessage(key: string, template: string): Promise<{
    finding: FindingDTO;
    stats: StatsDTO;
  }>;
  ExportChineseCSV(keys: string[]): Promise<ExportResultDTO | null>;
  ExportAuditReport(keys: string[]): Promise<ExportResultDTO | null>;
  ExportVerdicts(): Promise<ExportResultDTO | null>;
  AssistantBootstrap(): Promise<AssistantStateDTO>;
  AssistantGetState(): Promise<AssistantStateDTO>;
  AssistantSaveConfig(cfg: AssistantConfigDTO): Promise<void>;
  AssistantSaveProfile(name: string, cfg: AssistantConfigDTO): Promise<void>;
  AssistantLoadProfile(name: string): Promise<AssistantConfigDTO>;
  AssistantDeleteProfile(name: string): Promise<void>;
  AssistantListProfiles(): Promise<string[]>;
  AssistantGetHistory(): Promise<AssistantHistoryDTO[]>;
  AssistantClearHistory(): Promise<void>;
  AssistantRandomQuote(): Promise<string>;
  AssistantRunCreate(cfg: AssistantConfigDTO): Promise<void>;
  AssistantRunAnalyze(cfg: AssistantConfigDTO): Promise<void>;
  AssistantStop(): Promise<void>;
  AssistantPreviewCommands(cfg: AssistantConfigDTO): Promise<AssistantCommandsDTO>;
  AssistantChooseBinary(): Promise<string>;
  AssistantChooseQueryFile(): Promise<string>;
  AssistantChooseQueryDir(): Promise<string>;
  AssistantChooseDirectory(title: string, startDir: string): Promise<string>;
  AssistantChooseOutputFile(defaultName: string): Promise<string>;
  AssistantOpenLog(): Promise<void>;
  AssistantOpenPath(path: string): Promise<void>;
  AssistantOpenDataDir(): Promise<void>;
}

export interface ActiveLocation {
  kind: "sink" | "source" | "link" | "flow";
  index: number;
  loc: LocationDTO;
  /** 当前 codeFlow 的序号（从 1 开始），仅 kind=flow 时有值。 */
  flowIndex?: number;
  /** 当前 codeFlow 内的步骤序号（从 1 开始），仅 kind=flow 时有值。 */
  stepIndex?: number;
}

export interface NavPoint {
  csvPath: string;
  resolvedPath: string;
  line: number;
  active: ActiveLocation;
}
