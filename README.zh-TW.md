<p align="center">
  <img src="frontend/public/qterm-icon.png" width="96" alt="Qterm 圖示" />
</p>

<h1 align="center">Qterm</h1>

<p align="center">把終端、檔案傳輸、遠端桌面與日常維運放進同一個工作台。</p>

<p align="center">
  <a href="README.md">简体中文</a> ·
  <strong>繁體中文</strong> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a> ·
  <a href="README.ko.md">한국어</a> ·
  <a href="README.fr.md">Français</a> ·
  <a href="README.ru.md">Русский</a>
</p>

Qterm 是以 **Go、Wails v3、Vue 3 與 TypeScript** 打造的桌面連線管理工具，支援 macOS、Windows 與 Linux。以主機資產庫為入口，在多標籤、分屏工作區中連線伺服器與裝置，並透過側邊工具面板處理檔案、監控資源、管理容器及使用 AI 助手。

支援簡體中文、繁體中文、English、Français、日本語、한국어 與 Русский 七種介面語言。

## 介面預覽

**終端、SFTP 與右鍵工具**

![終端、SFTP 與右鍵工具](public/img_3.png)

<details>
<summary>分割工作區與自訂背景</summary>

![分割工作區與自訂背景](public/img_4.png)

</details>

<details>
<summary>淺色與深色介面</summary>

**淺色首頁**

![淺色首頁](public/img.png)

**深色首頁**

![深色首頁](public/img_2.png)

</details>

<details>
<summary>應用程式設定</summary>

![應用程式設定](public/img_1.png)

</details>

## 功能總覽

### 主機與連線

- **統一資產庫**：管理主機與巢狀分組，支援搜尋、標籤、備註、自訂圖示與顏色，以及側邊欄與全螢幕資產檢視。
- **多種連線方式**：SSH、Telnet、串列、RDP、VNC，並可開啟本機 Shell。
- **SSH 認證**：密碼、私鑰、SSH Agent、互動式認證，以及關聯 OTP 條目的雙因素驗證設定。
- **複雜網路存取**：SSH 跳板鏈、SOCKS5 / HTTP 代理；代理設定可重複使用，RDP 與 VNC 也能設定跳板或代理鏈路。
- **連線選項**：依協定設定終端、編碼、保持連線、登入後命令、SFTP 與遠端桌面參數。

### 終端工作區

- 多會話標籤、標籤鎖定與重新命名、拖曳排序、分屏，以及會話視窗拆分與接回。
- 終端搜尋、可設定的正則關鍵字高亮、命令歷史、補全與分組快捷命令。
- 可設定的多行貼上確認、批次命令、右鍵行為、字型、游標、回滾緩衝與渲染方式。
- 會話歷史與備註、終端錄製、回放與匯出。
- 淺色 / 深色主題、終端配色、背景圖與介面縮放。

### 檔案與維運

- **SFTP**：本機與遠端目錄瀏覽、上傳下載、拖放傳輸、傳輸佇列、進度與取消、覆寫確認，以及目錄跟隨。
- **遠端文字編輯**：在內建編輯器中開啟遠端檔案，編輯後儲存，並處理檔案變更衝突。
- **連接埠轉送**：本機轉送、遠端轉送與動態 SOCKS 轉送，支援分組及執行狀態管理。
- **資源監控**：檢視 CPU、記憶體、磁碟、網路等指標與歷史曲線；可用指標取決於遠端環境。
- **行程與服務**：檢視行程，管理受支援系統上的服務並讀取日誌。
- **Docker**：透過目前 SSH 會話管理遠端容器、鏡像、磁碟區與網路，檢視資源詳情及容器日誌。
- **認證工具**：管理 TOTP / HOTP 條目，匯入 `otpauth://` 或 QR 碼、顯示驗證碼、複製與展示 QR 碼。

### AI 助手

在工作區內進行串流對話，結合選定的終端、主機及其他上下文分析問題，並透過 Agent 工具協助執行任務。

- 設定多個模型供應商、服務位址、模型與 API Key，支援連線預覽。
- 支援 OpenAI Chat Completions、OpenAI Responses 與 Anthropic Messages 協定形態；相容服務的具體能力取決於其實作。
- 支援會話歷史、Markdown / 程式碼 / 數學公式展示，以及命令審批與 Agent 行為設定。
- Agent 命令依本機安全策略與審批設定執行；原生工具呼叫支援 OpenAI Chat Completions 與 Anthropic，Responses 使用相容回退。

使用 AI 前需自行設定模型服務。選入上下文的內容會傳送至所設定的服務，請依實際資料範圍選擇上下文。

## 頁面與操作入口

| 頁面 / 區域    | 可以做什麼                                                                                   |
| -------------- | -------------------------------------------------------------------------------------------- |
| 資產庫與側邊欄 | 搜尋與組織主機、管理分組、開啟連線、匯入既有設定                                             |
| 主機編輯器     | 依協定設定位址、認證、跳板 / 代理及進階連線參數                                              |
| 會話工作區     | 使用終端或遠端桌面，切換標籤、分屏、拆分視窗                                                 |
| 右側工具面板   | 開啟資源監控、行程、服務、Docker、SFTP、快捷命令、歷史命令、連接埠轉送、認證、主題與 AI 助手 |
| 設定 → 管理    | 憑據、指紋、代理、連接埠轉送、快捷命令、歷史會話、終端錄製                                   |
| 設定 → 偏好    | 應用、外觀、終端、遠端桌面、關鍵字高亮、憑據提示、快捷鍵、AI 助手、關於                      |
| 設定 → 關於    | 檢視版本與更新、匯入匯出等入口                                                               |

工具面板會依會話類型與連線狀態開放；例如 Docker 需要已連線的 SSH 會話與遠端 Docker 環境。

## 開始使用

1. 啟動應用程式，依引導完成本機保險庫初始化或解鎖。
2. 新增分組與主機，選擇協定並填寫連線資訊；也可以從既有用戶端匯入主機。
3. 為 SSH 主機設定密碼、私鑰或 SSH Agent；首次連線時核對並確認主機指紋。
4. 開啟主機會話，在右側選擇所需工具；需要 AI 時，先到「設定 → AI 助手」設定模型服務。
5. 在設定中調整外觀、終端、快捷鍵、錄製及檔案傳輸偏好。

### 設定遷移

提供 OpenSSH 設定、PuTTY、Xshell、Termius、SecureCRT、MobaXterm、WindTerm、FinalShell、Electerm 及 JSON 會話檔案等匯入入口，並支援 Qterm 資源備份的匯入與匯出。

匯入時先掃描與預覽，再選擇需要的條目。不同來源可遷移的協定、憑據與進階選項不同，不能視為原用戶端設定的完整複製；主密碼或系統憑據存取也可能需要額外輸入。

### 本機資料與憑據

主機、分組、歷史等資料使用本機 SQLite 儲存；機密資訊由加密保險庫管理。保險庫使用 AES-256-GCM，主密碼保護使用 Argon2id 派生，並提供金鑰調整與恢復包入口。SSH 首次連線需要確認主機金鑰，已信任的指紋可在設定中管理。

## 平台與使用邊界

| 平台    | 發佈目標              | 執行相依 / 注意事項                                                  |
| ------- | --------------------- | -------------------------------------------------------------------- |
| macOS   | Apple Silicon / Intel | 使用系統 WebView；本機可攜式套件採用臨時簽章，正式簽章與公證尚待完善 |
| Windows | amd64 / arm64         | 需要 WebView2 Runtime                                                |
| Linux   | amd64 / arm64         | 需要與建置目標相符的 GTK / WebKit 執行階段程式庫                     |

- **RDP 需要額外 helper**：使用以 FreeRDP 為基礎的 helper 及其動態程式庫。六平台本機打包入口僅在 macOS helper 與目標架構相符時一併包含；Windows / Linux 不會自動包含。缺少對應 helper 時無法直接使用 RDP。
- **遠端工具需要環境支援**：SFTP 需要伺服器提供子系統，Docker 需要遠端 CLI 與權限，服務管理與監控受作業系統及已安裝工具限制。
- **瀏覽器預覽僅供介面開發**：網頁中的記憶體閘道不能取代原生 SSH、SFTP、遠端桌面或保險庫能力。
- **自動更新已接入**：桌面端可檢查 GitHub Release，於背景下載並校驗 SHA-256，再由使用者確認重新啟動安裝；就緒後在頂部列提供提醒。真實 Release 全流程、Windows / Linux 替換重新啟動及發佈 CI 仍需實機驗收。
- **跨平台驗收仍在推進**：建置目標與既有實作不代表所有功能都已在六種系統 / 架構組合上驗證完畢。

## 本機開發

### 環境準備

| 工具       | 目前工程基線                                                             |
| ---------- | ------------------------------------------------------------------------ |
| Go         | 1.26.0 或與 `go.mod` 相容的版本                                          |
| Node.js    | 22.22.2+ 的 22.x，或 24.15.0+ 的 24.x；需符合鎖定相依套件的 engines 要求 |
| npm        | 安裝鎖定的前端相依套件                                                   |
| Wails CLI  | v3.1.0，與 Go 模組版本保持一致                                           |
| 平台工具鏈 | macOS 需要 Xcode Command Line Tools；其他平台需準備對應的原生建置相依    |

在倉庫根目錄執行：

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
npm --prefix frontend ci
wails3 doctor
wails3 task dev
```

確認 Go 的可執行檔目錄已加入 `PATH`。`wails3 task dev` 使用 `build/config.yml`，預設前端連接埠為 `9245`。

僅預覽前端（瀏覽器開啟 `http://127.0.0.1:9245`，此時走記憶體閘道，沒有真實 SSH / SFTP 能力）：

```sh
npm --prefix frontend run dev
```

### 檢查與建置

```sh
# 完整本機檢查：Go 格式、測試、vet；前端格式、lint、型別、單元測試、建置與 E2E
wails3 task check

# 建置目前平台應用程式
wails3 build

# 產生目前平台安裝 / 發佈產物
wails3 package
```

完整檢查需要倉庫中的測試檔案與 Playwright 瀏覽器環境；純程式碼快照不包含完整測試閘門。需要時安裝 E2E 瀏覽器：

```sh
cd frontend
npx playwright install chromium
```

### 六平台可攜式套件

在 **macOS** 倉庫根目錄執行：

```sh
# 僅輸出計畫
wails3 task release:all DRY_RUN=true

# 實際建置
wails3 task release:all
```

需要 Go、Node.js/npm、Wails CLI、Xcode Command Line Tools 與已啟動的 Docker Desktop。首次執行會建置 Linux 交叉編譯鏡像，需要網路連線。

產物寫入 `bin/releases/` 下的新目錄，包含 macOS / Windows ZIP、Linux tar.gz，以及全部成功後產生的 `SHA256SUMS`。此命令不會自動上傳，也不會產生 DMG、NSIS、deb 或 rpm。

倉庫另提供 GitHub Actions Release 工作流程，依版本 tag 建置六種目標並彙整校驗檔；工作流程的真實 runner 驗收仍待完成。

### 提交規範

提交訊息使用 Conventional Commits（`<type>(<scope>): <subject>`），由 commitlint 校驗、czg 產生：

```sh
npm install        # 首次：安裝 czg / commitlint / husky，並掛上 commit-msg 掛鉤
npm run commit     # 互動式產生提交訊息
```

- **type**：`feat`、`fix`、`docs`、`style`、`refactor`、`perf`、`test`、`build`、`ci`、`chore`、`revert`、`wip`、`workflow`、`types`、`release`。
- **scope**：`internal/*` 與 `frontend/src/features/*` 的目錄名會自動成為可選 scope，另有 `project`、`deps`、`build`、`ci`、`release`、`docs`、`scripts`、`i18n`、`ui`、`other`；可以省略。
- **header 最多 108 個字元**，subject 不能為空；不符合規則時提交會被拒絕。

規則細節見 `docs/development/commit-convention.md`（該目錄只在內部倉保留）。

## 技術與目錄

| 部分           | 技術                                                         |
| -------------- | ------------------------------------------------------------ |
| 桌面容器       | Wails v3.1.0、Go                                             |
| 前端           | Vue 3、TypeScript、Vite、Tailwind CSS、Reka UI               |
| 終端與遠端桌面 | xterm.js、noVNC、FreeRDP helper                              |
| 編輯器         | CodeMirror、Monaco Editor                                    |
| 儲存與加密     | SQLite、AES-256-GCM、Argon2id                                |
| 品質檢查       | Go test / vet、Vitest、Playwright、ESLint、Prettier、vue-tsc |

```text
main.go                  應用程式入口與資源嵌入
internal/app/            Wails Facade、裝配、視窗與執行階段橋接
internal/                主機、會話、終端、SFTP、AI 等功能與基礎設施
frontend/src/app/        應用程式外殼與視窗入口
frontend/src/features/   依功能組織的頁面、面板與狀態邏輯
frontend/src/services/   前端閘道與執行階段服務封裝
frontend/src/components/ 共用控制項、版面、對話框與工作區元件
frontend/src/locales/    七種語言的介面文案
frontend/bindings/       產生的 Wails TypeScript 繫結
build/                   平台建置設定與發佈腳本
tools/                   RDP helper 原始碼、OTP 本機測試夾具
Taskfile.yml             開發、驗證與建置任務入口
```

更細的目錄職責見 [`internal/README.md`](internal/README.md) 與 [`frontend/README.md`](frontend/README.md)。
