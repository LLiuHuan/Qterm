//go:build ios

package main

import "github.com/wailsapp/wails/v3/pkg/application"

func modifyOptionsForIOS(opts *application.Options) {

	opts.DisableDefaultSignalHandler = true
}
