package app

import (
	"changeme/internal/model"
	"changeme/internal/session"

	wailsapp "github.com/wailsapp/wails/v3/pkg/application"
)

func ConfigureRuntime(service *Service, application *wailsapp.App) {
	if service == nil || application == nil {
		return
	}
	ConfigureSessionDialogs(service, application.Dialog, application.Env)
	RegisterSessionStreams(application, service)

	RegisterRdpStreams(application, service)

	RegisterVncStreams(application, service)

	if service.sessions != nil {
		service.sessions.SetForwardEmitter(func(status model.ForwardStatus) {
			application.Event.Emit(session.ForwardStatusEvent, status)
		})
	}
	if service.remoteEdits != nil {
		service.remoteEdits.SetEventEmitter(func(name string, payload any) {
			application.Event.Emit(name, payload)
		})
	}
}
