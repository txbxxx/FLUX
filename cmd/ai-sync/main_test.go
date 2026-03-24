package main

import "testing"

func TestRuntimeOptionsDisableConsoleLogs(t *testing.T) {
	options := runtimeOptions()

	if !options.DisableConsoleLog {
		t.Fatal("expected CLI runtime options to disable console logs")
	}
}
