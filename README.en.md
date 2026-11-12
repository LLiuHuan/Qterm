<p align="center">
  <img src="frontend/public/qterm-icon.png" width="96" alt="Qterm icon" />
</p>

<h1 align="center">Qterm</h1>

<p align="center">Terminal, file transfer, remote desktop, and day-to-day operations in one workspace.</p>

<p align="center">
  <a href="README.md">简体中文</a> ·
  <a href="README.zh-TW.md">繁體中文</a> ·
  <strong>English</strong> ·
  <a href="README.ja.md">日本語</a> ·
  <a href="README.ko.md">한국어</a> ·
  <a href="README.fr.md">Français</a> ·
  <a href="README.ru.md">Русский</a>
</p>

Qterm is a desktop connection management tool built with **Go, Wails v3, Vue 3, and TypeScript** for macOS, Windows, and Linux. It starts from host assets and connects to servers and devices in a multi-tab, split-screen workspace, with a side tool panel for handling files, monitoring resources, managing containers, and using the AI assistant.

The interface is available in seven languages: Simplified Chinese, Traditional Chinese, English, French, Japanese, Korean, and Russian.

## Screenshots

**Terminal, SFTP, and context menu tools**

![Terminal, SFTP, and context menu tools](public/img_3.png)

<details>
<summary>Split panes and a custom background</summary>

![Split panes and a custom background](public/img_4.png)

</details>

<details>
<summary>Light and dark themes</summary>

**Light theme home screen**

![Light theme home screen](public/img.png)

**Dark theme home screen**

![Dark theme home screen](public/img_2.png)

</details>

<details>
<summary>Application settings</summary>

![Application settings](public/img_1.png)

</details>

## Feature overview

### Hosts and connections

- **Unified asset library**: manage hosts and nested groups, with search, tags, notes, custom icons and colors, plus sidebar and full-screen asset library views.
- **Multiple connection types**: SSH, Telnet, serial, RDP, and VNC, and you can open a local shell.
- **SSH authentication**: password, private key, SSH Agent, interactive authentication, and two-factor authentication configuration linked to OTP entries.
- **Complex network access**: SSH jump host chains and SOCKS5 / HTTP proxies; proxy configurations are reusable, and RDP and VNC can also use jump host or proxy chains.
- **Connection options**: per-protocol terminal, encoding, keepalive, command after login, SFTP, and remote desktop parameters.

### Terminal workspace

- Multiple session tabs, tab locking and renaming, drag-to-reorder, split panes, and detaching and re-attaching session windows.
- Terminal search, configurable regular-expression keyword highlighting, command history, completion, and grouped snippets.
- Configurable multi-line paste confirmation, bulk commands, right-click behavior, font, cursor, scrollback buffer, and rendering mode.
- Session history and notes, terminal recording, replay, and export.
- Light / dark themes, terminal color schemes, background images, and interface scaling.

### Files and operations

- **SFTP**: local and remote directory browsing, upload and download, drag-and-drop transfer, transfer queue, progress and cancellation, overwrite confirmation, and directory following.
- **Remote text editing**: open remote files in the built-in editor, save after editing, and handle file change conflicts.
- **Port forwarding**: local, remote, and dynamic SOCKS forwarding, with groups and running-state management.
- **Resource monitoring**: view CPU, memory, disk, network, and other metrics with history charts; available metrics depend on the remote environment.
- **Processes and services**: view processes, manage services on supported systems, and read logs.
- **Docker**: manage remote containers, images, volumes, and networks through the current SSH session, and view resource details and container logs.
- **tmux sessions**: list, create, and kill tmux sessions on the remote host; “Attach” writes `tmux attach` into the current terminal, and sessions survive disconnections.
- **Authentication tools**: manage TOTP / HOTP entries, import `otpauth://` or QR codes, display verification codes, and copy and show QR codes.

### AI assistant

Chat within the workspace with streaming responses, analyze problems with the selected terminal, host, and other context, and use Agent tools to help carry out tasks.

- Configure multiple model providers, service endpoints, models, and API keys, with connection previews.
- Support the OpenAI Chat Completions, OpenAI Responses, and Anthropic Messages protocol shapes; the exact capabilities of compatible services depend on their implementation.
- Support session history, Markdown / code / math formula rendering, and command approval and Agent behavior settings.
- Agent commands run according to the local security policy and approval settings; native tool calling supports OpenAI Chat Completions and Anthropic, while Responses uses a compatibility fallback.

You must configure a model service yourself before using AI. Content selected as context is sent to the configured service, so choose context according to the actual data scope.

## Pages and entry points

| Page / area               | What you can do                                                                                                                                                      |
| ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Asset library and sidebar | Search and organize hosts, manage groups, open connections, and import existing configurations                                                                       |
| Host editor               | Configure address, authentication, jump host / proxy, and advanced connection parameters per protocol                                                                |
| Session workspace         | Use the terminal or remote desktop, switch tabs, split panes, and detach windows                                                                                     |
| Right-side tool panel     | Open resource monitoring, processes, services, Docker, tmux sessions, SFTP, snippets, command history, port forwarding, authentication, themes, and the AI assistant |
| Settings → Manage         | Credentials, fingerprints, proxies, port forwarding, snippets, session history, terminal recordings                                                                  |
| Settings → Preferences    | Application, appearance, terminal, remote desktop, keyword highlighting, credential prompts, shortcuts, AI assistant, about                                          |
| Settings → About          | View version and updates, import and export, and other entry points                                                                                                  |

The tool panel opens according to the session type and connection state; for example, Docker requires a connected SSH session and a remote Docker environment.

## Getting started

1. Launch the app and complete local vault initialization or unlock it as prompted.
2. Create groups and hosts, choose a protocol, and fill in the connection information; you can also import hosts from an existing client.
3. Configure a password, private key, or SSH Agent for SSH hosts; verify and confirm the host fingerprint on the first connection.
4. Open a host session and choose the tools you need on the right; to use AI, first configure a model service in Settings → AI assistant.
5. Adjust appearance, terminal, shortcuts, recording, and file transfer preferences in settings.

### Configuration migration

Import entry points are provided for OpenSSH configuration, PuTTY, Xshell, Termius, SecureCRT, MobaXterm, WindTerm, FinalShell, Electerm, and JSON session files, along with importing and exporting Qterm asset backups.

Imports are scanned and previewed first, then you choose the entries you need. The protocols, credentials, and advanced options that can be migrated differ by source and are not a complete copy of the original client configuration; a master password or system credential access may also require additional input.

### Local data and credentials

Hosts, groups, history, and other data are stored in local SQLite; secrets are managed by an encrypted vault. The vault uses AES-256-GCM, master password protection uses Argon2id derivation, and key adjustment and recovery package entry points are provided. The first SSH connection requires confirming the host key, and trusted fingerprints can be managed in settings.

## Platforms and limits

| Platform | Distribution targets  | Runtime dependencies / notes                                                                                              |
| -------- | --------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| macOS    | Apple Silicon / Intel | Uses the system WebView; local portable packages are ad-hoc signed, and formal signing and notarization are still pending |
| Windows  | amd64 / arm64         | Requires the WebView2 Runtime                                                                                             |
| Linux    | amd64 / arm64         | Requires GTK / WebKit runtime libraries that match the build target                                                       |

- **RDP requires an extra helper**: it uses a FreeRDP-based helper and its dynamic libraries. The six-platform local packaging entry point ships it only when the macOS helper matches the target architecture; Windows / Linux do not ship it automatically. Without the matching helper, RDP cannot be used directly.
- **Remote tools depend on the environment**: SFTP requires the server to provide a subsystem, Docker requires the remote CLI and permissions, and service management and monitoring are limited by the operating system and the installed tools.
- **The browser preview is for interface development only**: the in-memory gateway in the web page cannot replace native SSH, SFTP, remote desktop, or vault capabilities.
- **Automatic updates are wired up**: the desktop app can check GitHub Releases, download in the background and verify SHA-256, and then install after the user confirms a restart; a reminder appears in the top bar once it is ready. The full end-to-end Release flow, replacement and restart on Windows / Linux, and the release CI still need real-device verification.
- **Cross-platform verification is still in progress**: build targets and existing implementations do not mean every feature has been verified on all six system / architecture combinations.

## Local development

### Environment setup

| Tool               | Current project baseline                                                                                  |
| ------------------ | --------------------------------------------------------------------------------------------------------- |
| Go                 | 1.26.0 or a version compatible with `go.mod`                                                              |
| Node.js            | 22.x at 22.22.2+ or 24.x at 24.15.0+; must satisfy the engines requirements of locked dependencies        |
| npm                | Install the locked frontend dependencies                                                                  |
| Wails CLI          | v3.1.0, matching the Go module version                                                                    |
| Platform toolchain | macOS requires Xcode Command Line Tools; other platforms need the corresponding native build dependencies |

Run from the repository root:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
npm --prefix frontend ci
wails3 doctor
wails3 task dev
```

Make sure the Go binary directory has been added to `PATH`. `wails3 task dev` uses `build/config.yml`, and the default frontend port is `9245`.

Preview the frontend only (open `http://127.0.0.1:9245` in a browser; this uses the in-memory gateway and has no real SSH / SFTP capabilities):

```sh
npm --prefix frontend run dev
```

### Checks and building

```sh
# Full local check: Go format, tests, vet; frontend format, lint, types, unit tests, build, and E2E
wails3 task check

# Build the app for the current platform
wails3 build

# Generate install / distribution artifacts for the current platform
wails3 package
```

The full check requires the tests in the repository and a Playwright browser environment; a pure code snapshot does not include the complete test gate. Install the E2E browser when needed:

```sh
cd frontend
npx playwright install chromium
```

### Six-platform portable packages

Run from the repository root on **macOS**:

```sh
# Print the plan only
wails3 task release:all DRY_RUN=true

# Build for real
wails3 task release:all
```

This requires Go, Node.js/npm, the Wails CLI, Xcode Command Line Tools, and a running Docker Desktop. The first run builds the Linux cross-compilation image and needs network access.

Artifacts are written to a new directory under `bin/releases/`, including macOS / Windows ZIPs, Linux tar.gz files, and a `SHA256SUMS` file generated after everything succeeds. The command does not upload anything, and does not produce DMG, NSIS, deb, or rpm.

The repository also provides a GitHub Actions Release workflow that builds the six targets from a version tag and aggregates the checksum file; verification on real runners is still pending.

### Commit convention

Commit messages use Conventional Commits (`<type>(<scope>): <subject>`), validated by commitlint and generated by czg:

```sh
npm install        # First run: install czg / commitlint / husky and hook commit-msg
npm run commit     # Interactively generate a commit message
```

- **type**: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`, `wip`, `workflow`, `types`, `release`.
- **scope**: directory names under `internal/*` and `frontend/src/features/*` automatically become available scopes, along with `project`, `deps`, `build`, `ci`, `release`, `docs`, `scripts`, `i18n`, `ui`, and `other`; it can be omitted.
- **header is at most 108 characters**, and subject cannot be empty; commits that break the rules are rejected.

See `docs/development/commit-convention.md` for the full rules (that directory is kept in the internal repository only).

## Technology and layout

| Part                        | Technology                                                   |
| --------------------------- | ------------------------------------------------------------ |
| Desktop shell               | Wails v3.1.0, Go                                             |
| Frontend                    | Vue 3, TypeScript, Vite, Tailwind CSS, Reka UI               |
| Terminal and remote desktop | xterm.js, noVNC, FreeRDP helper                              |
| Editor                      | CodeMirror, Monaco Editor                                    |
| Storage and encryption      | SQLite, AES-256-GCM, Argon2id                                |
| Quality checks              | Go test / vet, Vitest, Playwright, ESLint, Prettier, vue-tsc |

```text
main.go                  App entry and resource embedding
internal/app/            Wails facade, assembly, window and runtime bridging
internal/                Hosts, sessions, terminal, SFTP, AI, and other features and infrastructure
frontend/src/app/        App shell and window entry
frontend/src/features/   Pages, panels, and state logic organized by feature
frontend/src/services/   Frontend gateway and runtime service wrappers
frontend/src/components/ Shared controls, layout, dialogs, and workspace components
frontend/src/locales/    UI copy for seven languages
frontend/bindings/       Generated Wails TypeScript bindings
build/                   Platform build configuration and release scripts
tools/                   RDP helper source, OTP local test fixtures
Taskfile.yml             Entry points for development, verification, and build tasks
```

See [`internal/README.md`](internal/README.md) and [`frontend/README.md`](frontend/README.md) for more detailed directory responsibilities.
