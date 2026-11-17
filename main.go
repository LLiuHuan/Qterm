package main

import (
	"embed"
	"fmt"
	"log"
	"os"
	"runtime/debug"

	qtermapp "changeme/internal/app"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	defer func() {
		if recovered := recover(); recovered != nil {
			_, _ = fmt.Fprintf(os.Stderr, "qterm: panic: %v\n%s\n", recovered, debug.Stack())
			os.Exit(1)
		}
	}()

	if handled, err := qtermapp.RunUpdateHelperIfRequested(os.Args[1:]); handled {
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "qterm: 更新助手失败：%v\n", err)
			os.Exit(1)
		}
		return
	}
	err := qtermapp.Run(assets)
	if err != nil {
		log.Fatal(err)
	}
	_, _ = fmt.Fprintln(os.Stderr, "qterm: 应用主循环已退出（Run 返回 nil）")
}
