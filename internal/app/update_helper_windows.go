//go:build windows

package app

import (
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"changeme/internal/update"
)

const helperWaitLimit = 30 * time.Second

func applyUpdatePlan(plan update.UpdatePlan) error {
	if plan.Pid > 0 {
		waitForProcessExit(plan.Pid, helperWaitLimit)
	}
	helperLogger.log("INFO", "helper.apply.extract_start", "archive", plan.Archive, "target", plan.Target)
	if err := update.ApplyStagedArchive(plan.Archive, plan.Target); err != nil {

		helperLogger.log("ERROR", "helper.apply.extract_failed", "archive", plan.Archive, "target", plan.Target, "error", err)
		reportHelperFailure("安装更新失败", "无法用新版本替换当前程序：\n"+plan.Target+"\n\n请手动下载安装包。")
		return err
	}
	helperLogger.log("INFO", "helper.apply.extract_done", "target", plan.Target)

	if err := os.Remove(plan.Archive); err != nil && !os.IsNotExist(err) {
		helperLogger.log("WARN", "helper.apply.cleanup_failed", "path", plan.Archive, "error", err)
	}
	if err := os.Remove(plan.Archive + ".part"); err != nil && !os.IsNotExist(err) {
		helperLogger.log("WARN", "helper.apply.cleanup_failed", "path", plan.Archive+".part", "error", err)
	}

	if err := update.RecordPendingUpdate(plan.Target); err != nil {

		helperLogger.log("WARN", "helper.apply.pending_marker_failed", "target", plan.Target, "error", err)
	} else {
		helperLogger.log("INFO", "helper.apply.pending_marker_recorded", "target", plan.Target)
	}

	helperLogger.log("INFO", "helper.relaunch.start", "target", plan.Target)
	if err := relaunchTarget(plan.Target); err != nil {
		helperLogger.log("ERROR", "helper.relaunch.failed", "target", plan.Target, "error", err)
		reportHelperFailure("更新已完成，但启动失败", "新版本已经替换到位，请手动打开：\n"+plan.Target)
		return err
	}
	return nil
}

func waitForProcessExit(pid int, limit time.Duration) {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		helperLogger.log("INFO", "helper.wait.skip", "pid", pid, "reason", "open_process", "error", err)
		return
	}
	defer windows.CloseHandle(handle)
	started := time.Now()
	event, _ := windows.WaitForSingleObject(handle, uint32(limit/time.Millisecond))
	elapsed := time.Since(started)

	status := "signaled"
	if event == uint32(windows.WAIT_TIMEOUT) {
		status = "timeout"
	}
	helperLogger.log("INFO", "helper.wait.process_exit", "pid", pid, "status", status, "elapsed_ms", elapsed.Milliseconds())
}

func relaunchTarget(target string) error {
	asUserErr := relaunchTargetAsUser(target)
	if asUserErr == nil {
		helperLogger.log("INFO", "helper.relaunch.launched", "target", target, "method", "create_process_as_user")
		return nil
	}

	helperLogger.log("WARN", "helper.relaunch.fallback", "target", target, "method", "explorer_shell", "error", asUserErr)
	if err := relaunchViaShell(target); err != nil {
		return err
	}
	helperLogger.log("INFO", "helper.relaunch.launched", "target", target, "method", "explorer_shell")
	return nil
}

func relaunchTargetAsUser(target string) error {
	token, err := openShellToken()
	if err != nil {
		return err
	}
	defer token.Close()

	_ = enableAssignPrimaryTokenPrivilege()

	var environment *uint16
	if err := windows.CreateEnvironmentBlock(&environment, token, false); err != nil {

		return err
	}
	defer func() { _ = windows.DestroyEnvironmentBlock(environment) }()

	application, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	desktop, err := windows.UTF16PtrFromString(`winsta0\default`)
	if err != nil {
		return err
	}
	startup := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{}))}
	startup.Desktop = desktop
	startup.Flags = windows.STARTF_USESHOWWINDOW
	startup.ShowWindow = windows.SW_SHOWNORMAL

	var processInfo windows.ProcessInformation
	flags := uint32(windows.CREATE_UNICODE_ENVIRONMENT)
	if err := windows.CreateProcessAsUser(token, application, nil, nil, nil, false, flags, environment, nil, &startup, &processInfo); err != nil {
		return err
	}
	_ = windows.CloseHandle(processInfo.Thread)
	return windows.CloseHandle(processInfo.Process)
}

func openShellToken() (windows.Token, error) {
	shellWindow := windows.GetShellWindow()
	if shellWindow == 0 {
		return 0, errors.New("找不到用户外壳窗口")
	}
	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(shellWindow, &pid); err != nil {
		return 0, err
	}
	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(process)

	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, &token); err != nil {
		return 0, err
	}
	defer token.Close()

	var primary windows.Token
	if err := windows.DuplicateTokenEx(token, windows.TOKEN_ASSIGN_PRIMARY|windows.TOKEN_DUPLICATE|windows.TOKEN_QUERY, nil, windows.SecurityImpersonation, windows.TokenPrimary, &primary); err != nil {
		return 0, err
	}
	return primary, nil
}

func relaunchViaShell(target string) error {
	command := exec.Command("explorer.exe", target)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func reportHelperFailure(title, body string) {
	caption, _ := windows.UTF16PtrFromString(title)
	text, _ := windows.UTF16PtrFromString(body)
	_, _ = windows.MessageBox(0, text, caption, windows.MB_OK|windows.MB_ICONERROR)
}

func enableAssignPrimaryTokenPrivilege() error {
	process, err := windows.GetCurrentProcess()
	if err != nil {
		return err
	}
	var token windows.Token
	if err := windows.OpenProcessToken(process, windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return err
	}
	defer token.Close()

	name, err := windows.UTF16PtrFromString("SeAssignPrimaryTokenPrivilege")
	if err != nil {
		return err
	}
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, name, &luid); err != nil {
		return err
	}
	state := windows.Tokenprivileges{
		PrivilegeCount: 1,
		Privileges:     [1]windows.LUIDAndAttributes{{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}},
	}
	if err := windows.AdjustTokenPrivileges(token, false, &state, 0, nil, nil); err != nil {
		return err
	}

	if lastErr := windows.GetLastError(); lastErr != nil && lastErr != windows.ERROR_SUCCESS {
		if errno, ok := lastErr.(syscall.Errno); ok && errno == windows.ERROR_NOT_ALL_ASSIGNED {
			return errors.New("令牌里没有 SeAssignPrimaryTokenPrivilege")
		}
	}
	return nil
}
