package main

import (
	"context"
	"fmt"
	"os"

	appruntime "ai-sync-manager/internal/app/runtime"
	"ai-sync-manager/internal/app/usecase"
	clicobra "ai-sync-manager/internal/cli/cobra"
	"ai-sync-manager/internal/service/setting"
	"ai-sync-manager/internal/service/tool"
	"ai-sync-manager/internal/tui"
)

const version = "1.0.0-alpha"

// runtimeOptions 收敛命令行入口使用的运行时默认配置。
func runtimeOptions() appruntime.Options {
	return appruntime.Options{
		Version:           version,
		DisableConsoleLog: true,
	}
}

// main 负责组装运行时依赖，并把 CLI/TUI 所需对象注入命令入口。
func main() {
	rt, err := appruntime.New(runtimeOptions())
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
	defer func() {
		_ = rt.Close()
	}()

	accessor := tool.NewConfigAccessor(rt.RuleResolver)
	aiSettingService := setting.NewAISettingService(rt.AISettingDAO)
	workflow := usecase.NewLocalWorkflow(rt.Detector, rt.SnapshotService, accessor).
		WithScanRuleManager(rt.RuleManager).
		WithAISettingManager(aiSettingService)
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
