package app

import "github.com/wailsapp/wails/v3/pkg/application"

func newApplicationMenu() *application.Menu {
	menu := application.NewMenu()

	menu.AddRole(application.AppMenu)
	menu.AddRole(application.FileMenu)
	menu.AddRole(application.EditMenu)

	view := menu.AddSubmenu("View")
	view.AddRole(application.Reload)
	view.AddRole(application.ForceReload)

	view.AddRole(application.OpenDevTools)
	view.AddSeparator()
	view.AddRole(application.ToggleFullscreen)

	menu.AddRole(application.WindowMenu)
	menu.AddRole(application.HelpMenu)
	return menu
}
