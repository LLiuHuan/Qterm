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
	if err := update.ApplyStagedArchive(plan.Archive, plan.Target); err != nil {
		reportHelperFailure("安装更新失败", "无法用新版本替换当前程序：\n"+plan.Target+"\n\n请手动下载安装包。")
		return err
	}

	_ = os.Remove(plan.Archive)
	_ = os.Remove(plan.Archive + ".part")

	_ = update.RecordPendingUpdate(plan.Target)

	if err := relaunchTarget(plan.Target); err != nil {
		reportHelperFailure("更新已完成，但启动失败", "新版本已经替换到位，请手动打开：\n"+plan.Target)
		return err
	}
	return nil
}

func waitForProcessExit(pid int, limit time.Duration) {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return
	}
	defer windows.CloseHandle(handle)
	_, _ = windows.WaitForSingleObject(handle, uint32(limit/time.Millisecond))
}

func relaunchTarget(target string) error {
	if err := relaunchTargetAsUser(target); err == nil {
		return nil
	}
	return relaunchViaShell(target)
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
