package app

import (
	"errors"

	wailsapp "github.com/wailsapp/wails/v3/pkg/application"
)

func ConfigureSessionDialogs(service *Service, manager *wailsapp.DialogManager, environment *wailsapp.EnvironmentManager) {
	if service == nil || service.sftp == nil {
		return
	}
	if manager == nil {

		service.fileDialogs = nil
		service.sftp.SetDialogs(nil)
		return
	}
	service.fileDialogs = &appFileDialogs{manager: manager, environment: environment}
	service.sftp.SetDialogs(service.fileDialogs)
}

type appFileDialogs struct {
	manager *wailsapp.DialogManager

	environment *wailsapp.EnvironmentManager
}

func (d *appFileDialogs) ChooseUploadFile() (string, error) {
	return d.manager.OpenFile().SetTitle("选择要上传的文件").SetButtonText("上传").PromptForSingleSelection()
}
func (d *appFileDialogs) ChooseDownloadFile(filename string) (string, error) {
	return d.manager.SaveFileWithOptions(&wailsapp.SaveFileDialogOptions{Title: "保存远程文件", Filename: filename, ButtonText: "保存", CanCreateDirectories: true, AllowOtherFileTypes: true}).PromptForSingleSelection()
}

func (d *appFileDialogs) ChooseSessionHistoryTextFile(filename string) (string, error) {
	return d.manager.SaveFileWithOptions(&wailsapp.SaveFileDialogOptions{
		Title:                "导出会话记录",
		Filename:             filename,
		ButtonText:           "导出",
		CanCreateDirectories: true,
		AllowOtherFileTypes:  true,
		Filters:              []wailsapp.FileFilter{{DisplayName: "文本文件 (*.txt)", Pattern: "*.txt"}},
	}).PromptForSingleSelection()
}

func (d *appFileDialogs) ChooseTerminalRecordingExportDirectory() (string, error) {
	return d.manager.OpenFile().SetTitle("导出终端录制").SetButtonText("导出").CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
}

func (d *appFileDialogs) ChooseResourceBundleDestination() (string, error) {
	return d.manager.SaveFileWithOptions(&wailsapp.SaveFileDialogOptions{
		Title:                "导出 Qterm 资产",
		Filename:             "qterm-assets.qtrb",
		ButtonText:           "导出",
		CanCreateDirectories: true,
		AllowOtherFileTypes:  true,
		Filters:              []wailsapp.FileFilter{{DisplayName: "Qterm 资产包 (*.qtrb)", Pattern: "*.qtrb"}},
	}).PromptForSingleSelection()
}

func (d *appFileDialogs) ChooseResourceBundleFile() (string, error) {
	return d.manager.OpenFile().SetTitle("导入 Qterm 资产").SetButtonText("导入").CanChooseFiles(true).CanChooseDirectories(false).PromptForSingleSelection()
}

func (d *appFileDialogs) ChooseHostImportFile(title string) (string, error) {
	return d.manager.OpenFile().
		SetTitle(title).
		SetButtonText("选择").
		CanChooseFiles(true).
		CanChooseDirectories(false).
		PromptForSingleSelection()
}

func (d *appFileDialogs) ChooseHostImportDirectory(title string) (string, error) {
	return d.manager.OpenFile().
		SetTitle(title).
		SetButtonText("选择").
		CanChooseFiles(false).
		CanChooseDirectories(true).
		PromptForSingleSelection()
}

func (d *appFileDialogs) ChooseDebugLogDestination() (string, error) {
	return d.manager.SaveFileWithOptions(&wailsapp.SaveFileDialogOptions{
		Title:                "导出 Qterm 调试日志",
		Filename:             "qterm-debug.log",
		ButtonText:           "导出",
		CanCreateDirectories: true,
		AllowOtherFileTypes:  true,
	}).PromptForSingleSelection()
}

func (d *appFileDialogs) ChooseUploadDirectory() (string, error) {
	return d.manager.OpenFile().SetTitle("选择要上传的文件夹").SetButtonText("上传").CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
}

func (d *appFileDialogs) ChooseLocalDirectory() (string, error) {
	return d.manager.OpenFile().SetTitle("打开本地目录").SetButtonText("打开").CanChooseDirectories(true).CanChooseFiles(false).PromptForSingleSelection()
}

func (d *appFileDialogs) OpenPath(path string) error {
	if d == nil || d.environment == nil {
		return errors.New("environment manager is not ready")
	}
	return d.environment.OpenFileManager(path, false)
}
