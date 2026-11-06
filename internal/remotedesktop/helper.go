package remotedesktop

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
)

const (
	helperFirstFrameTimeout = 25 * time.Second

	helperFirstFrameTimeoutMin = 5 * time.Second
	helperFirstFrameTimeoutMax = 300 * time.Second

	helperPendingBytesLimit = 256 << 20

	freeRDPEventUnicode   = 6
	freeRDPEventClipboard = 7
	freeRDPEventClipReq   = 8
	freeRDPEventResize    = 9

	helperDynamicResolutionEnv = "QTERM_RDP_DYNAMIC_RESOLUTION"

	helperDynamicResolutionFallbackLine = "QTERM-RDP-FALLBACK no-frames"

	helperClipboardModeEnv    = "QTERM_RDP_CLIPBOARD_MODE"
	helperClipboardContentEnv = "QTERM_RDP_CLIPBOARD_CONTENT"

	helperExitPrefix = "QTERM-HELPER-EXIT"

	helperExitReasonStdinEOF   = "stdin-eof"
	helperExitReasonCloseEvent = "close-event"

	helperClipboardMaxBytes = 1 << 20

	helperFrameHeaderSize = 44
	helperPixelFormatRGBA = 2

	freeRDPEventMove   = 1
	freeRDPEventButton = 2
	freeRDPEventWheel  = 3
	freeRDPEventKey    = 4
	freeRDPEventClose  = 5
	freeRDPEventBytes  = 24

	helperStderrDrainTimeout = 500 * time.Millisecond

	helperProcessReapTimeout = 300 * time.Millisecond

	helperExitCertificateRejected = 5
)

func NormalizeFirstFrameTimeout(value time.Duration) time.Duration {
	if value < helperFirstFrameTimeoutMin || value > helperFirstFrameTimeoutMax {
		return helperFirstFrameTimeout
	}
	return value
}

const (
	helperCertificatePrefixSHA256  = "QTERM-CERT-SHA256"
	helperCertificatePrefixSubject = "QTERM-CERT-SUBJECT"
	helperCertificatePrefixIssuer  = "QTERM-CERT-ISSUER"
	helperCertificateNeedTrust     = "QTERM-CERT-NEED-TRUST"
	helperCertificateMismatch      = "QTERM-CERT-MISMATCH"
	helperCertificateInsecure      = "QTERM-CERT-INSECURE"
)

const (
	helperCertificateFingerprintEnv = "QTERM_RDP_CERT_SHA256"
	helperCertificateInsecureEnv    = "QTERM_RDP_CERT_INSECURE"
)

type CertificateState string

const (
	CertificateStateUnknown CertificateState = "unknown"

	CertificateStateNeedTrust CertificateState = "needTrust"

	CertificateStateMismatch CertificateState = "mismatch"

	CertificateStateTrusted CertificateState = "trusted"

	CertificateStateInsecure CertificateState = "insecure"
)

type CertificateStatus struct {
	State       CertificateState
	Fingerprint string
	Subject     string
	Issuer      string
}

var (
	ErrHelperMissing = errors.New("RDP helper 未找到")

	ErrUnknownEngine = errors.New("未知的远程桌面引擎")

	ErrCertificateUntrusted = errors.New("服务器证书尚未信任")

	ErrCertificateMismatch = errors.New("服务器证书与本机记录不一致（证书可能已变更）")
)

type HelperEngine string

const (
	HelperEngineIronRDP HelperEngine = "ironrdp"

	HelperEngineFreeRDP HelperEngine = "freerdp"
)

func ParseHelperEngine(value string) (HelperEngine, bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "native", "go":
		return "", false, nil
	case string(HelperEngineIronRDP):
		return HelperEngineIronRDP, true, nil
	case string(HelperEngineFreeRDP):
		return HelperEngineFreeRDP, true, nil
	}
	return "", false, fmt.Errorf("%w %q（可选：native/ironrdp/freerdp）", ErrUnknownEngine, value)
}

func resolveHelperEngine(value string) (HelperEngine, error) {
	engine, isHelper, err := ParseHelperEngine(value)
	if err != nil {
		return "", err
	}
	if !isHelper {
		return "", fmt.Errorf("%w：自研纯 Go 栈已移除，请使用 freerdp", ErrUnknownEngine)
	}
	if engine != HelperEngineFreeRDP {
		return "", fmt.Errorf("%w：%s helper 在 Windows Server 2008 R2 目标上无法出图，当前只保留 freerdp",
			ErrUnknownEngine, engine)
	}
	return engine, nil
}

func helperBinaryPath(engine HelperEngine) (string, error) {
	envKey := "QTERM_RDP_HELPER_IRONRDP"
	name := "qterm-ironrdp-helper"
	relative := filepath.Join("tools", "rdp-helper-poc", "ironrdp", "target", "release")
	if engine == HelperEngineFreeRDP {
		envKey = "QTERM_RDP_HELPER_FREERDP"
		name = "qterm-freerdp-helper"
		relative = filepath.Join("tools", "rdp-helper-poc", "freerdp", "build")
	}
	if runtime.GOOS == "windows" {
		relative = filepath.Join(relative, name+".exe")
	} else {
		relative = filepath.Join(relative, name)
	}
	if override := strings.TrimSpace(os.Getenv(envKey)); override != "" {
		if _, err := os.Stat(override); err != nil {
			return "", fmt.Errorf("%w：环境变量 %s 指向的 helper 不可用: %v", ErrHelperMissing, envKey, err)
		}
		return override, nil
	}
	for _, candidate := range helperCandidates(relative) {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	if found, err := exec.LookPath(name); err == nil {
		return found, nil
	}
	hint := "cargo build --manifest-path tools/rdp-helper-poc/ironrdp/Cargo.toml --release"
	if engine == HelperEngineFreeRDP {
		hint = "cmake -S tools/rdp-helper-poc/freerdp -B tools/rdp-helper-poc/freerdp/build ... && cmake --build tools/rdp-helper-poc/freerdp/build"
	}
	return "", fmt.Errorf("%w：%s（请先构建：%s；或用 %s 指定路径）", ErrHelperMissing, engine, hint, envKey)
}

func helperCandidates(relative string) []string {
	candidates := []string{relative}
	if workingDirectory, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(workingDirectory, relative))

		directory := workingDirectory
		for depth := 0; depth < 6; depth++ {
			parent := filepath.Dir(directory)
			if parent == directory {
				break
			}
			directory = parent
			if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
				candidates = append(candidates, filepath.Join(directory, relative))
				break
			}
		}
	}

	if executable, err := os.Executable(); err == nil {
		directory := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(directory, relative),
			filepath.Join(directory, "helpers", filepath.Base(relative)),

			filepath.Join(directory, "..", "Resources", "helpers", filepath.Base(relative)),
		)
	}

	if _, sourceFile, _, ok := runtime.Caller(0); ok {
		directory := filepath.Dir(sourceFile)
		for depth := 0; depth < 6; depth++ {
			directory = filepath.Dir(directory)
			if _, err := os.Stat(filepath.Join(directory, "go.mod")); err == nil {
				candidates = append(candidates, filepath.Join(directory, relative))
				break
			}
		}
	}
	return candidates
}

type HelperAvailability struct {
	Engine  string
	Ready   bool
	Path    string
	Message string
}

func HelperAvailabilityReport() []HelperAvailability {
	result := make([]HelperAvailability, 0, 1)
	for _, engine := range []HelperEngine{HelperEngineFreeRDP} {
		path, err := helperBinaryPath(engine)
		item := HelperAvailability{Engine: string(engine), Ready: err == nil, Path: path}
		if err != nil {
			item.Message = err.Error()
		}
		result = append(result, item)
	}
	return result
}

type helperSession struct {
	engine HelperEngine
	cmd    *exec.Cmd
	stdin  io.WriteCloser

	width  int
	height int

	writeMu sync.Mutex

	stdinCloseOnce sync.Once
	mu             sync.Mutex
	stats          EventStats
	frames         int
	err            error
	closed         bool

	jsonInput bool

	pending      []helperFrame
	pendingCount int
	pendingBytes int

	stderrTail []string

	stderrDone chan struct{}

	certificate CertificateStatus

	dynamicResolutionFallback bool

	resizeUnsupported bool

	remoteClipboard string

	clipboardSequence uint64

	waitOnce sync.Once

	exitCode int

	exitReason string
	startedAt  time.Time
	frameSeen  bool

	firstFrameTimeout time.Duration
}

func openHelperSession(ctx context.Context, engine HelperEngine, options SessionOptions) (*helperSession, error) {
	path, err := helperBinaryPath(engine)
	if err != nil {
		return nil, err
	}

	command := exec.Command(path)

	command.Env = helperEnvironment(os.Environ(), options.CertificateFingerprint, options.DynamicResolutionDisabled, options.Clipboard)
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 helper 输入管道失败: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 helper 画面管道失败: %w", err)
	}

	stderr, err := command.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("创建 helper 诊断管道失败: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("启动 %s helper 失败: %w", engine, err)
	}

	session := &helperSession{
		engine:    engine,
		cmd:       command,
		stdin:     stdin,
		width:     int(options.Width),
		height:    int(options.Height),
		jsonInput: engine == HelperEngineIronRDP,
		startedAt: time.Now(),

		certificate: CertificateStatus{State: CertificateStateUnknown},
		stderrDone:  make(chan struct{}),
		exitCode:    -1,

		firstFrameTimeout: NormalizeFirstFrameTimeout(options.FirstFrameTimeout),
	}
	go func() {
		defer close(session.stderrDone)
		session.captureStderr(stderr)
	}()
	if err := session.sendConnectionOptions(options); err != nil {
		_ = session.Close()
		return nil, err
	}
	go session.readFrames(stdout)
	return session, nil
}

func (s *helperSession) sendConnectionOptions(options SessionOptions) error {
	if err := validateHelperLineFields(options); err != nil {
		return err
	}

	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	width, height := s.width, s.height
	if width <= 0 {
		width = defaultWidth
	}
	if height <= 0 {
		height = defaultHeight
	}
	if s.jsonInput {
		config := struct {
			Host     string `json:"host"`
			Port     int    `json:"port"`
			Username string `json:"username"`
			Password string `json:"password"`
			Domain   string `json:"domain"`
			Width    int    `json:"width"`
			Height   int    `json:"height"`
		}{options.Host, options.Port, options.Username, options.Password, options.Domain, width, height}
		payload, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("组装 helper 连接配置失败: %w", err)
		}
		return s.writeBytesLocked(append(payload, '\n'), "传递 helper 连接配置")
	}

	arguments := []string{
		fmt.Sprintf("/v:%s:%d", options.Host, options.Port),
		"/u:" + options.Username,
		"/p:" + options.Password,
		fmt.Sprintf("/size:%dx%d", width, height),

		"/tls:seclevel:0",
		"/network:lan",
		"+auto-reconnect",
	}
	if options.Domain != "" {
		arguments = append(arguments, "/d:"+options.Domain)
	}
	for _, argument := range arguments {
		if err := validateHelperLine(argument); err != nil {
			return err
		}
		if err := s.writeBytesLocked([]byte(argument+"\n"), "传递 FreeRDP 参数"); err != nil {
			return fmt.Errorf("传递 FreeRDP 参数失败: %w", err)
		}
	}
	if err := s.writeBytesLocked([]byte("\n"), "结束 FreeRDP 参数流"); err != nil {
		return fmt.Errorf("结束 FreeRDP 参数流失败: %w", err)
	}
	return nil
}

func validateHelperLineFields(options SessionOptions) error {
	fields := []struct {
		name  string
		value string
	}{
		{name: "主机地址", value: options.Host},
		{name: "用户名", value: options.Username},
		{name: "密码", value: options.Password},
		{name: "域", value: options.Domain},
	}
	for _, field := range fields {
		if strings.ContainsAny(field.value, "\r\n") {
			return fmt.Errorf("helper 连接%s不能包含换行符", field.name)
		}
	}
	return nil
}

func validateHelperLine(value string) error {
	if strings.ContainsAny(value, "\r\n") {
		return errors.New("FreeRDP 参数不能包含换行符")
	}
	return nil
}

func (s *helperSession) captureStderr(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		s.applyCertificateLine(line)

		if value, ok := strings.CutPrefix(line, "QTERM-CLIPBOARD-B64 "); ok {
			if decoded, decodeErr := base64.StdEncoding.DecodeString(strings.TrimSpace(value)); decodeErr == nil {
				s.mu.Lock()
				s.remoteClipboard = string(decoded)
				s.clipboardSequence++
				s.mu.Unlock()
			}
			continue
		}
		if line == helperDynamicResolutionFallbackLine {
			s.mu.Lock()
			s.dynamicResolutionFallback = true
			s.mu.Unlock()
			continue
		}
		if line == "QTERM-RESIZE-UNSUPPORTED" {
			s.mu.Lock()
			s.resizeUnsupported = true
			s.mu.Unlock()
			continue
		}
		if reason, ok := strings.CutPrefix(line, helperExitPrefix+" "); ok {

			s.mu.Lock()
			if s.exitReason == "" {
				s.exitReason = strings.TrimSpace(reason)
			}
			s.mu.Unlock()
		}
		if line == "QTERM-CLIPBOARD-EMPTY" {
			s.mu.Lock()
			s.remoteClipboard = ""
			s.clipboardSequence++
			s.mu.Unlock()
			continue
		}
		s.mu.Lock()
		s.stderrTail = append(s.stderrTail, line)
		if len(s.stderrTail) > 20 {
			s.stderrTail = s.stderrTail[len(s.stderrTail)-20:]
		}
		s.mu.Unlock()
	}
}

func (s *helperSession) applyCertificateLine(line string) {
	switch line {
	case helperCertificateNeedTrust:
		s.markCertificateRejected(CertificateStateNeedTrust)
		return
	case helperCertificateMismatch:
		s.markCertificateRejected(CertificateStateMismatch)
		return
	case helperCertificateInsecure:
		s.markCertificateState(CertificateStateInsecure)
		return
	}
	switch {
	case strings.HasPrefix(line, helperCertificatePrefixSHA256+" "):
		value := strings.TrimSpace(strings.TrimPrefix(line, helperCertificatePrefixSHA256))

		normalized := NormalizeCertificateFingerprint(value)
		if normalized == "" {
			return
		}
		s.mu.Lock()
		s.certificate.Fingerprint = normalized
		s.mu.Unlock()
		s.markCertificateState(CertificateStateTrusted)
	case strings.HasPrefix(line, helperCertificatePrefixSubject+" "):
		value := strings.TrimSpace(strings.TrimPrefix(line, helperCertificatePrefixSubject))
		s.mu.Lock()
		s.certificate.Subject = value
		s.mu.Unlock()
	case strings.HasPrefix(line, helperCertificatePrefixIssuer+" "):
		value := strings.TrimSpace(strings.TrimPrefix(line, helperCertificatePrefixIssuer))
		s.mu.Lock()
		s.certificate.Issuer = value
		s.mu.Unlock()
	}
}

func (s *helperSession) markCertificateState(state CertificateState) {
	s.mu.Lock()
	if certificateStateRank(state) > certificateStateRank(normalizeCertificateState(s.certificate.State)) {
		s.certificate.State = state
	}
	s.mu.Unlock()
}

func (s *helperSession) markCertificateRejected(state CertificateState) {
	s.markCertificateState(state)
	if err := certificateStateError(state); err != nil {
		s.recordError(err)
	}
}

func certificateStateRank(state CertificateState) int {
	switch state {
	case CertificateStateMismatch:
		return 4
	case CertificateStateNeedTrust:
		return 3
	case CertificateStateInsecure:
		return 2
	case CertificateStateTrusted:
		return 1
	}
	return 0
}

func normalizeCertificateState(state CertificateState) CertificateState {
	switch state {
	case CertificateStateNeedTrust, CertificateStateMismatch, CertificateStateTrusted, CertificateStateInsecure:
		return state
	}
	return CertificateStateUnknown
}

func certificateStateError(state CertificateState) error {
	switch state {
	case CertificateStateNeedTrust:
		return fmt.Errorf("%w，需要你确认指纹后重新连接", ErrCertificateUntrusted)
	case CertificateStateMismatch:
		return fmt.Errorf("%w，已拒绝连接；请核对指纹或删除信任记录", ErrCertificateMismatch)
	}
	return nil
}

func (s *helperSession) CertificateStatus() CertificateStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.certificate
	status.State = normalizeCertificateState(status.State)
	return status
}

func (s *helperSession) CertificateError() error {
	return certificateStateError(s.CertificateStatus().State)
}

func (s *helperSession) waitStderrDrain() {
	if s.stderrDone == nil {
		return
	}
	select {
	case <-s.stderrDone:
	case <-time.After(helperStderrDrainTimeout):
	}
}

func (s *helperSession) HelperStderr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return strings.Join(s.stderrTail, "\n")
}

func (s *helperSession) readFrames(reader io.Reader) {
	buffered := bufio.NewReaderSize(reader, 1<<20)
	header := make([]byte, helperFrameHeaderSize)
	for {
		if _, err := io.ReadFull(buffered, header); err != nil {
			s.recordTerminalError(err)
			return
		}
		desktopWidth := int(binary.LittleEndian.Uint32(header[8:12]))
		desktopHeight := int(binary.LittleEndian.Uint32(header[12:16]))
		x := int(binary.LittleEndian.Uint32(header[16:20]))
		y := int(binary.LittleEndian.Uint32(header[20:24]))
		width := int(binary.LittleEndian.Uint32(header[24:28]))
		height := int(binary.LittleEndian.Uint32(header[28:32]))
		stride := int(binary.LittleEndian.Uint32(header[32:36]))
		format := binary.LittleEndian.Uint32(header[36:40])
		payloadLength := int(binary.LittleEndian.Uint32(header[40:44]))
		if err := validateHelperFrame(desktopWidth, desktopHeight, x, y, width, height, stride, format, payloadLength); err != nil {
			s.recordTerminalError(err)
			return
		}
		payload := make([]byte, payloadLength)
		if _, err := io.ReadFull(buffered, payload); err != nil {
			s.recordTerminalError(err)
			return
		}
		if !s.applyFrame(desktopWidth, desktopHeight, x, y, width, height, stride, payload) {
			return
		}
	}
}

func validateHelperFrame(desktopWidth, desktopHeight, x, y, width, height, stride int, format uint32, payloadLength int) error {
	if desktopWidth < 1 || desktopHeight < 1 || desktopWidth > 7680 || desktopHeight > 4320 {
		return fmt.Errorf("helper 返回非法桌面尺寸 %dx%d", desktopWidth, desktopHeight)
	}
	if width < 1 || height < 1 || x < 0 || y < 0 || x+width > desktopWidth || y+height > desktopHeight {
		return errors.New("helper 返回越界画面区域")
	}
	if format != helperPixelFormatRGBA || stride < width*4 || payloadLength != stride*height || payloadLength > 128<<20 {
		return errors.New("helper 返回非法画面负载")
	}
	return nil
}

func (s *helperSession) applyFrame(desktopWidth, desktopHeight, x, y, width, height, stride int, payload []byte) bool {
	s.mu.Lock()

	if len(payload) > helperPendingBytesLimit || s.pendingBytes > helperPendingBytesLimit-len(payload) {

		if s.err == nil {
			s.err = fmt.Errorf("helper 待处理画面超过 %d MiB，已停止读取以避免丢失画面更新", helperPendingBytesLimit>>20)
		}
		s.mu.Unlock()
		return false
	}
	s.pending = append(s.pending, helperFrame{
		desktopWidth: desktopWidth, desktopHeight: desktopHeight,
		x: x, y: y, width: width, height: height, stride: stride, payload: payload,
	})
	s.pendingBytes += len(payload)
	s.pendingCount++
	s.frames++
	s.frameSeen = true
	s.width = desktopWidth
	s.height = desktopHeight
	s.mu.Unlock()
	return true
}

type helperFrame struct {
	desktopWidth, desktopHeight int
	x, y, width, height, stride int
	payload                     []byte
}

func (s *helperSession) recordError(err error) {
	if errors.Is(err, io.EOF) || errors.Is(err, os.ErrClosed) {
		s.mu.Lock()
		reason := s.exitReason
		s.mu.Unlock()
		err = errors.New(helperExitDescription(reason))
	}
	s.mu.Lock()
	if s.err == nil && !s.closed {
		s.err = err
	}
	s.mu.Unlock()
}

func helperExitWasRequested(reason string) bool {
	switch strings.TrimSpace(reason) {
	case helperExitReasonStdinEOF, helperExitReasonCloseEvent:
		return true
	}
	return false
}

func helperExitDescription(reason string) string {
	trimmed := strings.TrimSpace(reason)
	switch {
	case trimmed == "":
		return "helper 进程已退出（未上报退出原因）"
	case helperExitWasRequested(trimmed):
		return fmt.Sprintf("helper 进程已退出（正常关闭：%s）", trimmed)
	default:
		return fmt.Sprintf("helper 进程已退出（意外退出：%s）", trimmed)
	}
}

func (s *helperSession) recordTerminalError(err error) {
	s.mu.Lock()
	alreadyClosed := s.closed
	s.mu.Unlock()
	if !alreadyClosed {

		s.waitStderrDrain()

		s.reapProcessWithin(helperProcessReapTimeout)

		s.mu.Lock()
		rejected := s.exitCode == helperExitCertificateRejected
		s.mu.Unlock()
		if rejected {

			s.markCertificateState(CertificateStateNeedTrust)
		}
	}

	if certificateErr := s.CertificateError(); certificateErr != nil {
		err = certificateErr
	}
	s.recordError(err)
}

func (s *helperSession) reapProcess() {
	s.waitOnce.Do(func() {
		if s.cmd == nil {
			return
		}

		_ = s.cmd.Wait()
		code := -1
		if state := s.cmd.ProcessState; state != nil {
			code = state.ExitCode()
		}
		s.mu.Lock()
		s.exitCode = code
		s.mu.Unlock()
	})
}

func (s *helperSession) reapProcessWithin(timeout time.Duration) {
	done := make(chan struct{})
	go func() {
		s.reapProcess()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
	}
}

const (
	helperStderrExcerptLines = 3
	helperStderrExcerptRunes = 160
)

func helperStderrExcerpt(lines []string) string {
	pickedIdx := make([]int, 0, helperStderrExcerptLines)

	for pass := 0; pass < 2 && len(pickedIdx) < helperStderrExcerptLines; pass++ {
		wantError := pass == 0
		for i := len(lines) - 1; i >= 0 && len(pickedIdx) < helperStderrExcerptLines; i-- {
			line := strings.TrimSpace(lines[i])
			if line == "" {
				continue
			}
			upper := strings.ToUpper(line)
			isError := strings.Contains(upper, "[ERROR]") || strings.Contains(upper, "ERROR:")
			if isError != wantError {
				continue
			}
			pickedIdx = append(pickedIdx, i)
		}
	}
	if len(pickedIdx) == 0 {
		return "（helper 没有输出诊断信息，完整记录见右侧竖栏「日志」）"
	}

	for i := 1; i < len(pickedIdx); i++ {
		for j := i; j > 0 && pickedIdx[j] < pickedIdx[j-1]; j-- {
			pickedIdx[j], pickedIdx[j-1] = pickedIdx[j-1], pickedIdx[j]
		}
	}
	picked := make([]string, 0, len(pickedIdx))
	for _, idx := range pickedIdx {
		picked = append(picked, truncateRunes(strings.TrimSpace(lines[idx]), helperStderrExcerptRunes))
	}
	return "；最近输出：" + strings.Join(picked, " | ") + "（完整输出见右侧竖栏「日志」）"
}

func truncateRunes(text string, limit int) string {
	runes := []rune(text)
	if len(runes) <= limit {
		return text
	}
	return string(runes[:limit]) + "…"
}

func (s *helperSession) processExitCode() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitCode
}

func (s *helperSession) Render(ctx context.Context, framebuffer *Framebuffer, budget time.Duration) (EventStats, error) {
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		if err := ctx.Err(); err != nil {
			return EventStats{}, err
		}
		s.mu.Lock()
		frames := s.pending
		s.pending = nil
		s.pendingBytes = 0
		err := s.err

		firstFrameTimeout := s.firstFrameTimeout
		if firstFrameTimeout <= 0 {
			firstFrameTimeout = helperFirstFrameTimeout
		}
		if err == nil && !s.frameSeen && !s.closed && time.Since(s.startedAt) > firstFrameTimeout {

			if certificateErr := certificateStateError(normalizeCertificateState(s.certificate.State)); certificateErr != nil {
				err = certificateErr
			} else {
				err = fmt.Errorf("%s helper 已启动但 %s 内没有收到任何画面帧%s",
					s.engine, firstFrameTimeout, helperStderrExcerpt(s.stderrTail))
			}
		}
		s.mu.Unlock()

		for _, frame := range frames {
			width, height := framebuffer.Size()
			if width != frame.desktopWidth || height != frame.desktopHeight {
				framebuffer.Resize(frame.desktopWidth, frame.desktopHeight)
			}
			if blitErr := framebuffer.BlitRGBA(frame.x, frame.y, frame.width, frame.height, frame.stride, frame.payload); blitErr != nil {
				err = blitErr
				break
			}
			s.mu.Lock()
			s.stats.BitmapsApplied++
			s.mu.Unlock()
		}
		if err != nil {
			return s.snapshotStats(), err
		}

		sleep := 10 * time.Millisecond
		if remaining := time.Until(deadline); remaining < sleep {
			sleep = remaining
		}
		if sleep > 0 {
			select {
			case <-ctx.Done():
				return s.snapshotStats(), ctx.Err()
			case <-time.After(sleep):
			}
		}
	}
	return s.snapshotStats(), nil
}

func (s *helperSession) snapshotStats() EventStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	stats := s.stats
	stats.Packets = s.frames
	return stats
}

func (s *helperSession) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	s.mu.Unlock()

	s.closeStdin()
	s.waitForProcessExit(2 * time.Second)
	return nil
}

func (s *helperSession) closeStdin() {
	s.stdinCloseOnce.Do(func() {
		if s.stdin != nil {
			_ = s.stdin.Close()
		}
	})
}

func (s *helperSession) waitForProcessExit(timeout time.Duration) {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		s.reapProcess()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		_ = s.cmd.Process.Kill()

		select {
		case <-done:
		case <-time.After(helperProcessReapTimeout):
		}
	}
}

func (s *helperSession) SendMouseMove(x, y int) error {
	s.mu.Lock()
	s.stats.PointerUpdates++
	s.stats.PointerApplied++
	s.mu.Unlock()
	return s.writeInput(helperInput{Type: "mouse_move", X: x, Y: y})
}

func (s *helperSession) SendMouseButton(x, y int, button MouseButton, pressed bool) error {
	s.mu.Lock()
	s.stats.PointerUpdates++
	s.stats.PointerApplied++
	s.mu.Unlock()
	return s.writeInput(helperInput{Type: "mouse_button", X: x, Y: y, Button: int(button), Pressed: pressed})
}

func (s *helperSession) SendMouseClick(x, y int, button MouseButton) error {
	if err := s.SendMouseButton(x, y, button, true); err != nil {
		return err
	}
	return s.SendMouseButton(x, y, button, false)
}

func (s *helperSession) SendMouseWheel(x, y, notches int, horizontal bool) error {
	s.mu.Lock()
	s.stats.PointerUpdates++
	s.stats.PointerApplied++
	s.mu.Unlock()
	return s.writeInput(helperInput{Type: "wheel", X: x, Y: y, Notches: notches, Horizontal: horizontal})
}

func (s *helperSession) SendKey(scanCode uint16, pressed, extended bool) error {
	s.mu.Lock()
	s.stats.OtherUpdates++
	s.mu.Unlock()
	return s.writeInput(helperInput{Type: "key", ScanCode: int(scanCode), Pressed: pressed, Extended: extended})
}

func (s *helperSession) SendText(text string) error {
	for _, r := range text {
		stroke, ok := asciiKeyStroke(r)
		if !ok {

			for _, unit := range utf16.Encode([]rune{r}) {
				if err := s.sendUnicodeKey(unit); err != nil {
					return err
				}
			}
			continue
		}
		if stroke.shift {
			if err := s.SendKey(0x2A, true, false); err != nil {
				return err
			}
		}
		if err := s.SendKey(stroke.scanCode, true, false); err != nil {
			return err
		}
		if err := s.SendKey(stroke.scanCode, false, false); err != nil {
			return err
		}
		if stroke.shift {
			if err := s.SendKey(0x2A, false, false); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *helperSession) sendUnicodeKey(unit uint16) error {
	if err := s.writeFreeRDPEvent(freeRDPEventUnicode, 0, 0, int(unit), 1); err != nil {
		return err
	}
	return s.writeFreeRDPEvent(freeRDPEventUnicode, 0, 0, int(unit), 0)
}

func (s *helperSession) SetLocalClipboardText(text string) error {
	payload := []byte(text)
	if len(payload) > helperClipboardMaxBytes {
		return fmt.Errorf("剪贴板文本过长（%d 字节，上限 %d）", len(payload), helperClipboardMaxBytes)
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.isClosed() {
		return errors.New("会话已关闭")
	}
	buffer := make([]byte, 24)
	binary.LittleEndian.PutUint32(buffer[0:4], freeRDPEventClipboard)
	binary.LittleEndian.PutUint32(buffer[12:16], uint32(len(payload)))
	if err := s.writeBytesLocked(buffer, "写入剪贴板事件"); err != nil {
		return fmt.Errorf("写入剪贴板事件失败: %w", err)
	}
	if len(payload) > 0 {
		if err := s.writeBytesLocked(payload, "写入剪贴板负载"); err != nil {
			return fmt.Errorf("写入剪贴板负载失败: %w", err)
		}
	}
	return nil
}

func (s *helperSession) DynamicResolutionFallback() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dynamicResolutionFallback
}

func (s *helperSession) ResizeUnsupported() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.resizeUnsupported
}

func (s *helperSession) Resize(width, height int) error {
	if width < 640 || width > 7680 || height < 480 || height > 4320 {
		return fmt.Errorf("远程桌面尺寸超出允许范围（640×480 ~ 7680×4320）: %dx%d", width, height)
	}

	s.mu.Lock()
	s.resizeUnsupported = false
	s.mu.Unlock()
	return s.writeFreeRDPEvent(freeRDPEventResize, width, height, 0, 0)
}

func (s *helperSession) RequestRemoteClipboard() error {
	return s.writeFreeRDPEvent(freeRDPEventClipReq, 0, 0, 0, 0)
}

func (s *helperSession) ClipboardStats() ClipboardStats {
	s.mu.Lock()
	defer s.mu.Unlock()
	return ClipboardStats{Text: s.remoteClipboard, Sequence: s.clipboardSequence}
}

func (s *helperSession) ChannelMap() map[string]uint16 { return map[string]uint16{} }

func (s *helperSession) SecurityLabel() string {
	if s.engine == HelperEngineIronRDP {
		return "IronRDP helper（NLA/TLS 由 helper 处理）"
	}
	return "FreeRDP helper（" + certificateStateLabel(s.CertificateStatus().State) + "）"
}

func certificateStateLabel(state CertificateState) string {
	switch state {
	case CertificateStateTrusted:
		return "证书已固定：指纹与本机信任记录一致"
	case CertificateStateNeedTrust:
		return "证书待确认：本机还没有该主机的信任记录"
	case CertificateStateMismatch:
		return "证书已变更：与本机信任记录不一致"
	case CertificateStateInsecure:
		return "证书校验已跳过（排障模式）"
	}
	return "证书状态未知"
}

type helperInput struct {
	Type       string `json:"type"`
	X          int    `json:"x,omitempty"`
	Y          int    `json:"y,omitempty"`
	Button     int    `json:"button,omitempty"`
	Pressed    bool   `json:"pressed,omitempty"`
	Notches    int    `json:"notches,omitempty"`
	Horizontal bool   `json:"horizontal,omitempty"`
	ScanCode   int    `json:"scan_code,omitempty"`
	Extended   bool   `json:"extended,omitempty"`
}

func (s *helperSession) writeInput(input helperInput) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.isClosed() {
		return errors.New("会话已关闭")
	}
	if s.jsonInput {
		return s.writeJSONLineLocked(input)
	}
	switch input.Type {
	case "mouse_move":
		return s.writeFreeRDPEventLocked(freeRDPEventMove, input.X, input.Y, 0, 0)
	case "mouse_button":
		flags := uint32(0)
		if input.Pressed {
			flags |= 1
		}
		return s.writeFreeRDPEventLocked(freeRDPEventButton, input.X, input.Y, input.Button, flags)
	case "wheel":
		flags := uint32(0)
		if input.Horizontal {
			flags |= 2
		}
		return s.writeFreeRDPEventLocked(freeRDPEventWheel, input.X, input.Y, input.Notches, flags)
	case "key":
		flags := uint32(0)
		if input.Pressed {
			flags |= 1
		}
		if input.Extended {
			flags |= 2
		}
		return s.writeFreeRDPEventLocked(freeRDPEventKey, input.X, input.Y, input.ScanCode, flags)
	}
	return fmt.Errorf("不支持的输入事件 %q", input.Type)
}

func (s *helperSession) writeJSONLine(value any) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.isClosed() {
		return errors.New("会话已关闭")
	}
	return s.writeJSONLineLocked(value)
}

func (s *helperSession) writeJSONLineLocked(value any) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("编码 helper 输入失败: %w", err)
	}
	if err := s.writeBytesLocked(append(payload, '\n'), "写入 helper 输入"); err != nil {
		return err
	}
	return nil
}

func (s *helperSession) writeFreeRDPEvent(eventType uint32, x, y, value int, flags uint32) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.isClosed() {
		return errors.New("会话已关闭")
	}
	return s.writeFreeRDPEventLocked(eventType, x, y, value, flags)
}

func (s *helperSession) writeFreeRDPEventLocked(eventType uint32, x, y, value int, flags uint32) error {
	buffer := make([]byte, freeRDPEventBytes)
	binary.LittleEndian.PutUint32(buffer[0:4], eventType)
	binary.LittleEndian.PutUint32(buffer[4:8], uint32(x))
	binary.LittleEndian.PutUint32(buffer[8:12], uint32(y))
	binary.LittleEndian.PutUint32(buffer[12:16], uint32(value))
	binary.LittleEndian.PutUint32(buffer[16:20], flags)
	if err := s.writeBytesLocked(buffer, "写入 FreeRDP 输入"); err != nil {
		return err
	}
	return nil
}

func (s *helperSession) writeBytesLocked(payload []byte, action string) error {
	if s.stdin == nil {
		return errors.New("helper 输入管道不可用")
	}
	written, err := s.stdin.Write(payload)
	if err != nil {
		return fmt.Errorf("%s失败: %w", action, err)
	}
	if written != len(payload) {
		return fmt.Errorf("%s失败: %w", action, io.ErrShortWrite)
	}
	return nil
}

func (s *helperSession) isClosed() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closed
}

func environmentWithoutProxy(environment []string) []string {
	result := make([]string, 0, len(environment))
	for _, item := range environment {
		upper := strings.ToUpper(item)
		if strings.HasPrefix(upper, "HTTP_PROXY=") || strings.HasPrefix(upper, "HTTPS_PROXY=") ||
			strings.HasPrefix(upper, "ALL_PROXY=") {
			continue
		}
		result = append(result, item)
	}
	return result
}

func helperEnvironment(
	environment []string,
	fingerprint string,
	disableDynamicResolution bool,
	clipboard ClipboardSettings,
) []string {
	result := environmentWithoutProxy(environment)

	result = withHelperEnv(result, helperClipboardModeEnv, NormalizeClipboardSettings(clipboard).Mode)
	result = withHelperEnv(result, helperClipboardContentEnv, advertisedClipboardContent(clipboard))
	if disableDynamicResolution {

		filtered := make([]string, 0, len(result)+1)
		for _, item := range result {
			if strings.HasPrefix(strings.ToUpper(item), helperDynamicResolutionEnv+"=") {
				continue
			}
			filtered = append(filtered, item)
		}
		result = append(filtered, helperDynamicResolutionEnv+"=0")
	}
	normalized := NormalizeCertificateFingerprint(fingerprint)
	if normalized == "" {
		return result
	}
	filtered := make([]string, 0, len(result)+1)
	for _, item := range result {
		if strings.HasPrefix(strings.ToUpper(item), helperCertificateFingerprintEnv+"=") {
			continue
		}
		filtered = append(filtered, item)
	}
	return append(filtered, helperCertificateFingerprintEnv+"="+normalized)
}

func withHelperEnv(environment []string, key, value string) []string {
	prefix := strings.ToUpper(key) + "="
	filtered := make([]string, 0, len(environment)+1)
	for _, item := range environment {
		if strings.HasPrefix(strings.ToUpper(item), prefix) {
			continue
		}
		filtered = append(filtered, item)
	}
	return append(filtered, key+"="+value)
}

func asciiKeyStroke(r rune) (stroke keyStroke, ok bool) {
	if mapped, found := asciiScanCodes[r]; found {
		return mapped, true
	}
	return keyStroke{}, false
}

type keyStroke struct {
	scanCode uint16
	shift    bool
}

var asciiScanCodes = map[rune]keyStroke{
	'a': {0x1E, false}, 'b': {0x30, false}, 'c': {0x2E, false}, 'd': {0x20, false},
	'e': {0x12, false}, 'f': {0x21, false}, 'g': {0x22, false}, 'h': {0x23, false},
	'i': {0x17, false}, 'j': {0x24, false}, 'k': {0x25, false}, 'l': {0x26, false},
	'm': {0x32, false}, 'n': {0x31, false}, 'o': {0x18, false}, 'p': {0x19, false},
	'q': {0x10, false}, 'r': {0x13, false}, 's': {0x1F, false}, 't': {0x14, false},
	'u': {0x16, false}, 'v': {0x2F, false}, 'w': {0x11, false}, 'x': {0x2D, false},
	'y': {0x15, false}, 'z': {0x2C, false},
	'A': {0x1E, true}, 'B': {0x30, true}, 'C': {0x2E, true}, 'D': {0x20, true},
	'E': {0x12, true}, 'F': {0x21, true}, 'G': {0x22, true}, 'H': {0x23, true},
	'I': {0x17, true}, 'J': {0x24, true}, 'K': {0x25, true}, 'L': {0x26, true},
	'M': {0x32, true}, 'N': {0x31, true}, 'O': {0x18, true}, 'P': {0x19, true},
	'Q': {0x10, true}, 'R': {0x13, true}, 'S': {0x1F, true}, 'T': {0x14, true},
	'U': {0x16, true}, 'V': {0x2F, true}, 'W': {0x11, true}, 'X': {0x2D, true},
	'Y': {0x15, true}, 'Z': {0x2C, true},
	'0': {0x0B, false}, '1': {0x02, false}, '2': {0x03, false}, '3': {0x04, false},
	'4': {0x05, false}, '5': {0x06, false}, '6': {0x07, false}, '7': {0x08, false},
	'8': {0x09, false}, '9': {0x0A, false},
	')': {0x0B, true}, '!': {0x02, true}, '@': {0x03, true}, '#': {0x04, true},
	'$': {0x05, true}, '%': {0x06, true}, '^': {0x07, true}, '&': {0x08, true},
	'*': {0x09, true}, '(': {0x0A, true},
	' ': {0x39, false}, '\t': {0x0F, false}, '\n': {0x1C, false}, '\r': {0x1C, false},
	'-': {0x0C, false}, '_': {0x0C, true}, '=': {0x0D, false}, '+': {0x0D, true},
	'[': {0x1A, false}, '{': {0x1A, true}, ']': {0x1B, false}, '}': {0x1B, true},
	'\\': {0x2B, false}, '|': {0x2B, true}, ';': {0x27, false}, ':': {0x27, true},
	'\'': {0x28, false}, '"': {0x28, true}, '`': {0x29, false}, '~': {0x29, true},
	',': {0x33, false}, '<': {0x33, true}, '.': {0x34, false}, '>': {0x34, true},
	'/': {0x35, false}, '?': {0x35, true},
}
