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
	err := qtermapp.Run(assets)
	if err != nil {
		log.Fatal(err)
	}
	_, _ = fmt.Fprintln(os.Stderr, "qterm: 应用主循环已退出（Run 返回 nil）")
}
