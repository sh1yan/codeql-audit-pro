# CodeQLAuditPro

**一个面向安全审计人员的 CodeQL 桌面工作台：从创建数据库、运行查询，到载入 SARIF / CSV、对应源码、追踪完整污点路径，全部在一个应用里完成。**

[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Wails](https://img.shields.io/badge/GUI-Wails%202.16%2B-DF0000)](https://wails.io)
[![React](https://img.shields.io/badge/React-19-61DAFB?logo=react&logoColor=black)](https://react.dev)
[![TypeScript](https://img.shields.io/badge/TypeScript-5.9-3178C6?logo=typescript&logoColor=white)](https://www.typescriptlang.org)
[![SARIF](https://img.shields.io/badge/SARIF-v2.1.0-6E4AFF)](https://docs.oasis-open.org/sarif/sarif/v2.1.0/sarif-v2.1.0.html)
[![Platform](https://img.shields.io/badge/platform-macOS%20%7C%20Windows%20%7C%20Linux-lightgrey)](#快速开始)
[![License](https://img.shields.io/badge/license-GPL--3.0-blue)](#开源许可)
![Version](https://img.shields.io/badge/version-v1.2.0-informational)

<a href="docs/screenshot-hub.png">
  <img src="docs/screenshot-hub.png" alt="CodeQLAuditPro 首页：一个工具，两个工作流" width="100%" />
</a>

---

## 目录

- [为什么是 CodeQLAuditPro](#为什么是-codeqlauditpro)
- [功能亮点](#功能亮点)
- [SARIF v2.1.0 支持](#sarif-v210-支持)
- [界面截图](#界面截图)
- [快速开始](#快速开始)
- [使用指南](#使用指南)
- [历史审计项目](#历史审计项目)
- [命令行模式](#命令行模式)
- [项目架构](#项目架构)
- [数据目录](#数据目录)
- [隐私与安全](#隐私与安全)
- [开发与测试](#开发与测试)
- [Roadmap](#roadmap)
- [参与贡献](#参与贡献)
- [开源许可](#开源许可)

---

## 为什么是 CodeQLAuditPro

CodeQL 的能力很强，但实际审计通常要在多个环节之间来回切换：

1. 用命令行创建数据库、选择查询套件、等待分析完成；
2. 从 SARIF / CSV 里翻找告警，再去 IDE 里定位源码；
3. 自己拼接污点路径、核对 CWE、判断是不是误报；
4. 最后整理成可交付的审计报告。

CodeQLAuditPro 把这些环节收进一个桌面应用，并围绕审计人员的实际操作组织界面：

- **工作流 01 · CodeQL 运行器**：可视化创建数据库、运行查询，输出 SARIF v2.1.0 或 CSV；
- **工作流 02 · 审计工作台**：载入结果与本地源码，逐行对应、追踪污点路径、标记已读、做 AI 误报验证、导出审计报告。

两个工作流共享同一个进程，切换页面不会停止正在运行的 CodeQL 任务，也不会丢失审计现场。

审计往往不是一次做完的。结果文件、源码目录、已读标记、AI 误报判定和路径映射都会记录成「历史项目」，下次进入审计工作台点一下 **继续审计** 就能接着上次的进度，不必重新开文件、重新定位源码。

---

## 功能亮点

| 能力 | 说明 |
|---|---|
| **SARIF v2.1.0 + CSV** | 同时支持 CodeQL 的两种主流结果格式，按文件扩展名 / 文件头自动识别 |
| **完整污点传播路径** | 还原 SARIF `codeFlows` 的每一个节点，按 source → 中间节点 → sink 展示 |
| **CWE 与规则元数据** | 展示 CWE、`precision`、`kind`、`security-severity`、`problem.severity` 与规则帮助链接 |
| **SARIF 指纹** | 展示 `partialFingerprints` / `fingerprints`，并作为已读状态与 AI 判定的稳定键 |
| **源码对应** | 容器路径 / 相对路径 → 本地源码，支持前缀规则与单文件覆盖 |
| **行列级定位** | CodeMirror 6 代码视图，沉淀点、来源点、路径节点全部高亮并可跳转 |
| **跨文件跳转** | 点击消息内联链接、相关位置或路径节点，直接打开对应文件和行列 |
| **中文审计界面** | 规则名、描述、漏洞详情离线中文化，翻译库可人工校正 |
| **一键补全翻译** | 直接标注待翻译条数，批量交给大模型补齐并实时显示进度，失败时给出具体原因 |
| **AI 误报验证** | 结合规则语义与源码片段，单条 / 批量验证并持久化结论 |
| **审计闭环** | 已读 / 未读、严重级别 / 规则 / 文件 / AI 判定筛选、跳转历史 |
| **历史审计项目** | 结果文件、源码目录、已读、AI 判定与路径映射落进本地 SQLite，启动即可继续审计 |
| **可交付导出** | 中文 CSV、32 列审计报告、AI 验证结果 CSV |
| **CodeQL 运行器** | 建库、跑查询、实时控制台、完整日志、配置快照、可复制命令 |
| **轻量启动** | 普通启动只读历史元数据，不解析上次的大文件；`-csv` 启动仍直接载入 |
| **命令行模式** | 不启动界面，直接解析 + 中文化 + 导出，便于批处理 |

---

## SARIF v2.1.0 支持

CSV 只有 9 列，CodeQL 在导出时已经把证据链压缩成消息文本；SARIF 则保留了完整的结构化数据。CodeQLAuditPro 会把这些字段全部解析出来，而不是只取「规则 + 位置」：

| SARIF 字段 | 在审计工作台中的用途 |
|---|---|
| `runs[].results[]` | 一条告警；规则、级别、消息、位置 |
| `tool.driver.rules[]` | 规则元数据：CWE、precision、kind、security-severity、帮助链接 |
| `properties.tags` | 提取 `external/cwe/cwe-089` 等标签，展示为可点击的 CWE |
| `codeFlows[].threadFlows[].locations[]` | 完整污点传播路径，逐节点展示与跳转 |
| `relatedLocations` | 相关位置列表；消息中的 `[标签](id)` 会变成可点击链接 |
| `partialFingerprints` / `fingerprints` | 指纹展示，并用于跨扫描稳定识别同一条告警 |
| `artifactLocation` + `region` | 把逻辑文件路径与行列映射到本地源码 |

载入 SARIF 后，右侧详情区会额外出现：

- **污点传播路径**：按 CodeQL 记录的顺序还原每一条路径，点击任意一步跳到源码；
- **相关位置**：`relatedLocations` 的标签、文件、行列；
- **SARIF 元数据**：工具版本、规则 ID、precision、查询类型、Security Score、Tags；
- **指纹**：`primaryLocationLineHash`、`primaryLocationStartColumnFingerprint` 等。

<a href="docs/screenshot-audit-sarif-full.png">
  <img src="docs/screenshot-audit-sarif-full.png" alt="SARIF 污点路径、相关位置、CWE 与指纹" width="100%" />
</a>

> CSV 仍然完整支持：同样的三栏界面、翻译、路径映射、AI 验证与导出能力都不会因为 SARIF 的加入而减少。

---

## 界面截图

### 工具选择首页

一个入口，两个工作流。进入任一工作台后，点左上角「首页」即可返回。

![工具选择首页](docs/screenshot-hub.png)

### 审计工作台 · SARIF

![审计工作台](docs/screenshot.png)

### 完整污点路径与 SARIF 元数据

![SARIF 元数据](docs/screenshot-audit-sarif-full.png)

### CodeQL 运行器

![CodeQL 运行器](docs/screenshot-assistant.png)

### 命令示例

按当前配置实时生成建库 / 分析命令，也可以复制按语言整理的成熟示例到终端独立执行。

![命令示例](docs/screenshot-commands.png)

### 批量 AI 验证

![批量 AI 验证](docs/screenshot-batch.png)

批量验证是后台任务：进度区、窗口底部与顶栏都有「强制停止」入口，
点下后不再发起新的判定请求，已完成的结论立即保存；
即使关掉批量窗口（或切到别的页面），顶栏也会常驻显示 `已完成/总数` 与停止按钮，
重新打开窗口会从后端读回真实进度，而不是显示成「未开始」。

### 路径映射

![路径映射](docs/screenshot-mapping.png)

浅色主题：

![浅色主题](docs/screenshot-audit-sarif-light.png)

---

## 快速开始

### 环境要求

- **Go 1.25+**
- **Node.js 20+** 与 **pnpm 10+**
- **Wails CLI 2.16+**：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **CodeQL CLI**（工作流 01 需要）：`codeql` 可在 PATH 中，或在界面里指定绝对路径
- 平台依赖：
  - macOS：Xcode Command Line Tools
  - Windows：WebView2 Runtime
  - Linux：WebKitGTK 与 gcc

先确认 Wails 环境：

```bash
wails doctor
```

### 安装依赖

```bash
git clone https://github.com/sh1yan/codeql-audit-pro
cd codeql-audit-pro

# 前端依赖
make deps
# 或：cd frontend && pnpm install
```

### 开发模式

```bash
make dev
# 等价于 wails dev
```

`wails dev` 会同时启动 Vite 与原生窗口，React 代码热更新，Go 代码自动重新编译。

如果只想在浏览器里查看界面（使用内置 mock 数据，不需要后端）：

```bash
cd frontend
pnpm dev
# http://localhost:34115
```

### 打包

```bash
make build          # 等价于 wails build，产物在 build/bin/
make build-fast     # 跳过 pnpm install，只重建前端后打包
```

macOS 产物：`build/bin/CodeQLAuditPro.app`。

> macOS 首次从「下载」目录启动时，如果系统询问是否允许访问该目录，选择「允许」即可；
> 也可以把 `.app` 拖到 `/Applications` 或 `~/Applications` 后再启动。

普通启动不会自动解析上一次的结果文件；进入审计工作台后，从历史项目列表点「继续审计」即可恢复现场。

---

## 使用指南

### 工作流 01 · 创建数据库与运行查询

1. 首页进入 **CodeQL 运行器**；
2. 填写 CodeQL 二进制、查询 / 套件、数据库目录、源码根目录；
3. 选择语言，点击 **创建数据库**，在右侧控制台查看实时日志；
4. 选择输出格式：
   - **SARIF v2.1.0**：推荐，审计工作台可以展示完整污点路径、CWE 与指纹；
   - **CSV**：兼容旧流程与轻量导出；
5. 指定输出文件，点击 **开始分析**；
6. 分析完成后，控制台上方会出现「用审计工作台打开」按钮，直接带着结果进入工作流 02。

命令行等价示例：

```bash
# 1. 创建 Python 数据库
codeql database create ~/codeql-db/py-demo \
  --language=python \
  --source-root=/path/to/python-project \
  --threads=4 --ram=4096 --overwrite

# 2. 运行官方安全套件，输出 SARIF v2.1.0（推荐）
codeql database analyze ~/codeql-db/py-demo \
  codeql/python-queries:codeql-suites/python-security-extended.qls \
  --download \
  --format=sarifv2.1.0 \
  --output=~/py-results.sarif \
  --threads=4 --ram=4096

# 3. 或输出 CSV（兼容旧流程）
codeql database analyze ~/codeql-db/py-demo \
  codeql/python-queries:codeql-suites/python-security-extended.qls \
  --download \
  --format=csv \
  --output=~/py-results.csv \
  --threads=4 --ram=4096
```

> 内存限额或线程数填 `0` 时，应用生成的命令会省略对应的 `--ram` / `--threads`，由 CodeQL 自己决定。

编译型语言需要构建命令：

```bash
codeql database create ~/codeql-db/java-demo \
  --language=java \
  --source-root=/path/to/java-project \
  --command="mvn -B -DskipTests clean package" \
  --threads=4 --ram=4096 --overwrite
```

更多语言、查询包、自定义查询与资源调优示例，在应用内的「命令示例」sheet 里，带一键复制按钮。

### 工作流 02 · 载入结果与源码审计

1. 首页进入 **审计工作台**；如果之前审计过，直接在启动页的历史项目列表里点 **继续审计**，已读、AI 判定与路径映射会一并恢复；
2. 点击 **打开结果文件**，选择 `.sarif` / `.sarif.json` / `.json` / `.csv`；
3. 点击 **源码目录**，选择被审计的源码根目录；
4. 左栏选择告警 → 中栏自动打开源码并高亮 → 右栏查看详情；
5. 如果是 SARIF，右侧会展示：
   - 污点传播路径（可逐节点跳转）；
   - 相关位置与消息内联链接；
   - CWE、Tags、precision、security-severity；
   - SARIF 指纹；
6. 看完自动标为已读；可用「只看未读」继续；
7. 需要时做 AI 误报验证、路径映射、导出审计报告；
8. 审计告一段落时，从「功能 → 退出当前项目」回到选择界面，进度已保存在历史项目里。

**关于源码对应**

CodeQL 数据库通常在容器或构建机上创建，结果里的路径可能是 `/app/views/user.py` 或 `app/views/user.py`。审计工作台按以下顺序定位本地文件：

1. 人工配置的单文件覆盖；
2. 前缀替换规则；
3. 相对路径后缀匹配；
4. 同名文件匹配（有歧义时提示）。

在「路径映射」里可以查看未定位文件、添加规则或手动指定本地文件；定位结果会实时回写到告警、来源点和 codeFlow 的每一步。

### 补全翻译

规则名、规则描述和漏洞详情默认使用离线翻译库。遇到库里没有的条目时：

1. 顶栏「功能 → 补全翻译」会显示还有多少条内容缺中文；
2. 点击后应用把缺失内容交给你在「设置 → 模型与 AI」里配置的 OpenAI 兼容接口，浮层实时显示进度，完成后列表、详情与统计立即刷新；
3. 补全结果会写回 `translations.db`，下次遇到同一规则不再重复请求；
4. 如果仍有失败，通知里会带上最近一次错误（超时、401、模型名不对等），修正设置后可再次点击。

该功能依赖「设置 → 模型与 AI → 启用大模型翻译兜底」以及接口地址、模型与 API Key；未配置时点击会直接提示，不会发出请求。

### 常用快捷键

| 快捷键 | 功能 |
|---|---|
| `Cmd/Ctrl + F` | 在当前源码文件中查找（可在设置里修改） |
| `Esc` | 关闭查找框 |
| 首页 / 工作台切换 | 不中断正在运行的 CodeQL 任务 |

---

## 历史审计项目

每次成功载入结果文件后，应用都会把这次审计记成一个「历史项目」：结果文件、源码目录、已读标记、AI 误报判定、路径前缀规则与手工映射。记录保存在本地 `history.db`（SQLite），与 `config.json` 分离，配置文件不会随项目数量膨胀。

### 启动页

启动页列出最近打开的 6 个项目，每条显示：

- 结果文件名、路径（长路径自动省略中段）与格式标签（SARIF v2.1.0 / CSV）；
- 源码目录，未设置时显示「未设置源码目录」；
- 总条数、已读数量、AI 判定数量与最近打开时间；
- **继续审计**：重新解析结果并恢复现场；结果文件被移动或删除时按钮禁用，并标注「结果文件缺失」。

超过 6 个时点「查看全部」进入完整列表。

### 管理历史项目

「功能 → 历史项目」或启动页的「管理 / 删除」会打开历史项目弹窗，可以：

- 查看全部记录，当前正在审计的项目会高亮并标注「当前」；
- 每条记录额外显示格式、文件大小、工具名、已定位 / 总条数与最近打开时间；
- 结果文件或源码目录不存在时给出「结果文件已不存在」/「源码目录不可用」提示；
- 载入任意一条记录（结果文件不存在时会直接提示，不会启动解析）；
- 删除单条记录，或清空全部；
- 点「刷新」重新读取 `history.db`。

### 删除会删掉什么

删除历史项目只移除 **已读标记、AI 判定、路径前缀规则和手工映射**，不会删除：

- 结果文件（`.sarif` / `.csv`）；
- 源码目录及其中的文件；
- 全局翻译库 `translations.db`。

删除当前项目或主动退出项目后会自动回到选择界面，不需要重启应用；也可以随时从「功能 → 退出当前项目」主动退出。

### 启动行为

- **普通双击启动**：打开后停在「首页」，进入审计工作台时显示历史项目选择页，不会自动解析上一次的结果文件；
- **带 `-csv` / `-results` 启动**：直接进入审计工作台并载入指定结果；
- **旧版配置迁移**：`config.json` 里遗留的 `projects` 数组会在首次启动时导入 `history.db`，原文件备份为 `config.json.bak`，随后从 `config.json` 中移除。

---

## 命令行模式

不启动图形界面，直接解析 + 中文化 + 导出：

```bash
# 源码方式
go run . -cli -csv ../pyhackme.sarif -src /path/to/src -out result_中文.csv

# 打包后的可执行文件
./build/bin/CodeQLAuditPro.app/Contents/MacOS/CodeQLAuditPro \
  -cli -csv ../pyhackme.sarif -src /path/to/src -out result_中文.csv
```

`-csv` 在图形界面下同样适用：`CodeQLAuditPro -csv x.sarif` 会直接进入审计工作台并载入该文件；不带 `-csv` 的普通启动则停在选择页。

| 参数 | 说明 |
|---|---|
| `-csv <路径>` | CodeQL 结果文件（CSV 或 SARIF v2.1.0） |
| `-results <路径>` | 同 `-csv`，语义更清晰的别名 |
| `-src <路径>` | 被审计的源码目录 |
| `-cli` | 命令行模式（不启动图形界面） |
| `-out <路径>` | 命令行模式下导出的中文 CSV |
| `-fonts` | 打印 WebView 中文字体说明 |
| `-version` | 版本信息 |

---

## 项目架构

```text
codeql-audit-pro/
├── app.go                  # Wails 后端：项目载入、路径映射、已读状态
├── app_ai.go               # AI 误报验证（单条 / 批量）与补全翻译
├── app_assistant.go        # CodeQL 运行器绑定、事件、对话框
├── app_dto.go              # 暴露给前端的 DTO（含 SARIF 字段）
├── app_export.go           # 中文 CSV / 审计报告 / AI 结果导出
├── app_history.go          # 历史审计项目：读取、删除、清空、退出项目
├── cli.go                  # 命令行模式
├── main.go                 # Wails 启动、窗口、CLI 参数
│
├── internal/
│   ├── codeqlcsv/          # CSV 解析、中文九列导出、32 列审计报告
│   ├── codeqlsarif/        # SARIF v2.1.0 解析（codeFlow / CWE / 指纹）
│   ├── project/            # 结果格式识别、翻译与定位编排
│   ├── model/              # 领域模型（Finding / CodeFlow / RuleInfo …）
│   ├── pathmap/            # 逻辑路径 → 本地源码
│   ├── sourcefile/         # 源码读取与编码探测
│   ├── translate/          # 离线翻译引擎与翻译库
│   ├── verify/             # AI 误报判定
│   ├── config/             # 配置、历史项目 SQLite、数据目录迁移
│   ├── editor/             # 外部编辑器探测与调用
│   └── assistant/          # CodeQL 进程、日志、历史、命令生成
│
└── frontend/
    └── src/
        ├── App.tsx                     # 首页 / 审计 / 运行器三层，历史项目与补全翻译
        ├── components/
        │   ├── Hub.tsx                 # 一个工具 + 两个工作流入口
        │   ├── StartScreen.tsx         # 审计选择页 + 历史项目列表
        │   ├── ProjectHistoryModal.tsx # 历史项目管理 / 删除 / 清空
        │   ├── AssistantWorkspace.tsx  # CodeQL 运行器
        │   ├── CommandSheet.tsx        # 命令示例
        │   ├── Sidebar.tsx             # 告警列表与筛选（动态行高虚拟列表）
        │   ├── CodePanel.tsx           # 源码视图与路径节点高亮
        │   ├── DetailPanel.tsx         # 详情、codeFlow、CWE、指纹
        │   └── ...
        ├── lib/api.ts                  # Wails 调用 / 事件 / mock
        └── types.ts                    # 前端 DTO 类型
```

### 为什么切换工作台不会中断

- CodeQL 进程由 Go 侧 `internal/assistant.Runner` 在后台 goroutine 中运行；
- 前端把首页、审计工作台、运行器同时挂载，用 `display: none` 切换可见性；
- 切换页面不会卸载组件，因此控制台内容、滚动位置、筛选、选中告警、查找状态都不会丢；
- 审计进度（已读、AI 判定、路径映射）会自动保存到 `history.db`，退出项目或重启应用都能恢复；
- 只有用户点「强制停止」或关闭应用，CodeQL 进程才会结束。

### 后端事件

| 事件 | 载荷 | 用途 |
|---|---|---|
| `project:progress` | `{phase, done, total, message}` | 载入结果、翻译、定位进度 |
| `assistant:output` | `{lines: [{text, error}], reset}` | 运行器控制台输出 |
| `assistant:status` | `{running, status, statusLabel, action, startedAt, elapsedMs, logPath}` | 运行状态与耗时 |
| `assistant:done` | `{action, status, statusLabel, outputFile, durationMs, error}` | 任务结束、结果联动 |
| `translate:progress` | `{done, total, message}` | 补全翻译进度 |
| `batch:progress` / `batch:done` | 批量 AI 验证进度 / 汇总 | 实时回填判定结果 |
| `batch:stopping` | `{done, total}` | 用户点了「强制停止」，剩余条目不再发起请求 |

前端还可以直接调用 `BatchState()` 拿回 `{running, stopping, done, total, ...}`：
窗口被关掉再打开、或界面重新载入后，用它恢复真实进度与停止入口。

---

## 数据目录

macOS / Windows / Linux 统一使用用户主目录下的 `.CodeQLAuditPro/`：

```text
~/.CodeQLAuditPro/
├── config.json                 # 设置与界面偏好（不再保存项目历史）
├── config.json.bak             # 旧配置一次性迁移时的备份（仅迁移后存在）
├── history.db                  # 历史审计项目（已读、AI 判定、路径映射）
├── translations.db             # 离线翻译库（SQLite）
└── assistant/                  # CodeQL 运行器
    ├── last_config.json        # 上次运行参数
    ├── history.json            # 运行历史（最多 100 条）
    ├── profiles/               # 配置快照
    │   └── *.json
    └── logs/                   # 完整运行日志（保留 3 天）
        └── *.log
```

`history.db` 与 `translations.db` 都使用 SQLite。`history.db` 开启 WAL 模式，运行时会同时出现 `history.db-wal` / `history.db-shm`，属于正常现象，无需手动删除。

设置环境变量 `CODEQLAUDITPRO_HOME` 可以切换到绿色 / 便携目录：

```bash
CODEQLAUDITPRO_HOME=/path/to/portable ./CodeQLAuditPro
```

旧版 `CODEQL_AUDIT_HOME` 及 `~/Library/Application Support/codeql-audit/` 的数据会在首次启动时自动迁移到新目录，只迁移一次，不覆盖已有文件。

如果旧版 `config.json` 里还保留着 `projects` 数组，启动时会自动导入 `history.db`，随后从 `config.json` 中移除，避免配置文件随审计项目增多而无限膨胀。迁移前会把原文件备份为 `config.json.bak`。

`history.db` 保存的是审计记录本身。删除历史项目只清理这些记录，不会删除结果文件、源码目录或翻译库。

---

## 隐私与安全

- 所有 CodeQL 分析都在本机运行，不上传源码、数据库或结果；
- 历史项目、已读标记、AI 判定与路径映射只写入本地 `history.db`，不会上传；
- AI 误报验证默认**关闭**；启用后会把规则、详情和必要的源码片段发送到**你自己配置的** OpenAI 兼容接口；
- 「补全翻译」只在手动点击时发起请求，发送待翻译的规则名、描述与漏洞消息文本，不发送源码文件；
- 可在设置中关闭「AI 验证时发送源码片段」，只发送位置与规则信息；
- CodeQL 运行器不会上传配置、日志或源码；
- 「命令示例」只是文本，不执行 shell；命令由用户自行复制到终端；
- 应用启动时会读取 zsh / bash 登录环境以补齐 PATH，不会把环境内容写入磁盘。

---

## 开发与测试

```bash
make test           # go test ./...
make typecheck      # tsc --noEmit
make vet            # go vet ./...
make fmt            # gofmt
make build          # wails build
make check          # fmt + vet + typecheck + test
```

测试覆盖：

- SARIF v2.1.0 解析：规则元数据、CWE、codeFlow、relatedLocations、指纹与稳定键；
- CSV 解析与 32 列审计报告；
- 历史审计项目：旧 `config.json` 的 `projects` 迁移到 `history.db`、迁移后重写配置、删除 / 清空与 `last_project` 清理；
- 翻译补全：`needsTranslation` 判定（含大模型回退英文仍算缺失）、`TranslateMissing` 清空待翻译标记；
- CodeQL 运行器：参数切分、命令生成、配置快照、历史持久化、进程停止；
- Wails 后端编排：任务启动、输出收集、历史写入、日志落盘。

---

## Roadmap

- [x] SARIF v2.1.0 结果解析与源码对应
- [x] 完整 codeFlow 污点传播路径展示
- [x] CWE、规则元数据、指纹展示
- [x] 32 列审计报告导出
- [x] SARIF suppressions / baselineState 展示
- [x] 基于指纹的两次扫描结果对比（新增 / 已修复 / 仍存在）
- [x] CWE / Tag 维度筛选
- [x] GitHub Code Scanning 上传辅助
- [x] 更多语言的构建命令模板
- [x] 历史审计项目（本地 SQLite，可继续审计 / 删除 / 清空）
- [x] 一键补全翻译（进度提示与失败原因）
- [x] 启动不再自动解析上次结果，`-csv` 仍直接载入

---

## 参与贡献

欢迎提交 Issue、功能建议与 Pull Request。

提交 PR 前建议先跑：

```bash
make check
```

如果改动涉及 SARIF 字段或 UI，请在 PR 描述里附上示例结果文件和截图，方便复现与评审。

---

## 开源许可

本项目基于 **GPL-3.0** 发布，详见 [LICENSE](LICENSE)。

致谢：

- [CodeQL](https://codeql.github.com/) — 查询引擎与 SARIF 输出；
- [Wails](https://wails.io/) — Go + Web 桌面应用框架；
- [React](https://react.dev/) / [TypeScript](https://www.typescriptlang.org/) — 前端；
- [CodeMirror 6](https://codemirror.net/) — 代码视图与语法高亮。

---

<p align="center">
  为 CodeQL 审计人员而做 · 让每一条告警都能落到源码与数据流上
</p>
