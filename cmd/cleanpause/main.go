package main

import (
	"cleanpause/internal/input"
	"cleanpause/internal/platform"
	"cleanpause/internal/ui"
	"os"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--input-worker" {
		os.Exit(input.Worker(os.Args[2:]))
	}
	simulate := false
	for _, a := range os.Args[1:] {
		if a == "--simulate-input" {
			simulate = true
		}
	}
	initial, configPath := "", ""
	if !simulate {
		elevated, err := platform.Elevated()
		if err != nil {
			ui.ShowError("Не удалось проверить права процесса: " + err.Error())
			return
		}
		if !elevated {
			if err := platform.RelaunchElevated(os.Args[1:]); err != nil {
				ui.ShowError(err.Error())
			}
			return
		}
	}
	for i, a := range os.Args[1:] {
		if a == "--show-settings" {
			initial = "settings"
		}
		if a == "--config" && i+2 < len(os.Args) {
			configPath = os.Args[i+2]
		}
	}
	if err := ui.Run(simulate, initial, configPath); err != nil {
		ui.ShowError(err.Error())
		os.Exit(1)
	}
}
