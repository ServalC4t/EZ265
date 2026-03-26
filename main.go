package main

import (
	"fmt"
	"os"
	"strings"

	"h265conv/internal/encoder"
	"h265conv/internal/gui"
	"h265conv/internal/i18n"
	"h265conv/internal/ipc"
)

func main() {
	// Apply saved language preference before anything else
	settings := encoder.NewSettings()
	sd := settings.Load()
	switch sd.Language {
	case "ja":
		i18n.SetLang(i18n.LangJA)
	case "en":
		i18n.SetLang(i18n.LangEN)
	// "auto" or empty: keep OS-detected default
	}

	mode, files := parseArgs()

	// If another instance is running, send files and exit
	if len(files) > 0 && ipc.TryConnect(mode, files) {
		return
	}

	// Start IPC server for subsequent instances
	srv, _ := ipc.StartServer(func(m ipc.Mode, incoming []string) {
		if gui.Synchronize != nil {
			gui.Synchronize(m == ipc.ModeStart, incoming)
		}
	})
	if srv != nil {
		defer srv.Close()
	}

	// Launch GUI
	autoStart := mode == ipc.ModeStart && len(files) > 0
	if err := gui.Run(files, autoStart); err != nil {
		fmt.Fprintf(os.Stderr, "GUI error: %v\n", err)
		os.Exit(1)
	}
}

func parseArgs() (ipc.Mode, []string) {
	var files []string
	mode := ipc.ModeAdd
	args := os.Args[1:]

	for i := 0; i < len(args); i++ {
		arg := args[i]
		if strings.EqualFold(arg, "--encode") {
			mode = ipc.ModeStart
			if i+1 < len(args) {
				i++
				files = append(files, args[i])
			}
		} else if strings.EqualFold(arg, "--add") {
			mode = ipc.ModeAdd
			if i+1 < len(args) {
				i++
				files = append(files, args[i])
			}
		} else if !strings.HasPrefix(arg, "-") {
			files = append(files, arg)
		}
	}
	return mode, files
}
