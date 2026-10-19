import { useEffect, useState } from "react";
import {
  Bot,
  Check,
  Code2,
  Eye,
  EyeOff,
  ExternalLink,
  FolderOpen,
  Info,
  KeyRound,
  Link2,
  Loader2,
  MonitorCog,
  Save,
  Settings2,
  ShieldCheck,
  Sparkles,
  SquarePen,
  TerminalSquare,
  TestTube2,
} from "lucide-react";
import type { FileDTO, SettingsDTO } from "../types";
import { api } from "../lib/api";
import { Modal, Spinner, Toggle, WarningBox } from "./ui";
import { cn } from "../lib/format";

type Tab = "model" | "editor" | "code" | "about";

export function SettingsModal({
  open,
  onClose,
  settings,
  defaults,
  currentFile,
  currentLine,
  version,
  platform,
  onSaved,
  notify,
}: {
  open: boolean;
  onClose: () => void;
  settings: SettingsDTO;
  defaults: { searchShortcut: string; editorArgs: string };
  currentFile: FileDTO | null;
  currentLine: number;
  version: string;
  platform: string;
  onSaved: (s: SettingsDTO) => void;
  notify: (kind: "info" | "success" | "error", message: string) => void;
}) {
  const [tab, setTab] = useState<Tab>("model");
  const [draft, setDraft] = useState<SettingsDTO>(settings);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [showKey, setShowKey] = useState(false);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ ok: boolean; text: string } | null>(null);

  useEffect(() => {
    if (open) {
      setDraft(structuredClone(settings));
      setError("");
      setTestResult(null);
      setTab("model");
    }
  }, [open, settings]);

  const patch = (p: Partial<SettingsDTO>) => setDraft((d) => ({ ...d, ...p }));
  const patchLLM = (p: Partial<SettingsDTO["llm"]>) =>
    setDraft((d) => ({ ...d, llm: { ...d.llm, ...p } }));
  const patchEditor = (p: Partial<SettingsDTO["editor"]>) =>
    setDraft((d) => ({ ...d, editor: { ...d.editor, ...p } }));

  const save = async () => {
    setSaving(true);
    setError("");
    try {
      const saved = await api.SaveSettings(draft);
      onSaved(saved);
      notify("success", "设置已保存");
      onClose();
    } catch (err) {
      setError(String(err));
    } finally {
      setSaving(false);
    }
  };

  const test = async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const res = await api.TestConnection(draft.llm);
      setTestResult({
        ok: true,
        text: `连接成功\n接口：${res.endpoint}\n模型：${res.model}\n回复：${res.reply}`,
      });
    } catch (err) {
      setTestResult({ ok: false, text: String(err) });
    } finally {
      setTesting(false);
    }
  };

  const testEditor = async () => {
    if (!currentFile) {
      notify("info", "还没有打开文件，请先选择一条漏洞。");
      return;
    }
    try {
      const desc = await api.OpenInEditor(currentFile.path, currentLine || 1, 1);
      notify("success", `已调用编辑器：${desc}`);
    } catch (err) {
      notify("error", String(err));
    }
  };

  const tabs: { key: Tab; label: string; icon: typeof Bot }[] = [
    { key: "model", label: "模型与 AI", icon: Bot },
    { key: "editor", label: "外部编辑器", icon: SquarePen },
    { key: "code", label: "代码视图", icon: Code2 },
    { key: "about", label: "关于与路径", icon: Info },
  ];

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="设置"
      subtitle="大模型、AI 误报验证、代码视图与外部编辑器"
      icon={Settings2}
      size="lg"
      footer={
        <>
          <span className="text-[10px] text-tertiary mr-auto">
            配置保存在 <span className="font-mono">{draft.configPath}</span>
          </span>
          <button className="btn btn-outline" onClick={onClose}>
            取消
          </button>
          <button className="btn btn-primary" onClick={save} disabled={saving}>
            {saving ? <Spinner size={12} /> : <Save size={13} />}
            保存设置
          </button>
        </>
      }
    >
      <div className="settings-layout">
        <nav className="settings-nav">
          {tabs.map((t) => (
            <button
              key={t.key}
              className={cn(tab === t.key && "active")}
              onClick={() => setTab(t.key)}
            >
              <t.icon size={13} />
              {t.label}
            </button>
          ))}
        </nav>

        <div className="min-h-0">
          {error && (
            <div className="mb-3">
              <WarningBox error>{error}</WarningBox>
            </div>
          )}

          {tab === "model" && (
            <div className="form-grid">
              <div className="warning-box">
                <Sparkles size={14} className="mt-0.5 flex-none" />
                <div>
                  <b>两个开关相互独立。</b>「翻译兜底」只管未命中翻译库时调用大模型；
                  「AI 误报验证」只管右栏的误报判定。关掉翻译不会影响 AI 验证。
                </div>
              </div>

              <div className="detail-section">
                <div className="detail-section-head">
                  <Bot size={11} className="text-accent" /> 大模型（OpenAI 兼容接口）
                </div>
                <div className="p-3 flex flex-col gap-3">
                  <Toggle
                    checked={draft.llm.enabled}
                    onChange={(v) => patchLLM({ enabled: v })}
                    label="启用大模型翻译兜底（仅在未命中翻译库时调用）"
                  />
                  <div className="form-row">
                    <label className="form-label">接口地址</label>
                    <div>
                      <div className="relative">
                        <Link2 size={12} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-tertiary" />
                        <input
                          className="input pl-8 font-mono text-[11px]"
                          value={draft.llm.baseUrl}
                          onChange={(e) => patchLLM({ baseUrl: e.target.value })}
                          placeholder="https://api.deepseek.com/v1"
                          spellCheck={false}
                        />
                      </div>
                      <div className="form-hint">兼容 DeepSeek / OpenAI / vLLM / Ollama 的 /v1 接口。</div>
                    </div>
                  </div>
                  <div className="form-row">
                    <label className="form-label">API Key</label>
                    <div>
                      <div className="relative">
                        <KeyRound size={12} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-tertiary" />
                        <input
                          className="input pl-8 pr-9 font-mono text-[11px]"
                          type={showKey ? "text" : "password"}
                          value={draft.llm.apiKey}
                          onChange={(e) => patchLLM({ apiKey: e.target.value })}
                          placeholder="sk-...（本地模型可留空）"
                          spellCheck={false}
                        />
                        <button
                          className="absolute right-1 top-1/2 -translate-y-1/2 btn btn-icon btn-sm"
                          onClick={() => setShowKey((v) => !v)}
                          title={showKey ? "隐藏" : "显示"}
                        >
                          {showKey ? <EyeOff size={12} /> : <Eye size={12} />}
                        </button>
                      </div>
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div className="form-row" style={{ gridTemplateColumns: "1fr" }}>
                      <label className="form-label">模型</label>
                      <input
                        className="input font-mono text-[11px]"
                        value={draft.llm.model}
                        onChange={(e) => patchLLM({ model: e.target.value })}
                        placeholder="deepseek-chat"
                        spellCheck={false}
                      />
                    </div>
                    <div className="form-row" style={{ gridTemplateColumns: "1fr" }}>
                      <label className="form-label">超时（秒）</label>
                      <input
                        className="input"
                        type="number"
                        min={5}
                        value={draft.llm.timeoutSeconds}
                        onChange={(e) => patchLLM({ timeoutSeconds: Number(e.target.value) || 60 })}
                      />
                    </div>
                  </div>
                  <Toggle
                    checked={draft.llm.templateOnly}
                    onChange={(v) => patchLLM({ templateOnly: v })}
                    label="翻译隐私：只发送消息模板与占位符（不发送源码）"
                  />
                  <div className="flex items-center gap-2">
                    <button className="btn btn-outline btn-sm" onClick={test} disabled={testing}>
                      {testing ? <Spinner size={11} /> : <TestTube2 size={11} />}
                      测试连接
                    </button>
                    {testResult && (
                      <span
                        className={cn(
                          "text-[10px] whitespace-pre-wrap",
                          testResult.ok ? "text-ai-fp" : "text-sev-error",
                        )}
                      >
                        {testResult.text}
                      </span>
                    )}
                  </div>
                </div>
              </div>

              <div className="detail-section">
                <div className="detail-section-head">
                  <ShieldCheck size={11} className="text-accent" /> AI 误报验证
                </div>
                <div className="p-3 flex flex-col gap-3">
                  <Toggle
                    checked={draft.aiVerifyEnabled}
                    onChange={(v) => patch({ aiVerifyEnabled: v })}
                    label="启用 AI 误报验证（独立于上面的翻译开关）"
                  />
                  <Toggle
                    checked={draft.aiSendCode}
                    onChange={(v) => patch({ aiSendCode: v })}
                    label="验证时发送沉淀点/来源点附近的源码片段"
                  />
                  <div className="form-hint">
                    关闭后只发送规则、漏洞详情与文件位置，不发送任何源码内容。
                    判定结果按项目持久化，重新打开同一份 CSV 会自动回填。
                  </div>
                </div>
              </div>
            </div>
          )}

          {tab === "editor" && (
            <div className="form-grid">
              <div className="detail-section">
                <div className="detail-section-head">
                  <TerminalSquare size={11} className="text-accent" /> 用编辑器打开
                </div>
                <div className="p-3 flex flex-col gap-3">
                  <div className="form-row">
                    <label className="form-label">编辑器</label>
                    <div>
                      <input
                        className="input font-mono text-[11px]"
                        value={draft.editor.cmd}
                        onChange={(e) => patchEditor({ cmd: e.target.value })}
                        placeholder="/usr/local/bin/code 或 /Applications/Visual Studio Code.app"
                        spellCheck={false}
                      />
                      <div className="form-hint">
                        留空自动探测 VS Code / Cursor / Sublime / Zed。macOS 也可以直接填 .app 包路径。
                      </div>
                    </div>
                  </div>
                  <div className="form-row">
                    <label className="form-label">参数模板</label>
                    <div>
                      <input
                        className="input font-mono text-[11px]"
                        value={draft.editor.args}
                        onChange={(e) => patchEditor({ args: e.target.value })}
                        placeholder={defaults.editorArgs}
                        spellCheck={false}
                      />
                      <div className="form-hint">
                        占位符：{"{file}"} 文件绝对路径、{"{line}"} 行号、{"{col}"} 列号。默认{" "}
                        <span className="font-mono">{defaults.editorArgs}</span> 适用于 VS Code / Cursor；
                        Sublime 可用 <span className="font-mono">{"{file}:{line}"}</span>。
                      </div>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button className="btn btn-outline btn-sm" onClick={testEditor}>
                      <MonitorCog size={11} />
                      测试打开当前文件
                    </button>
                    <span className="text-[10px] text-tertiary">
                      {currentFile ? currentFile.path : "还没有打开文件"}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          )}

          {tab === "code" && (
            <div className="form-grid">
              <div className="detail-section">
                <div className="detail-section-head">
                  <Code2 size={11} className="text-accent" /> 代码视图
                </div>
                <div className="p-3 flex flex-col gap-4">
                  <Toggle
                    checked={draft.showColumnRuler}
                    onChange={(v) => patch({ showColumnRuler: v })}
                    label="显示列标尺（在代码上方标出「第几列」）"
                  />
                  <div className="form-hint">
                    列标尺与代码列位置严格对齐：每 10 列一个刻度，当前聚焦的沉淀点/来源点画成色带，
                    并贴着「列 26-28」色块（列号取自 CSV 原值）。
                  </div>
                  <div className="divider" />
                  <div className="form-row">
                    <label className="form-label">查找快捷键</label>
                    <div>
                      <div className="flex items-center gap-2">
                        <input
                          className="input font-mono text-[11px]"
                          value={draft.codeSearchShortcut}
                          onChange={(e) => patch({ codeSearchShortcut: e.target.value })}
                          placeholder={defaults.searchShortcut}
                          spellCheck={false}
                        />
                        <button
                          className="btn btn-outline btn-sm whitespace-nowrap"
                          onClick={() => patch({ codeSearchShortcut: defaults.searchShortcut })}
                        >
                          恢复默认
                        </button>
                      </div>
                      <div className="form-hint">
                        格式：修饰键 + 主键，例如 {defaults.searchShortcut}、Ctrl+F、Ctrl+Shift+G、Alt+F3。
                        留空即恢复默认。格式不合法时保存会报错并保留原键位。
                      </div>
                    </div>
                  </div>
                </div>
              </div>

              <div className="detail-section">
                <div className="detail-section-head">
                  <Info size={11} className="text-accent" /> 交互提示
                </div>
                <div className="p-3 grid grid-cols-2 gap-2 text-[11px] text-secondary leading-relaxed">
                  <div>拖拽代码可跨行选中</div>
                  <div>Cmd/Ctrl+C 复制选中或当前行</div>
                  <div>Cmd/Ctrl+A 全选整个文件</div>
                  <div>右键菜单可复制/全选/编辑器打开</div>
                  <div>查找命中不会覆盖漏洞高亮</div>
                  <div>切换文件后查找词会保留并重新匹配</div>
                </div>
              </div>
            </div>
          )}

          {tab === "about" && (
            <div className="form-grid">
              <div className="detail-section">
                <div className="detail-section-head">
                  <Info size={11} className="text-accent" /> 运行信息
                </div>
                <div className="p-3">
                  <table className="data-table">
                    <tbody>
                      <tr>
                        <td className="strong w-28">版本</td>
                        <td className="font-mono">v{version} · Wails</td>
                      </tr>
                      <tr>
                        <td className="strong">平台</td>
                        <td>{platform}</td>
                      </tr>
                      <tr>
                        <td className="strong">数据目录</td>
                        <td className="font-mono break-all">{draft.dataDir}</td>
                      </tr>
                      <tr>
                        <td className="strong">配置文件</td>
                        <td className="font-mono break-all">{draft.configPath}</td>
                      </tr>
                      <tr>
                        <td className="strong">翻译库</td>
                        <td className="font-mono break-all">{draft.translateDb}</td>
                      </tr>
                    </tbody>
                  </table>
                  <div className="flex gap-2 mt-3">
                    <button
                      className="btn btn-outline btn-sm"
                      onClick={() =>
                        api.RevealInFinder(draft.dataDir).catch((err) => notify("error", String(err)))
                      }
                    >
                      <FolderOpen size={11} /> 打开数据目录
                    </button>
                    <button
                      className="btn btn-outline btn-sm"
                      onClick={() => void api.OpenExternalURL("https://codeql.github.com/")}
                    >
                      <ExternalLink size={11} /> CodeQL 官网
                    </button>
                  </div>
                </div>
              </div>

              <div className="detail-section">
                <div className="detail-section-head">
                  <ShieldCheck size={11} className="text-accent" /> 隐私说明
                </div>
                <div className="p-3 text-[11px] text-secondary leading-relaxed flex flex-col gap-2">
                  <div>
                    <b className="text-primary">翻译：</b>默认只把「消息模板 + 占位符」发给大模型，
                    不发送源码内容。
                  </div>
                  <div>
                    <b className="text-primary">AI 误报验证：</b>会发送规则信息与漏洞详情；
                    源码片段由「AI 验证时发送源码片段」控制，可以完全关闭。
                  </div>
                  <div>
                    <b className="text-primary">离线可用：</b>内置词典、SQLite 翻译库与模板化翻译都在本地，
                    不联网也能完成常见 CodeQL 结果的中文化。
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>
    </Modal>
  );
}

