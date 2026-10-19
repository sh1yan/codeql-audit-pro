import { useMemo } from "react";
import {
  AlertTriangle,
  Anchor,
  ArrowUpRight,
  Bot,
  Braces,
  CheckCircle2,
  ClipboardCopy,
  ExternalLink,
  FileText,
  Fingerprint,
  GitBranch,
  Languages,
  Link2,
  ListTree,
  MapPin,
  PencilLine,
  RefreshCw,
  Settings2,
  ShieldAlert,
  Sparkles,
  Tags,
  Trash2,
  Workflow,
} from "lucide-react";
import type { ActiveLocation, FindingDTO, Lang, LinkDTO, LocationDTO } from "../types";
import { cn, confidenceText, fmtDateTime, severityMeta, verdictMeta } from "../lib/format";
import { EmptyPane, Spinner, Tooltip } from "./ui";

export function DetailPanel({
  finding,
  lang,
  active,
  verifying,
  aiReady,
  onVerify,
  onClearVerdict,
  onCopy,
  onCorrect,
  onJump,
  onJumpFlow,
  onLinkClick,
  onOpenEditor,
  onOpenMapping,
  onOpenSettings,
  onOpenExternal,
}: {
  finding: FindingDTO | null;
  lang: Lang;
  active: ActiveLocation | null;
  verifying: boolean;
  aiReady: boolean;
  onVerify: () => void;
  onClearVerdict: () => void;
  onCopy: () => void;
  onCorrect: () => void;
  onJump: (kind: "sink" | "source", index: number) => void;
  onJumpFlow: (flowIndex: number, stepIndex: number) => void;
  onLinkClick: (link: LinkDTO) => void;
  onOpenEditor: () => void;
  onOpenMapping: () => void;
  onOpenSettings: () => void;
  onOpenExternal: (url: string) => void;
}) {
  const severity = useMemo(() => severityMeta(finding?.severity ?? "note"), [finding?.severity]);
  const verdict = useMemo(() => verdictMeta(finding?.ai?.verdict), [finding?.ai?.verdict]);

  if (!finding) {
    return (
      <aside className="pane pane-right">
        <EmptyPane
          icon={ShieldAlert}
          title="漏洞详情"
          desc="选择一条漏洞查看中文详情、AI 判定与完整证据链。"
        />
      </aside>
    );
  }

  const showZh = lang !== "en";
  const showEn = lang !== "zh";
  const hasZhMessage = (finding.messageZh?.length ?? 0) > 0;
  const sink = finding.sink;

  return (
    <aside className="pane pane-right">
      <div className="detail-scroll">
        <div className="detail-inner">
          {/* 标题与元信息 */}
          <div>
            <div className="flex items-start gap-2">
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-[9.5px] font-mono text-tertiary">#{finding.index}</span>
                  {finding.reviewed ? (
                    <span className="badge bg-surface-3 text-tertiary border-subtle">已读</span>
                  ) : (
                    <span className="badge bg-accent-soft text-accent border-accent">
                      <span className="w-1 h-1 rounded-full bg-current" /> 未读
                    </span>
                  )}
                  <span className={cn("badge", severity.bg, severity.text, severity.border)}>
                    {finding.severityZh}
                  </span>
                  <span className="badge bg-surface-3 text-tertiary border-subtle font-mono">
                    {finding.severity}
                  </span>
                </div>
                <h2 className="detail-title mt-1.5">{finding.ruleDisplay}</h2>
                <div className="detail-sub">{finding.ruleName}</div>
              </div>
            </div>

            <div className="meta-row mt-2">
              {sink.resolved ? (
                <span className="badge bg-ai-fp-soft text-ai-fp border-ai-fp/25">
                  <CheckCircle2 size={9} /> 已定位
                </span>
              ) : (
                <button
                  className="badge bg-sev-warning-soft text-sev-warning border-sev-warning/30 cursor-pointer"
                  onClick={onOpenMapping}
                >
                  <AlertTriangle size={9} /> 未定位
                </button>
              )}
              {finding.translatePending && (
                <span className="badge bg-sev-warning-soft text-sev-warning border-sev-warning/30">
                  <Languages size={9} /> 待翻译
                </span>
              )}
              {finding.translateSourceZh && !finding.translatePending && (
                <span className="badge bg-surface-3 text-tertiary border-subtle">
                  <Languages size={9} /> {finding.translateSourceZh}
                </span>
              )}
              {verdict && finding.ai && (
                <span className={cn("badge", verdict.bg, verdict.text, verdict.border)}>
                  <Sparkles size={9} /> {finding.ai.verdictZh}
                </span>
              )}
              <span
                className={cn(
                  "badge font-mono",
                  finding.format === "sarif"
                    ? "bg-accent-soft text-accent border-accent"
                    : "bg-surface-3 text-tertiary border-subtle",
                )}
                title={finding.toolName ? `${finding.toolName} ${finding.toolVersion}` : undefined}
              >
                {finding.formatLabel || (finding.format === "sarif" ? "SARIF v2.1.0" : "CSV")}
              </span>
              {finding.codeFlows?.length > 0 && (
                <span className="badge bg-surface-3 text-cyan border-subtle">
                  <Workflow size={9} /> 污点路径 {finding.codeFlows.length} 条 /{" "}
                  {finding.codeFlows.reduce((n, cf) => n + cf.steps.length, 0)} 步
                </span>
              )}
            </div>

            {finding.cwes?.length > 0 && (
              <div className="cwe-row mt-1.5">
                {finding.cwes.map((cwe) => (
                  <button
                    key={cwe}
                    className="badge cwe-chip"
                    onClick={() => onOpenExternal(`https://cwe.mitre.org/data/definitions/${cwe.replace(/^CWE-/i, "")}.html`)}
                    title={`在 CWE 官网查看 ${cwe}`}
                  >
                    <Tags size={9} /> {cwe}
                    <ExternalLink size={8} className="opacity-60" />
                  </button>
                ))}
              </div>
            )}

            <div className="mt-2.5 flex flex-col gap-1.5">
              <div className="meta-line">
                <MapPin size={11} className="mt-0.5 text-accent flex-none" />
                <div className="min-w-0">
                  <b className="break-all">{sink.displayPath}</b>
                  <span className="ml-1.5 whitespace-nowrap">
                    第 {sink.lineRange} 行{sink.columnRange ? ` · 列 ${sink.columnRange}` : ""}
                  </span>
                </div>
              </div>
              {sink.resolvedPath && (
                <div className="meta-line">
                  <Link2 size={11} className="mt-0.5 text-tertiary flex-none" />
                  <span className="font-mono text-[10px] break-all text-tertiary">{sink.resolvedPath}</span>
                </div>
              )}
            </div>
          </div>

          {/* AI 误报验证 */}
          <div
            className={cn(
              "ai-card",
              finding.ai && "has-verdict",
              finding.ai?.verdict === "false_positive" && "fp",
              finding.ai?.verdict === "true_positive" && "tp",
              finding.ai?.verdict === "uncertain" && "uncertain",
            )}
          >
            <div className="ai-head">
              <div className="ai-icon">
                <Bot size={13} />
              </div>
              <div className="flex-1 min-w-0">
                <div className="text-[11.5px] font-semibold text-primary">AI 误报验证</div>
                <div className="text-[9.5px] text-tertiary">
                  结合规则语义与源码片段做辅助判定
                </div>
              </div>
              {finding.ai ? (
                <span
                  className={cn(
                    "badge",
                    verdict?.bg,
                    verdict?.text,
                    verdict?.border,
                  )}
                >
                  {finding.ai.verdictZh}
                </span>
              ) : (
                <span className="badge bg-surface-3 text-tertiary border-subtle">尚未验证</span>
              )}
            </div>

            {finding.ai ? (
              <>
                <div className="flex items-center gap-2 text-[10px] text-tertiary flex-wrap">
                  {finding.ai.confidence > 0 && (
                    <span
                      className={cn(
                        "badge",
                        finding.ai.confidence >= 0.75
                          ? "bg-ai-fp-soft text-ai-fp border-ai-fp/25"
                          : "bg-sev-warning-soft text-sev-warning border-sev-warning/25",
                      )}
                    >
                      置信度 {confidenceText(finding.ai.confidence)}
                    </span>
                  )}
                  {finding.ai.model && (
                    <span className="font-mono">{finding.ai.model}</span>
                  )}
                  {finding.ai.at && <span>{fmtDateTime(finding.ai.at)}</span>}
                </div>
                {finding.ai.reason && (
                  <div className="ai-reason">
                    <span className="text-tertiary">理由：</span>
                    {finding.ai.reason}
                  </div>
                )}
                <div className="flex items-center gap-2">
                  <button className="btn btn-sm btn-outline" onClick={onVerify} disabled={verifying}>
                    {verifying ? <Spinner size={11} /> : <RefreshCw size={11} />}
                    重新验证
                  </button>
                  <button className="btn btn-sm text-tertiary" onClick={onClearVerdict}>
                    <Trash2 size={11} /> 清除结果
                  </button>
                  <div className="flex-1" />
                  <span className="text-[9px] text-tertiary">判定仅供参考</span>
                </div>
              </>
            ) : (
              <>
                <div className="text-[10.5px] text-secondary leading-relaxed">
                  让大模型结合规则语义与沉淀点/来源点代码片段，判断这条告警是否为误报。
                  结论会落盘，列表里一眼可见。
                </div>
                <div className="flex items-center gap-2">
                  <button className="btn btn-soft btn-sm" onClick={onVerify} disabled={verifying}>
                    {verifying ? <Spinner size={11} /> : <Sparkles size={11} />}
                    {verifying ? "正在验证…" : "AI 验证本条"}
                  </button>
                  {!aiReady && (
                    <button className="btn btn-sm text-tertiary" onClick={onOpenSettings}>
                      <Settings2 size={11} /> 配置大模型
                    </button>
                  )}
                  <div className="flex-1" />
                  <span className="text-[9px] text-tertiary">最终结论仍需人工确认</span>
                </div>
              </>
            )}
          </div>

          {/* 规则描述 */}
          <Section title="规则描述" icon={FileText}>
            {showZh && (
              <div className="message-text">{finding.descriptionDisplay || "（无描述）"}</div>
            )}
            {showEn && finding.description && (
              <div className={cn("message-text text-secondary", showZh && "mt-2 pt-2 border-t border-subtle")}>
                {finding.description}
              </div>
            )}
          </Section>

          {/* 漏洞详情 */}
          <Section title="漏洞详情" icon={Braces}>
            {finding.translatePending && (
              <div className="mb-2 warning-box" style={{ fontSize: 10.5 }}>
                <AlertTriangle size={13} className="mt-0.5 flex-none" />
                <div className="flex-1">
                  本条还未生成中文译文（翻译库未命中且未配置大模型）。
                  <button className="link-inline ml-1" onClick={onCorrect}>
                    手动纠错
                  </button>
                  或到设置里配置大模型。
                </div>
              </div>
            )}
            {showZh && hasZhMessage && (
              <div className="message-text">
                <Segments
                  finding={finding}
                  segments={finding.messageZh}
                  onLinkClick={onLinkClick}
                />
              </div>
            )}
            {showEn && (
              <div
                className={cn(
                  "message-text",
                  showZh && hasZhMessage && "mt-2 pt-2 border-t border-subtle text-secondary",
                )}
              >
                <Segments finding={finding} segments={finding.messageEn} onLinkClick={onLinkClick} />
              </div>
            )}
            {!showZh && !showEn && <div className="message-text">（语言设置异常）</div>}

            <div className="flex items-center gap-1.5 mt-3 pt-2 border-t border-subtle">
              <button className="btn btn-sm" onClick={onCopy}>
                <ClipboardCopy size={11} /> 复制中文
              </button>
              <button className="btn btn-sm" onClick={onCorrect}>
                <PencilLine size={11} /> 译得不对？纠错
              </button>
              <div className="flex-1" />
              <Tooltip tip="用外部编辑器打开沉淀点">
                <button className="btn btn-icon btn-sm" onClick={onOpenEditor}>
                  <ArrowUpRight size={12} />
                </button>
              </Tooltip>
            </div>
          </Section>

          {/* 完整污点传播路径（SARIF codeFlows） */}
          {finding.codeFlows?.length > 0 && (
            <Section
              title={`污点传播路径 · ${finding.codeFlows.length} 条 / ${finding.codeFlows.reduce(
                (n, cf) => n + cf.steps.length,
                0,
              )} 步`}
              icon={Workflow}
            >
              <div className="flow-list">
                {finding.codeFlows.map((cf) => {
                  const flowActive =
                    active?.kind === "flow" && active.flowIndex === cf.index;
                  return (
                    <details className="flow-group" key={cf.index} open={cf.index === 1}>
                      <summary className="flow-group-head">
                        <span className="flow-badge">路径 {cf.index}</span>
                        <span className="text-[9.5px] text-tertiary">{cf.steps.length} 步</span>
                        {cf.message && (
                          <span className="text-[9.5px] text-tertiary truncate min-w-0 flex-1">
                            {cf.message}
                          </span>
                        )}
                        {flowActive && <span className="text-[9px] text-accent">当前</span>}
                      </summary>
                      <div className="flow-steps">
                        {cf.steps.map((step) => {
                          const stepActive =
                            active?.kind === "flow" &&
                            active.flowIndex === cf.index &&
                            active.stepIndex === step.index;
                          return (
                            <button
                              key={step.index}
                              className={cn("flow-step", stepActive && "active")}
                              onClick={() => onJumpFlow(cf.index, step.index)}
                              title={`${step.location.displayPath}:${step.location.startLine}`}
                            >
                              <span className="flow-step-index">{step.index}</span>
                              <span className="min-w-0 flex-1">
                                <span className="flow-step-msg">
                                  {step.location.labelZh || step.message || "（无节点标签）"}
                                </span>
                                <span className="flow-step-loc">
                                  {step.location.displayPath}:{step.location.startLine}
                                  {step.location.columnRange ? ` · 列 ${step.location.columnRange}` : ""}
                                </span>
                              </span>
                              {!step.location.resolved && (
                                <span className="badge bg-sev-warning-soft text-sev-warning border-sev-warning/25">
                                  未定位
                                </span>
                              )}
                              {step.index === 1 && (
                                <span className="flow-step-kind flow-step-kind--source">source</span>
                              )}
                              {step.index === cf.steps.length && cf.steps.length > 1 && (
                                <span className="flow-step-kind flow-step-kind--sink">sink</span>
                              )}
                              {step.kinds?.length > 0 && (
                                <span className="flow-step-kind">{step.kinds.join(" · ")}</span>
                              )}
                            </button>
                          );
                        })}
                      </div>
                    </details>
                  );
                })}
              </div>
              <div className="text-[9.5px] text-tertiary leading-relaxed mt-2">
                来自 SARIF <code>codeFlows[].threadFlows[].locations[]</code>，
                按 CodeQL 记录的顺序还原 source → 中间节点 → sink。点任意一步可跳到源码。
              </div>
            </Section>
          )}

          {/* 相关位置（SARIF relatedLocations） */}
          {finding.relatedLocations?.length > 0 && (
            <Section title={`相关位置 · ${finding.relatedLocations.length}`} icon={ListTree}>
              <div className="flex flex-col">
                {finding.relatedLocations.map((rel, i) => (
                  <ChainItem
                    key={`${rel.id}:${rel.location.file}:${rel.location.startLine}:${i}`}
                    tag={`关联 ${rel.id}`}
                    icon={GitBranch}
                    loc={{
                      ...rel.location,
                      label: rel.message || rel.location.label,
                    }}
                    active={
                      active?.kind === "link" &&
                      active.loc.file === rel.location.file &&
                      active.loc.startLine === rel.location.startLine
                    }
                    tone="source"
                    onClick={() =>
                      onLinkClick({
                        label: rel.message || rel.location.label,
                        labelZh: rel.location.labelZh,
                        rawUri: "",
                        file: rel.location.file,
                        startLine: rel.location.startLine,
                        startCol: rel.location.startCol,
                        endLine: rel.location.endLine,
                        endCol: rel.location.endCol,
                      })
                    }
                  />
                ))}
              </div>
              <div className="text-[9.5px] text-tertiary leading-relaxed mt-2">
                消息里的 <code>[标签](数字)</code> 引用与这里的 id 一一对应。
              </div>
            </Section>
          )}

          {/* 证据链 / 消息引用位置 */}
          <Section
            title={`${finding.codeFlows?.length > 0 ? "消息引用位置" : "证据链"} · ${
              1 + finding.sources.length
            }`}
            icon={GitBranch}
          >
            <div className="flex flex-col">
              <ChainItem
                tag="沉淀点"
                icon={Anchor}
                loc={finding.sink}
                active={active?.kind === "sink"}
                tone="sink"
                onClick={() => onJump("sink", 0)}
              />
              {finding.sources.map((s, i) => (
                <ChainItem
                  key={`${s.file}:${s.startLine}:${i}`}
                  tag={`来源 ${i + 1}`}
                  icon={GitBranch}
                  loc={s}
                  active={active?.kind === "source" && active.index === i}
                  tone="source"
                  onClick={() => onJump("source", i)}
                />
              ))}
            </div>
            <div className="text-[9.5px] text-tertiary leading-relaxed mt-2">
              行号/列号直接取自 CodeQL {finding.format === "sarif" ? "SARIF region" : "CSV"}，
              未做任何加减。若与本地源码对不上，通常是本地源码版本与生成数据库时不一致。
            </div>
          </Section>

          {/* SARIF 规则元数据与指纹 */}
          {finding.format === "sarif" && (finding.ruleInfo || finding.fingerprints?.length > 0) && (
            <Section title="SARIF 元数据" icon={Fingerprint}>
              <div className="meta-grid">
                <span>工具</span>
                <b className="font-mono break-all">
                  {finding.toolName || "CodeQL"}
                  {finding.toolVersion ? ` ${finding.toolVersion}` : ""}
                </b>
                <span>规则 ID</span>
                <b className="font-mono break-all">{finding.ruleName}</b>
                {finding.ruleInfo?.precision && (
                  <>
                    <span>Precision</span>
                    <b>{finding.ruleInfo.precision}</b>
                  </>
                )}
                {finding.ruleInfo?.kind && (
                  <>
                    <span>查询类型</span>
                    <b className="font-mono">{finding.ruleInfo.kind}</b>
                  </>
                )}
                {finding.ruleInfo?.securitySeverity && (
                  <>
                    <span>Security Score</span>
                    <b className="font-mono">{finding.ruleInfo.securitySeverity}</b>
                  </>
                )}
                {finding.ruleInfo?.problemSeverity && (
                  <>
                    <span>Problem Level</span>
                    <b className="font-mono">{finding.ruleInfo.problemSeverity}</b>
                  </>
                )}
              </div>

              {finding.ruleInfo?.helpUri && (
                <button
                  className="btn btn-sm btn-outline mt-2"
                  onClick={() => onOpenExternal(finding.ruleInfo!.helpUri)}
                >
                  <ExternalLink size={11} /> 打开规则帮助
                </button>
              )}

              {finding.tags?.length > 0 && (
                <div className="mt-2.5">
                  <div className="text-[9.5px] text-tertiary mb-1">Tags</div>
                  <div className="flex flex-wrap gap-1">
                    {finding.tags.map((tag) => (
                      <span key={tag} className="badge tag-chip font-mono">
                        {tag}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {finding.fingerprints?.length > 0 && (
                <div className="mt-2.5">
                  <div className="text-[9.5px] text-tertiary mb-1 flex items-center gap-1">
                    <Fingerprint size={10} /> 指纹（可用于跨扫描去重 / 结果对比）
                  </div>
                  <div className="fingerprint-list">
                    {finding.fingerprints.map((fp) => (
                      <div key={fp.name} className="fingerprint-row">
                        <span className="fingerprint-name">{fp.name}</span>
                        <code className="fingerprint-value">{fp.value}</code>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </Section>
          )}

          {/* 英文原文 */}
          <details className="raw-details detail-section">
            <summary>
              <span className="text-[10px] font-semibold tracking-wider">英文原文</span>
              <span className="text-[9px] text-tertiary ml-1">RAW CODEQL MESSAGE</span>
            </summary>
            <div className="raw-code">{finding.messageRaw || "（空）"}</div>
            <div className="raw-code" style={{ borderTop: "1px solid var(--border)" }}>
              rule: {finding.ruleName}
              {"\n"}description: {finding.description}
              {"\n"}message_key: {finding.messageKey}
              {finding.primaryFingerprint ? `\nprimary_fingerprint: ${finding.primaryFingerprint}` : ""}
            </div>
          </details>
        </div>
      </div>
    </aside>
  );
}

function Section({
  title,
  icon: Icon,
  children,
}: {
  title: string;
  icon: typeof FileText;
  children: React.ReactNode;
}) {
  return (
    <div className="detail-section">
      <div className="detail-section-head">
        <Icon size={11} className="text-accent" />
        {title}
      </div>
      <div className="detail-section-body">{children}</div>
    </div>
  );
}

function Segments({
  finding,
  segments,
  onLinkClick,
}: {
  finding: FindingDTO;
  segments: FindingDTO["messageEn"];
  onLinkClick: (link: LinkDTO) => void;
}) {
  if (!segments?.length) return <>（无详情）</>;
  return (
    <>
      {segments.map((seg, i) => {
        if (seg.kind === "link" && seg.link) {
          const link = seg.link;
          const label = seg.text || link.labelZh || link.label || link.file;
          return (
            <button
              key={i}
              className="link-inline"
              onClick={() => onLinkClick(link)}
              title={`跳转到 ${link.file}:${link.startLine}:${link.startCol}`}
            >
              {label}
              <ArrowUpRight size={10} className="inline-block ml-0.5 -mt-0.5 opacity-70" />
            </button>
          );
        }
        return <span key={i}>{seg.text}</span>;
      })}
      {finding.sources.length > 0 && (
        <span className="text-[9px] text-tertiary ml-1">
          （{finding.sources.length} 处数据流来源）
        </span>
      )}
    </>
  );
}

function ChainItem({
  tag,
  icon: Icon,
  loc,
  active,
  tone,
  onClick,
}: {
  tag: string;
  icon: typeof Anchor;
  loc: LocationDTO;
  active: boolean;
  tone: "sink" | "source";
  onClick: () => void;
}) {
  return (
    <button className={cn("chain-item", active && "active")} onClick={onClick}>
      <span
        className="chain-index"
        style={
          active
            ? undefined
            : {
                color: tone === "sink" ? "var(--red)" : "var(--amber)",
                borderColor: tone === "sink" ? "rgba(248,113,113,.3)" : "rgba(251,191,36,.3)",
                background: tone === "sink" ? "rgba(248,113,113,.1)" : "rgba(251,191,36,.1)",
              }
        }
      >
        <Icon size={10} />
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex items-center gap-1.5 flex-wrap">
          <span className="text-[9px] font-semibold text-tertiary tracking-wide">{tag}</span>
          <span className="text-[9px] text-tertiary">第 {loc.lineRange} 行</span>
          {loc.columnRange && <span className="text-[9px] text-tertiary">列 {loc.columnRange}</span>}
          {!loc.resolved && (
            <span className="badge bg-sev-warning-soft text-sev-warning border-sev-warning/25">
              未定位
            </span>
          )}
          {active && <span className="text-[9px] text-accent">当前</span>}
        </span>
        <span className="chain-file block mt-0.5">{loc.displayPath}</span>
        <span className="chain-sub">
          {loc.labelZh || loc.label ? <span>{loc.labelZh || loc.label}</span> : null}
          {loc.resolvedPath && (
            <span className="font-mono truncate max-w-full text-tertiary" title={loc.resolvedPath}>
              {loc.resolvedPath}
            </span>
          )}
        </span>
      </span>
      <ArrowUpRight size={12} className={cn("mt-0.5 flex-none", active ? "text-accent" : "text-tertiary")} />
    </button>
  );
}
