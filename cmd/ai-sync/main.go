package main

import (
	"context"
	"fmt"
	"os"

	appruntime "ai-sync-manager/internal/app/runtime"
	"ai-sync-manager/internal/app/usecase"
	clicobra "ai-sync-manager/internal/cli/cobra"
	"ai-sync-manager/internal/service/tool"
	"ai-sync-manager/internal/tui"
)

const version = "1.0.0-alpha"

func runtimeOptions() appruntime.Options {
	return appruntime.Options{
		Version:           version,
		DisableConsoleLog: true,
	}
}

func main() {
	rt, err := appruntime.New(runtimeOptions())
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer func() {
		_ = rt.Close()
	}()

	accessor := tool.NewConfigAccessor()
	workflow := usecase.NewLocalWorkflow(rt.Detector, rt.SnapshotService, accessor)
	runner := tui.NewRunner(workflow, rt.DataDir, os.Stdin, os.Stdout)
	editor := tui.NewConfigEditor(os.Stdin, os.Stdout)

	exitCode := clicobra.Execute(clicobra.Dependencies{
		Workflow: workflow,
		TUI:      runner,
		Editor:   editor,
		DataDir:  rt.DataDir,
		Out:      os.Stdout,
		Err:      os.Stderr,
		Context:  context.Background(),
	}, os.Args[1:])
	os.Exit(exitCode)
}
