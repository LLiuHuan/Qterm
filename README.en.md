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

## Installation

Installers are published in [GitHub Releases](https://github.com/LLiuHuan/Qterm/releases). Expand the **Assets** section for the version you want and download the file matching your operating system and processor architecture; `Source code` archives are source snapshots, not installers.

| Device                                       | Architecture |
| -------------------------------------------- | ------------ |
| Apple Silicon Mac (M-series)                 | `arm64`      |
| Intel Mac                                    | `amd64`      |
| 64-bit Intel / AMD Windows or Linux computer | `amd64`      |
| ARM64 Windows computer or Linux device       | `arm64`      |

On Windows, check the system type in Settings → System → About; on macOS, check the chip in About This Mac; on Linux, run `uname -m` (`x86_64` means `amd64`, and `aarch64` means `arm64`). 32-bit packages are not provided.

### macOS

Requires macOS 12 Monterey or later.

**Install with Homebrew**

Install [Homebrew](https://brew.sh/), then add the Qterm tap and install the app:

```sh
brew tap lliuhuan/tap
brew install --cask lliuhuan/tap/qterm
```

Homebrew selects the arm64 or amd64 package for your Mac automatically. After installation, open Qterm from Applications or run:

```sh
open /Applications/qterm.app
```

Upgrade to a newer version:

```sh
brew update
brew upgrade --cask lliuhuan/tap/qterm
```

- The current macOS package uses an ad-hoc signature and is not notarized by Apple. If macOS blocks the first launch, allow Qterm in System Settings → Privacy & Security.
- If Qterm was installed manually, Homebrew may report that the app already exists. Quit Qterm, move the existing `/Applications/qterm.app` to another directory with Finder if you want to keep it, then install; you do not need to delete Qterm user data.

**Manual installation**

1. Download `qterm-darwin-arm64.dmg` for Apple Silicon or `qterm-darwin-amd64.dmg` for Intel.
2. Open the DMG and drag `qterm.app` to Applications.
3. Launch it from Applications; follow the first-launch security instructions above if prompted.

You can also download the ZIP for the same architecture, extract it, and move `qterm.app` to Applications. To upgrade manually, quit Qterm and replace the old app with the new one.

### Windows

**Installer**

1. Download `qterm-windows-amd64-setup.exe`; choose `qterm-windows-arm64-setup.exe` on an ARM64 device.
2. Run it and follow the installer, choosing an installation directory if needed; approve the administrator prompt when shown.
3. Start Qterm from the Start menu or desktop shortcut.

Qterm requires the **Microsoft Edge WebView2 Runtime**. The installer checks for it and attempts to install it when missing. If that fails, or if you need an offline installation, download the Evergreen Standalone Installer from the [official Microsoft WebView2 page](https://developer.microsoft.com/en-us/microsoft-edge/webview2/), choose x64 or ARM64, install it, and then start Qterm.

**ZIP package**

Download `qterm-windows-amd64.zip` or `qterm-windows-arm64.zip`, extract the complete archive to a writable directory, and run `qterm.exe` from there. The ZIP package also requires WebView2 Runtime; do not run the executable from the archive preview.

Quit Qterm before upgrading with the installer. For the ZIP package, extract the new version and replace the old program.

### Linux

Linux requires a graphical desktop session and GTK / WebKitGTK runtime libraries matching the package. The current packages declare **GTK 4 and WebKitGTK 6.0**; older distributions may not provide compatible versions, so matching the CPU architecture alone does not guarantee compatibility.

The examples below use `amd64`; replace `amd64` with `arm64` on an ARM64 device. Download the release file first, then run the corresponding command in its download directory.

**Debian / Ubuntu: DEB package**

Download `qterm-linux-amd64.deb` and install it through APT so dependencies are resolved:

```sh
sudo apt update
sudo apt install ./qterm-linux-amd64.deb
```

Launch it from the application menu or run `qterm` in a terminal. Download the new DEB and repeat the same install command to upgrade.

**Fedora / RHEL family: RPM package**

On a distribution that provides the required runtime libraries, download `qterm-linux-amd64.rpm` and install it:

```sh
sudo dnf install ./qterm-linux-amd64.rpm
```

Launch it from the application menu or run `qterm`. To upgrade:

```sh
sudo dnf upgrade ./qterm-linux-amd64.rpm
```

If the package manager reports missing dependencies, check that your distribution repositories provide the required versions; do not force installation while ignoring dependencies. Older CentOS / RHEL versions are not guaranteed to work.

**AppImage**

Download `qterm-linux-amd64.AppImage`, make it executable, and launch it:

```sh
chmod +x qterm-linux-amd64.AppImage
./qterm-linux-amd64.AppImage
```

If FUSE is missing, install the FUSE runtime required by your distribution or use the DEB / RPM package instead. AppImage compatibility with arbitrarily old distributions is not guaranteed. Quit Qterm before replacing it with a newer file.

**tar.gz package**

Download `qterm-linux-amd64.tar.gz` and extract it:

```sh
tar -xzf qterm-linux-amd64.tar.gz
chmod +x qterm
./qterm
```

This archive only provides the program; it does not install system libraries or an application-menu entry. To upgrade, quit Qterm and replace the extracted program with the new version.

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
