<p align="center">
  <img src="frontend/public/qterm-icon.png" width="96" alt="Qterm 아이콘" />
</p>

<h1 align="center">Qterm</h1>

<p align="center">터미널, 파일 전송, 원격 데스크톱과 일상적인 운영 작업을 하나의 작업 공간에 담았습니다.</p>

<p align="center">
  <a href="README.md">简体中文</a> ·
  <a href="README.zh-TW.md">繁體中文</a> ·
  <a href="README.en.md">English</a> ·
  <a href="README.ja.md">日本語</a> ·
  <strong>한국어</strong> ·
  <a href="README.fr.md">Français</a> ·
  <a href="README.ru.md">Русский</a>
</p>

Qterm은 **Go, Wails v3, Vue 3, TypeScript**로 만든 데스크톱 연결 관리 도구이며 macOS, Windows, Linux를 지원합니다. 호스트 자산을 시작점으로 여러 탭과 분할 창이 있는 작업 공간에서 서버와 장치에 연결하고, 오른쪽 도구 패널에서 파일 작업, 리소스 모니터링, 컨테이너 관리와 AI 어시스턴트 사용을 처리합니다.

简体中文, 繁體中文, English, Français, 日本語, 한국어, Русский 7개 언어의 인터페이스를 지원합니다.

## 스크린샷

**터미널, SFTP 및 우클릭 메뉴**

![터미널, SFTP 및 우클릭 메뉴](public/img_3.png)

<details>
<summary>분할 작업 공간과 사용자 지정 배경</summary>

![분할 작업 공간과 사용자 지정 배경](public/img_4.png)

</details>

<details>
<summary>밝은 테마와 어두운 테마</summary>

**밝은 테마의 홈 화면**

![밝은 테마의 홈 화면](public/img.png)

**어두운 테마의 홈 화면**

![어두운 테마의 홈 화면](public/img_2.png)

</details>

<details>
<summary>앱 설정</summary>

![앱 설정](public/img_1.png)

</details>

## 기능 개요

### 호스트와 연결

- **통합 자산 라이브러리**: 호스트와 중첩 그룹을 관리하고 검색, 태그, 메모, 사용자 지정 아이콘과 색상, 사이드바 및 전체 화면 자산 보기를 지원합니다.
- **다양한 연결 방식**: SSH, Telnet, 직렬, RDP, VNC를 지원하며 로컬 셸도 열 수 있습니다.
- **SSH 인증**: 비밀번호, 개인 키, SSH Agent, 대화형 인증과 OTP 항목을 연동하는 2단계 인증 설정을 지원합니다.
- **복잡한 네트워크 접근**: SSH 점프 호스트 체인, SOCKS5 / HTTP 프록시. 프록시 설정을 재사용할 수 있고 RDP와 VNC에도 점프 호스트나 프록시 경로를 설정할 수 있습니다.
- **연결 옵션**: 프로토콜별로 터미널, 인코딩, 연결 유지, 로그인 후 명령, SFTP와 원격 데스크톱 매개변수를 설정합니다.

### 터미널 작업 공간

- 다중 세션 탭, 탭 잠금과 이름 변경, 드래그 정렬, 분할 창, 세션 창 분리와 복귀.
- 터미널 검색, 정규 표현식으로 설정하는 키워드 강조, 명령 기록, 자동 완성, 그룹으로 묶은 빠른 명령.
- 설정 가능한 여러 줄 붙여넣기 확인, 일괄 명령, 오른쪽 클릭 동작, 글꼴, 커서, 스크롤 버퍼와 렌더링 방식.
- 세션 기록과 메모, 터미널 녹화, 재생, 내보내기.
- 라이트 / 다크 테마, 터미널 색상 구성, 배경 이미지와 인터페이스 확대/축소.

### 파일과 운영

- **SFTP**: 로컬과 원격 디렉터리 탐색, 업로드와 다운로드, 드래그 앤 드롭 전송, 전송 큐, 진행률과 취소, 덮어쓰기 확인, 디렉터리 따라가기.
- **원격 텍스트 편집**: 내장 편집기에서 원격 파일을 열어 편집하고 저장하며, 파일 변경 충돌을 처리합니다.
- **포트 포워딩**: 로컬, 원격, 동적 SOCKS 포워딩을 지원하고 그룹과 실행 상태를 관리합니다.
- **리소스 모니터링**: CPU, 메모리, 디스크, 네트워크 등의 지표와 기록 그래프를 확인합니다. 사용 가능한 지표는 원격 환경에 따라 다릅니다.
- **프로세스와 서비스**: 프로세스를 확인하고, 지원되는 시스템에서 서비스를 관리하며 로그를 조회합니다.
- **Docker**: 현재 SSH 세션을 통해 원격 컨테이너, 이미지, 볼륨, 네트워크를 관리하고 리소스 세부 정보와 컨테이너 로그를 확인합니다.
- **인증 도구**: TOTP / HOTP 항목을 관리하고 `otpauth://` 또는 QR 코드에서 가져오며, 일회용 코드 표시, 복사, QR 코드 표시를 지원합니다.

### AI 어시스턴트

작업 공간 안에서 스트리밍 대화를 하고, 선택한 터미널과 호스트 및 기타 컨텍스트를 바탕으로 문제를 분석하며, Agent 도구로 작업 실행을 지원합니다.

- 여러 모델 공급자, 서비스 주소, 모델과 API Key를 설정할 수 있고 연결 미리보기를 지원합니다.
- OpenAI Chat Completions, OpenAI Responses, Anthropic Messages 프로토콜 형식을 지원합니다. 호환 서비스의 실제 기능은 해당 구현에 따릅니다.
- 세션 기록, Markdown / 코드 / 수식 표시, 명령 승인과 Agent 동작 설정을 지원합니다.
- Agent 명령은 로컬 보안 정책과 승인 설정에 따라 실행됩니다. 네이티브 도구 호출은 OpenAI Chat Completions와 Anthropic을 지원하고, Responses는 호환 폴백을 사용합니다.

AI를 사용하기 전에 모델 서비스를 직접 설정해야 합니다. 컨텍스트에 포함한 내용은 설정한 서비스로 전송되므로 실제 데이터 범위에 맞게 컨텍스트를 선택하세요.

## 화면과 작업 진입점

| 화면 / 영역                | 할 수 있는 일                                                                                                       |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| 자산 라이브러리와 사이드바 | 호스트 검색과 정리, 그룹 관리, 연결 열기, 기존 설정 가져오기                                                        |
| 호스트 편집기              | 프로토콜별로 주소, 인증, 점프 호스트 / 프록시와 고급 연결 매개변수 설정                                             |
| 세션 작업 공간             | 터미널이나 원격 데스크톱 사용, 탭 전환, 분할 창, 창 분리                                                            |
| 오른쪽 도구 패널           | 리소스 모니터링, 프로세스, 서비스, Docker, SFTP, 빠른 명령, 명령 기록, 포트 포워딩, 인증, 테마와 AI 어시스턴트 열기 |
| 설정 → 관리                | 자격 증명, 지문, 프록시, 포트 포워딩, 빠른 명령, 세션 기록, 터미널 녹화                                             |
| 설정 → 환경 설정           | 앱, 모양, 터미널, 원격 데스크톱, 키워드 강조, 자격 증명 제안, 단축키, AI 어시스턴트, 정보                           |
| 설정 → 정보                | 버전과 업데이트 확인, 가져오기 / 내보내기 등의 진입점                                                               |

도구 패널의 항목은 세션 유형과 연결 상태에 따라 표시됩니다. 예를 들어 Docker는 연결된 SSH 세션과 원격 Docker 환경이 필요합니다.

## 시작하기

1. 앱을 실행하고 안내에 따라 로컬 보관소를 초기화하거나 잠금을 해제합니다.
2. 그룹과 호스트를 새로 만들고 프로토콜을 선택한 뒤 연결 정보를 입력합니다. 기존 클라이언트에서 호스트를 가져올 수도 있습니다.
3. SSH 호스트에는 비밀번호, 개인 키 또는 SSH Agent를 설정합니다. 첫 연결 시 호스트 지문을 확인하고 승인하세요.
4. 호스트 세션을 열고 오른쪽에서 필요한 도구를 선택합니다. AI가 필요하면 먼저 '설정 → AI 어시스턴트'에서 모델 서비스를 설정합니다.
5. 설정에서 모양, 터미널, 단축키, 녹화와 파일 전송 환경 설정을 조정합니다.

### 설정 마이그레이션

OpenSSH 설정, PuTTY, Xshell, Termius, SecureCRT, MobaXterm, WindTerm, FinalShell, Electerm, JSON 세션 파일 등의 가져오기 진입점을 제공하며, Qterm 리소스 백업의 가져오기와 내보내기도 지원합니다.

가져올 때는 먼저 검사하고 미리 본 뒤 필요한 항목을 선택합니다. 가져오기 원본에 따라 옮길 수 있는 프로토콜, 자격 증명과 고급 옵션이 다르며, 원래 클라이언트 설정의 완전한 복사본으로 볼 수 없습니다. 마스터 비밀번호나 시스템 자격 증명 접근에 추가 입력이 필요할 수도 있습니다.

### 로컬 데이터와 자격 증명

호스트, 그룹, 기록 등의 데이터는 로컬 SQLite에 저장하고 비밀 정보는 암호화된 보관소에서 관리합니다. 보관소는 AES-256-GCM을 사용하며 마스터 비밀번호 보호에는 Argon2id 파생을 사용합니다. 키 조정과 복구 패키지 진입점도 제공합니다. SSH 첫 연결 시에는 호스트 키 확인이 필요하며, 신뢰한 지문은 설정에서 관리할 수 있습니다.

## 플랫폼과 사용 범위

| 플랫폼  | 배포 대상             | 실행 시 의존성 / 주의 사항                                                                                  |
| ------- | --------------------- | ----------------------------------------------------------------------------------------------------------- |
| macOS   | Apple Silicon / Intel | 시스템 WebView를 사용합니다. 로컬 포터블 패키지는 임시 서명이며 정식 서명과 공증은 아직 완료되지 않았습니다 |
| Windows | amd64 / arm64         | WebView2 Runtime이 필요합니다                                                                               |
| Linux   | amd64 / arm64         | 빌드 대상과 일치하는 GTK / WebKit 런타임 라이브러리가 필요합니다                                            |

- **RDP는 추가 helper에 의존**: FreeRDP 기반 helper와 그 동적 라이브러리를 사용합니다. 6개 플랫폼 로컬 패키징 진입점은 macOS helper가 대상 아키텍처와 일치할 때만 이를 포함하며, Windows / Linux에는 자동으로 포함하지 않습니다. 해당 helper가 없으면 RDP를 바로 사용할 수 없습니다.
- **원격 도구는 환경에 의존**: SFTP는 서버의 하위 시스템이 필요하고 Docker는 원격 CLI와 권한이 필요합니다. 서비스 관리와 모니터링은 운영 체제와 설치된 도구에 따라 제한됩니다.
- **브라우저 미리보기는 화면 개발 전용**: 웹의 인메모리 게이트웨이는 네이티브 SSH, SFTP, 원격 데스크톱, 보관소 기능을 대체할 수 없습니다.
- **자동 업데이트 연동됨**: 데스크톱 앱은 GitHub Release를 확인하고 백그라운드에서 내려받아 SHA-256을 검증한 뒤, 사용자가 확인하면 다시 시작하여 설치합니다. 준비가 끝나면 상단 표시줄에 알림을 표시합니다. 실제 Release 전체 흐름, Windows / Linux 교체와 재시작, 릴리스 CI는 실기 검증이 필요합니다.
- **크로스 플랫폼 검증 진행 중**: 빌드 대상과 기존 구현은 모든 기능이 6가지 OS / 아키텍처 조합에서 검증되었다는 뜻이 아닙니다.

## 로컬 개발

### 환경 준비

| 도구             | 현재 프로젝트 기준                                                                                     |
| ---------------- | ------------------------------------------------------------------------------------------------------ |
| Go               | 1.26.0 또는 `go.mod`와 호환되는 버전                                                                   |
| Node.js          | 22.22.2+의 22.x 또는 24.15.0+의 24.x. 고정된 의존성의 engines 요구 사항을 충족해야 합니다              |
| npm              | 고정된 프런트엔드 의존성 설치                                                                          |
| Wails CLI        | v3.1.0, Go 모듈 버전과 일치                                                                            |
| 플랫폼 도구 체인 | macOS는 Xcode Command Line Tools가 필요하고, 다른 플랫폼은 해당 네이티브 빌드 의존성을 준비해야 합니다 |

리포지토리 루트에서 실행합니다:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.1.0
npm --prefix frontend ci
wails3 doctor
wails3 task dev
```

Go 실행 파일 디렉터리를 `PATH`에 추가하세요. `wails3 task dev`는 `build/config.yml`을 사용하며 기본 프런트엔드 포트는 `9245`입니다.

프런트엔드만 미리 보는 경우(브라우저에서 `http://127.0.0.1:9245`를 엽니다. 이때는 인메모리 게이트웨이를 사용하므로 실제 SSH / SFTP 기능은 없습니다):

```sh
npm --prefix frontend run dev
```

### 검사와 빌드

```sh
# 로컬 전체 검사: Go 포맷, 테스트, vet. 프런트엔드 포맷, lint, 타입, 단위 테스트, 빌드와 E2E
wails3 task check

# 현재 플랫폼 앱 빌드
wails3 build

# 현재 플랫폼 설치 / 배포 산출물 생성
wails3 package
```

전체 검사에는 리포지토리의 테스트 파일과 Playwright 브라우저 환경이 필요합니다. 순수 코드 스냅샷에는 전체 테스트 게이트가 포함되지 않습니다. 필요하면 E2E 브라우저를 설치하세요:

```sh
cd frontend
npx playwright install chromium
```

### 6개 플랫폼 포터블 패키지

**macOS** 리포지토리 루트에서 실행합니다:

```sh
# 계획만 출력
wails3 task release:all DRY_RUN=true

# 실제 빌드
wails3 task release:all
```

Go, Node.js/npm, Wails CLI, Xcode Command Line Tools와 실행 중인 Docker Desktop이 필요합니다. 처음 실행하면 Linux 크로스 컴파일 이미지를 빌드하므로 네트워크가 필요합니다.

산출물은 `bin/releases/` 아래의 새 디렉터리에 기록되며, macOS / Windows ZIP, Linux tar.gz와 모두 성공했을 때 생성되는 `SHA256SUMS`가 포함됩니다. 이 명령은 자동 업로드를 하지 않으며 DMG, NSIS, deb, rpm도 생성하지 않습니다.

리포지토리에는 GitHub Actions Release 워크플로도 있으며, 버전 태그에 따라 6가지 대상을 빌드하고 검증 파일을 모읍니다. 워크플로의 실제 runner 검증은 아직 완료되지 않았습니다.

### 커밋 규약

커밋 메시지는 Conventional Commits(`<type>(<scope>): <subject>`)를 사용하고 commitlint로 검증하며 czg로 생성합니다:

```sh
npm install        # 최초: czg / commitlint / husky를 설치하고 commit-msg 훅을 연결
npm run commit     # 커밋 메시지를 대화형으로 생성
```

- **type**: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`, `wip`, `workflow`, `types`, `release`.
- **scope**: `internal/*`와 `frontend/src/features/*`의 디렉터리 이름이 그대로 선택 가능한 scope가 되며, `project`, `deps`, `build`, `ci`, `release`, `docs`, `scripts`, `i18n`, `ui`, `other`도 있습니다. 생략할 수 있습니다.
- **header는 최대 108자**, subject는 비울 수 없습니다. 규칙을 만족하지 않으면 커밋이 거부됩니다.

규칙 세부 사항은 `docs/development/commit-convention.md`를 참고하세요(이 디렉터리는 내부 리포지토리에만 보관됩니다).

## 기술과 디렉터리

| 부분                   | 기술                                                         |
| ---------------------- | ------------------------------------------------------------ |
| 데스크톱 컨테이너      | Wails v3.1.0, Go                                             |
| 프런트엔드             | Vue 3, TypeScript, Vite, Tailwind CSS, Reka UI               |
| 터미널과 원격 데스크톱 | xterm.js, noVNC, FreeRDP helper                              |
| 편집기                 | CodeMirror, Monaco Editor                                    |
| 저장소와 암호화        | SQLite, AES-256-GCM, Argon2id                                |
| 품질 검사              | Go test / vet, Vitest, Playwright, ESLint, Prettier, vue-tsc |

```text
main.go                  앱 진입점과 리소스 임베드
internal/app/            Wails Facade, 조립, 창과 런타임 브리지
internal/                호스트, 세션, 터미널, SFTP, AI 등의 기능과 기반
frontend/src/app/        앱 셸과 창 진입점
frontend/src/features/   기능 단위로 구성한 화면, 패널, 상태 로직
frontend/src/services/   프런트엔드 게이트웨이와 런타임 서비스 래퍼
frontend/src/components/ 공용 컨트롤, 레이아웃, 대화 상자와 작업 공간 컴포넌트
frontend/src/locales/    7개 언어의 인터페이스 문구
frontend/bindings/       생성된 Wails TypeScript 바인딩
build/                   플랫폼별 빌드 설정과 릴리스 스크립트
tools/                   RDP helper 소스, OTP 로컬 테스트 픽스처
Taskfile.yml             개발, 검증, 빌드 작업 진입점
```

디렉터리의 자세한 역할은 [`internal/README.md`](internal/README.md)와 [`frontend/README.md`](frontend/README.md)를 참고하세요.
