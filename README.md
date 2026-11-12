<p align="center">
  <img src="frontend/public/qterm-icon.png" width="96" alt="Qterm 图标" />
</p>

<h1 align="center">Qterm</h1>

<p align="center">把终端、文件传输、远程桌面与日常运维放进同一个工作台。</p>

<p align="center">
  <strong>简体中文</strong> ·
  <a href="README.zh-TW.md">繁體中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a> ·
  <a href="README.ko.md">한국어</a> ·
  <a href="README.fr.md">Français</a> ·
  <a href="README.ru.md">Русский</a>
</p>

Qterm 是基于 **Go、Wails v3、Vue 3 和 TypeScript** 构建的桌面连接管理工具，面向 macOS、Windows 和 Linux。以主机资产为入口，在多标签、分屏工作区中连接服务器和设备，并通过侧边工具面板处理文件、监控资源、管理容器及使用 AI 助手。

支持简体中文、繁體中文、English、Français、日本語、한국어和 Русский 七种界面语言。

## 界面预览

**终端、SFTP 与右键工具**

![终端、SFTP 与右键工具](public/img_3.png)

<details>
<summary>分屏工作区与自定义背景</summary>

![分屏工作区与自定义背景](public/img_4.png)

</details>

<details>
<summary>浅色与深色界面</summary>

**浅色首页**

![浅色首页](public/img.png)

**深色首页**

![深色首页](public/img_2.png)

</details>

<details>
<summary>应用设置</summary>

![应用设置](public/img_1.png)

</details>

## 功能概览

### 主机与连接

- **统一资产库**：管理主机与嵌套分组，支持搜索、标签、备注、自定义图标与颜色，以及侧栏和全屏资产视图。
- **多种连接方式**：SSH、Telnet、串口、RDP、VNC，并可打开本机 Shell。
- **SSH 认证**：密码、私钥、SSH Agent、交互式认证，以及关联 OTP 条目的双因素验证配置。
- **复杂网络访问**：SSH 跳板链、SOCKS5 / HTTP 代理；代理配置可复用，RDP 与 VNC 也可配置跳板或代理链路。
- **连接选项**：按协议设置终端、编码、保活、登录后命令、SFTP 和远程桌面参数。

### 终端工作区

- 多会话标签、标签锁定与重命名、拖拽排序、分屏，以及会话窗口拆分与接回。
- 终端搜索、可配置的正则关键字高亮、命令历史、补全与分组快捷命令。
- 可配置的多行粘贴确认、批量命令、右键行为、字体、光标、滚动缓冲与渲染方式。
- 会话历史与备注、终端录制、回放和导出。
- 浅色 / 深色主题、终端配色、背景图和界面缩放。

### 文件与运维

- **SFTP**：本地与远端目录浏览、上传下载、拖放传输、传输队列、进度与取消、覆盖确认，以及目录跟随。
- **远程文本编辑**：在内置编辑器中打开远程文件，编辑后保存，并处理文件变更冲突。
- **端口转发**：本地转发、远程转发和动态 SOCKS 转发，支持分组及运行状态管理。
- **资源监控**：查看 CPU、内存、磁盘、网络等指标与历史曲线；可用指标取决于远端环境。
- **进程与服务**：查看进程，管理受支持系统上的服务并读取日志。
- **Docker**：通过当前 SSH 会话管理远端容器、镜像、卷和网络，查看资源详情及容器日志。
- **tmux 会话**：列出、新建和结束远端 tmux 会话；「连接」把 `tmux attach` 写进当前终端，断开连接不影响会话存活。
- **认证工具**：管理 TOTP / HOTP 条目，导入 `otpauth://` 或二维码、显示验证码、复制与展示二维码。

### 外部 CLI

桌面端会在本机启动一个只监听 `127.0.0.1` 的命令行服务，并把「端口 + 一次性 token」写进用户配置目录的 `cli.json`（权限 0600，退出时删除）。`qterm` 命令行读这个文件，用同一份资产与会话能力工作，适合被 Codex、Claude 这类外部工具调用。

```sh
go build -o bin/qterm ./cmd/qterm      # 构建客户端

qterm status                            # 服务是否可用、资产/会话计数
qterm hosts --query web                 # 按关键字查资产（不含任何凭据）
qterm sessions                          # 当前已打开的会话
qterm open <hostID>                     # 打开一个会话
qterm exec <sessionID> -- df -h         # 在会话上执行命令（仅 SSH）
qterm sftp ls <sessionID> /etc          # 列远端目录
qterm sftp read <sessionID> /etc/hosts  # 读远端文本
qterm shell <sessionID>                 # 交互式终端（vim/top 等全屏程序可用）
qterm close <sessionID>
```

- `--json` 让 stdout 只输出 JSON，便于程序解析；`--timeout` 覆盖默认 130 秒。
- 退出码：`0` 成功、`1` 用法错误、`2` 连接不到应用、`3` 握手文件缺失或过期、`4` 未授权、`5` 业务错误。
- 服务端限制：请求体 1 MiB、命令 8 KiB、SFTP 读 64 KiB / 写 256 KiB、目录 300 条；写入用 `expectedHash` 做乐观并发，冲突时不覆盖。
- 应用未运行、应用锁屏或握手文件被删除时，客户端会直接失败而不是排队等待。
- `qterm shell` 走一条**独立的** PTY 通道，不会顶掉应用里正在看的那个终端；`Ctrl-D` 或远端 `exit` 结束，窗口缩放会自动同步（Windows 无缩放事件）。

### AI 助手

在工作区内进行流式对话，结合选定的终端、主机及其他上下文分析问题，并通过 Agent 工具协助执行任务。

- 配置多个模型供应商、服务地址、模型与 API Key，支持连接预览。
- 支持 OpenAI Chat Completions、OpenAI Responses 和 Anthropic Messages 协议形态；兼容服务的具体能力取决于其实现。
- 支持会话历史、Markdown / 代码 / 数学公式展示，以及命令审批和 Agent 行为设置。
- Agent 命令按本地安全策略与审批配置执行；原生工具调用支持 OpenAI Chat Completions 和 Anthropic，Responses 使用兼容回退。

使用 AI 前需自行配置模型服务。选入上下文的内容会发送至所配置的服务，请按实际数据范围选择上下文。

## 页面与操作入口

| 页面 / 区域    | 可以做什么                                                                                            |
| -------------- | ----------------------------------------------------------------------------------------------------- |
| 资产库与侧边栏 | 搜索和组织主机、管理分组、打开连接、导入已有配置                                                      |
| 主机编辑器     | 按协议配置地址、认证、跳板 / 代理及高级连接参数                                                       |
| 会话工作区     | 使用终端或远程桌面，切换标签、分屏、拆分窗口                                                          |
| 右侧工具面板   | 打开资源监控、进程、服务、Docker、tmux 会话、SFTP、快捷命令、历史命令、端口转发、认证、主题与 AI 助手 |
| 设置 → 管理    | 凭据、指纹、代理、端口转发、快捷命令、历史会话、终端录制                                              |
| 设置 → 偏好    | 应用、外观、终端、远程桌面、关键字高亮、凭据提示、快捷键、AI 助手、关于                               |
| 设置 → 关于    | 查看版本与更新、导入导出等入口                                                                        |

工具面板会根据会话类型与连接状态开放；例如 Docker 需要已连接的 SSH 会话与远端 Docker 环境。

## 开始使用

1. 启动应用，按引导完成本地保险库初始化或解锁。
2. 新建分组与主机，选择协议并填写连接信息；也可以从已有客户端导入主机。
3. 为 SSH 主机配置密码、私钥或 SSH Agent；首次连接时核对并确认主机指纹。
4. 打开主机会话，在右侧选择所需工具；需要 AI 时，先到「设置 → AI 助手」配置模型服务。
5. 在设置中调整外观、终端、快捷键、录制及文件传输偏好。

### 配置迁移

提供 OpenSSH 配置、PuTTY、Xshell、Termius、SecureCRT、MobaXterm、WindTerm、FinalShell、Electerm 及 JSON 会话文件等导入入口，并支持 Qterm 资源备份的导入与导出。

导入时先扫描和预览，再选择需要的条目。不同来源可迁移的协议、凭据和高级选项不同，不能视为原客户端配置的完整复制；主密码或系统凭据访问也可能需要额外输入。

### 本地数据与凭据

主机、分组、历史等数据使用本地 SQLite 存储；秘密信息由加密保险库管理。保险库使用 AES-256-GCM，主密码保护使用 Argon2id 派生，并提供密钥调整和恢复包入口。SSH 首次连接需要确认主机密钥，已信任指纹可在设置中管理。

## 平台与使用边界

| 平台    | 分发目标              | 运行依赖 / 注意事项                                              |
| ------- | --------------------- | ---------------------------------------------------------------- |
| macOS   | Apple Silicon / Intel | 使用系统 WebView；本地便携包采用临时签名，正式签名与公证尚待完善 |
| Windows | amd64 / arm64         | 需要 WebView2 Runtime                                            |
| Linux   | amd64 / arm64         | 需要与构建目标匹配的 GTK / WebKit 运行库                         |

- **RDP 依赖额外 helper**：使用基于 FreeRDP 的 helper 及其动态库。六平台本地打包入口仅在 macOS helper 与目标架构匹配时携带它；Windows / Linux 不自动携带。缺少对应 helper 时无法直接使用 RDP。
- **远端工具依赖环境**：SFTP 需要服务器提供子系统，Docker 需要远端 CLI 与权限，服务管理和监控受操作系统及已安装工具限制。
- **浏览器预览仅用于界面开发**：网页中的内存网关不能替代原生 SSH、SFTP、远程桌面或保险库能力。
- **自动更新已接入**：桌面端可检查 GitHub Release，后台下载并校验 SHA-256，再由用户确认重启安装；就绪后提供顶栏提醒。真实 Release 全流程、Windows / Linux 替换重启及发布 CI 仍需实机验收。
- **跨平台验收仍在推进**：构建目标与已有实现不代表所有功能已经在六种系统 / 架构组合上验证完毕。

## 本地开发

### 环境准备

| 工具       | 当前工程基线                                                         |
| ---------- | -------------------------------------------------------------------- |
| Go         | 1.26.0 或与 `go.mod` 兼容的版本                                      |
| Node.js    | 22.22.2+ 的 22.x，或 24.15.0+ 的 24.x；需满足锁定依赖的 engines 要求 |
| npm        | 安装锁定的前端依赖                                                   |
| Wails CLI  | v3.1.0，与 Go 模块版本保持一致                                       |
| 平台工具链 | macOS 需要 Xcode Command Line Tools；其他平台需准备对应原生构建依赖  |

在仓库根目录执行：

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
npm --prefix frontend ci
wails3 doctor
wails3 task dev
```

确保 Go 的可执行文件目录已加入 `PATH`。`wails3 task dev` 使用 `build/config.yml`，默认前端端口为 `9245`。

仅预览前端（浏览器打开 `http://127.0.0.1:9245`，此时走内存网关，没有真实 SSH / SFTP 能力）：

```sh
npm --prefix frontend run dev
```

### 检查与构建

```sh
# 完整本地检查：Go 格式、测试、vet；前端格式、lint、类型、单测、构建与 E2E
wails3 task check

# 构建当前平台应用
wails3 build

# 生成当前平台安装 / 分发产物
wails3 package
```

完整检查需要仓库中的测试文件和 Playwright 浏览器环境；纯代码快照不包含完整测试门禁。需要时安装 E2E 浏览器：

```sh
cd frontend
npx playwright install chromium
```

### 六平台便携包

在 **macOS** 仓库根目录执行：

```sh
# 只输出计划
wails3 task release:all DRY_RUN=true

# 实际构建
wails3 task release:all
```

需要 Go、Node.js/npm、Wails CLI、Xcode Command Line Tools 和已启动的 Docker Desktop。首次运行会构建 Linux 交叉编译镜像，需要联网。

产物写入 `bin/releases/` 下的新目录，包含 macOS / Windows ZIP、Linux tar.gz，以及全部成功后生成的 `SHA256SUMS`。该命令不自动上传，也不生成 DMG、NSIS、deb 或 rpm。

仓库另提供 GitHub Actions Release 工作流，按版本 tag 构建六种目标并汇总校验文件；工作流的真实 runner 验收仍待完成。

### 提交规范

提交信息使用 Conventional Commits（`<type>(<scope>): <subject>`），由 commitlint 校验、czg 生成：

```sh
npm install        # 首次：安装 czg / commitlint / husky，并挂上 commit-msg 钩子
npm run commit     # 交互式生成提交信息
```

- **type**：`feat`、`fix`、`docs`、`style`、`refactor`、`perf`、`test`、`build`、`ci`、`chore`、`revert`、`wip`、`workflow`、`types`、`release`。
- **scope**：`internal/*` 与 `frontend/src/features/*` 的目录名自动成为可选 scope，另有 `project`、`deps`、`build`、`ci`、`release`、`docs`、`scripts`、`i18n`、`ui`、`other`；可以省略。
- **header 最多 108 字符**，subject 不能为空；不满足规则时提交会被拒绝。

规则细节见 `docs/development/commit-convention.md`（该目录只在内部仓保留）。

## 技术与目录

| 部分           | 技术                                                         |
| -------------- | ------------------------------------------------------------ |
| 桌面容器       | Wails v3.1.0、Go                                             |
| 前端           | Vue 3、TypeScript、Vite、Tailwind CSS、Reka UI               |
| 终端与远程桌面 | xterm.js、noVNC、FreeRDP helper                              |
| 编辑器         | CodeMirror、Monaco Editor                                    |
| 存储与加密     | SQLite、AES-256-GCM、Argon2id                                |
| 质量检查       | Go test / vet、Vitest、Playwright、ESLint、Prettier、vue-tsc |

```text
main.go                  应用入口与资源嵌入
internal/app/            Wails Facade、装配、窗口与运行时桥接
internal/                主机、会话、终端、SFTP、AI 等功能与基础设施
frontend/src/app/        应用外壳与窗口入口
frontend/src/features/   按功能组织的页面、面板和状态逻辑
frontend/src/services/   前端网关与运行时服务封装
frontend/src/components/ 公共控件、布局、对话框与工作区组件
frontend/src/locales/    七种语言的界面文案
frontend/bindings/       生成的 Wails TypeScript 绑定
build/                   平台构建配置与发布脚本
tools/                   RDP helper 源码、OTP 本地测试夹具
Taskfile.yml             开发、验证与构建任务入口
```

更细的目录职责见 [`internal/README.md`](internal/README.md) 与 [`frontend/README.md`](frontend/README.md)。
