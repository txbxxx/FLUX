package usecase

import (
	"context"
	"errors"
	"testing"

	"flux/internal/service/tool"
)

type stubConfigAccessor struct {
	resolveTool tool.ToolType
	resolvePath string
	resolveErr  error
	target      *tool.ConfigTarget

	listTarget *tool.ConfigTarget
	listErr    error
	listResult []tool.ConfigEntry

	readTarget  *tool.ConfigTarget
	readErr     error
	readContent string

	writeTarget  *tool.ConfigTarget
	writeContent string
	writeErr     error
}

func (s *stubConfigAccessor) Resolve(toolType tool.ToolType, relativePath string) (*tool.ConfigTarget, error) {
	s.resolveTool = toolType
	s.resolvePath = relativePath
	if s.resolveErr != nil {
		return nil, s.resolveErr
	}
	return s.target, nil
}

func (s *stubConfigAccessor) ListDir(target *tool.ConfigTarget) ([]tool.ConfigEntry, error) {
	s.listTarget = target
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.listResult, nil
}

func (s *stubConfigAccessor) ReadFile(target *tool.ConfigTarget) (string, error) {
	s.readTarget = target
	if s.readErr != nil {
		return "", s.readErr
	}
	return s.readContent, nil
}

func (s *stubConfigAccessor) WriteFile(target *tool.ConfigTarget, content string) error {
	s.writeTarget = target
	s.writeContent = content
	if s.writeErr != nil {
		return s.writeErr
	}
	return nil
}

func TestConfigBrowserReturnsDirectoryResult(t *testing.T) {
	accessor := &stubConfigAccessor{
		target: &tool.ConfigTarget{
			ToolType:     tool.ToolTypeCodex,
			RootPath:     "/home/test/.codex",
			RelativePath: "skills",
			AbsolutePath: "/home/test/.codex/skills",
			IsDir:        true,
		},
		listResult: []tool.ConfigEntry{
			{Name: "aiskills", RelativePath: "skills/aiskills", IsDir: true},
			{Name: "README.md", RelativePath: "skills/README.md", IsDir: false},
		},
	}

	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{}, accessor)

	result, err := workflow.GetConfig(context.Background(), GetConfigInput{
		Tool: "codex",
		Path: "skills",
	})
	if err != nil {
		t.Fatalf("expected directory result, got %v", err)
	}
	if accessor.resolveTool != tool.ToolTypeCodex || accessor.resolvePath != "skills" {
		t.Fatalf("unexpected resolve input: tool=%s path=%s", accessor.resolveTool, accessor.resolvePath)
	}
	if result.Kind != ConfigTargetDirectory {
		t.Fatalf("expected directory result, got %+v", result)
	}
	if len(result.Entries) != 2 || result.Entries[0].Name != "aiskills" {
		t.Fatalf("unexpected directory entries: %+v", result.Entries)
	}
}

func TestConfigBrowserReturnsFileResult(t *testing.T) {
	accessor := &stubConfigAccessor{
		target: &tool.ConfigTarget{
			ToolType:     tool.ToolTypeCodex,
			RootPath:     "/home/test/.codex",
			RelativePath: "skills/README.md",
			AbsolutePath: "/home/test/.codex/skills/README.md",
			IsDir:        false,
		},
		readContent: "# hello",
	}

	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{}, accessor)

	result, err := workflow.GetConfig(context.Background(), GetConfigInput{
		Tool: "codex",
		Path: "skills/README.md",
	})
	if err != nil {
		t.Fatalf("expected file result, got %v", err)
	}
	if result.Kind != ConfigTargetFile || result.Content != "# hello" {
		t.Fatalf("unexpected file result: %+v", result)
	}
	if result.Editable != true {
		t.Fatalf("expected file to be editable, got %+v", result)
	}
}

func TestConfigBrowserRejectsDirectoryEditMode(t *testing.T) {
	accessor := &stubConfigAccessor{
		target: &tool.ConfigTarget{
			ToolType:     tool.ToolTypeCodex,
			RelativePath: "skills",
			AbsolutePath: "/home/test/.codex/skills",
			IsDir:        true,
		},
	}

	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{}, accessor)

	_, err := workflow.GetConfig(context.Background(), GetConfigInput{
		Tool: "codex",
		Path: "skills",
		Edit: true,
	})
	if err == nil {
		t.Fatal("expected directory edit mode to be rejected")
	}

	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected UserError, got %T", err)
	}
	if userErr.Message != "目录无法编辑，请指定文件" {
		t.Fatalf("unexpected user message: %q", userErr.Message)
	}
}

func TestConfigBrowserMapsAccessorErrors(t *testing.T) {
	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{}, &stubConfigAccessor{
		resolveErr: errors.New("请求路径超出允许范围"),
	})

	_, err := workflow.GetConfig(context.Background(), GetConfigInput{
		Tool: "codex",
		Path: "skills/../../secret.txt",
	})
	if err == nil {
		t.Fatal("expected mapped accessor error")
	}

	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected UserError, got %T", err)
	}
	if userErr.Message != "读取配置失败" {
		t.Fatalf("unexpected user message: %q", userErr.Message)
	}
	if userErr.Suggestion == "" {
		t.Fatal("expected suggestion for accessor failure")
	}
}

func TestConfigBrowserSaveConfigWritesContent(t *testing.T) {
	accessor := &stubConfigAccessor{
		target: &tool.ConfigTarget{
			ToolType:     tool.ToolTypeCodex,
			RelativePath: "skills/README.md",
			AbsolutePath: "/home/test/.codex/skills/README.md",
			IsDir:        false,
		},
	}

	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{}, accessor)

	err := workflow.SaveConfig(context.Background(), SaveConfigInput{
		Tool:    "codex",
		Path:    "skills/README.md",
		Content: "# updated",
	})
	if err != nil {
		t.Fatalf("expected save to succeed, got %v", err)
	}
	if accessor.writeTarget == nil || accessor.writeTarget.RelativePath != "skills/README.md" {
		t.Fatalf("expected resolved write target, got %+v", accessor.writeTarget)
	}
	if accessor.writeContent != "# updated" {
		t.Fatalf("unexpected write content: %q", accessor.writeContent)
	}
}

func TestConfigBrowserSaveConfigMapsWriteErrors(t *testing.T) {
	accessor := &stubConfigAccessor{
		target: &tool.ConfigTarget{
			ToolType:     tool.ToolTypeCodex,
			RelativePath: "skills/README.md",
			AbsolutePath: "/home/test/.codex/skills/README.md",
			IsDir:        false,
		},
		writeErr: errors.New("disk full"),
	}

	workflow := NewLocalWorkflow(&stubDetector{}, &stubSnapshotService{}, accessor)

	err := workflow.SaveConfig(context.Background(), SaveConfigInput{
		Tool:    "codex",
		Path:    "skills/README.md",
		Content: "# updated",
	})
	if err == nil {
		t.Fatal("expected save error")
	}

	var userErr *UserError
	if !errors.As(err, &userErr) {
		t.Fatalf("expected UserError, got %T", err)
	}
	if userErr.Message != "无法保存配置" {
		t.Fatalf("unexpected user message: %q", userErr.Message)
	}
}
