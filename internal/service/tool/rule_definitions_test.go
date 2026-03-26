package tool

import "testing"

func TestDefaultGlobalRulesForCodex(t *testing.T) {
	rules := DefaultGlobalRules(ToolTypeCodex)
	if len(rules) != 4 {
		t.Fatalf("expected 4 codex global rules, got %d", len(rules))
	}

	assertRulePath(t, rules, "config.toml", false)
	assertRulePath(t, rules, "skills", true)
	assertRulePath(t, rules, "rules", true)
	assertRulePath(t, rules, "superpowers", true)
}

func TestDefaultGlobalRulesForClaudePluginWhitelist(t *testing.T) {
	rules := DefaultGlobalRules(ToolTypeClaude)

	assertRulePath(t, rules, "settings.json", false)
	assertRulePath(t, rules, "CLAUDE.md", false)
	assertRulePath(t, rules, "commands", true)
	assertRulePath(t, rules, "skills", true)
	assertRulePath(t, rules, "output-styles", true)
	assertRulePath(t, rules, "plugins/blocklist.json", false)
	assertRulePath(t, rules, "plugins/installed_plugins.json", false)
	assertRulePath(t, rules, "plugins/known_marketplaces.json", false)

	for _, rule := range rules {
		if rule.Path == "plugins" {
			t.Fatalf("expected plugins directory not to be a default rule: %+v", rule)
		}
	}
}

func TestProjectRuleTemplates(t *testing.T) {
	codex := ProjectRuleTemplates(ToolTypeCodex)
	if len(codex) != 2 {
		t.Fatalf("expected 2 codex project templates, got %d", len(codex))
	}
	assertRulePath(t, codex, ".codex", true)
	assertRulePath(t, codex, "AGENTS.md", false)

	claude := ProjectRuleTemplates(ToolTypeClaude)
	if len(claude) != 2 {
		t.Fatalf("expected 2 claude project templates, got %d", len(claude))
	}
	assertRulePath(t, claude, ".claude", true)
	assertRulePath(t, claude, "CLAUDE.md", false)
}

func TestRequiredGlobalRulePaths(t *testing.T) {
	codex := RequiredGlobalRulePaths(ToolTypeCodex)
	if len(codex) != 1 || codex[0] != "config.toml" {
		t.Fatalf("unexpected codex required paths: %#v", codex)
	}

	claude := RequiredGlobalRulePaths(ToolTypeClaude)
	if len(claude) != 1 || claude[0] != "settings.json" {
		t.Fatalf("unexpected claude required paths: %#v", claude)
	}
}

func assertRulePath(t *testing.T, rules []SyncRuleDefinition, path string, isDir bool) {
	t.Helper()
	for _, rule := range rules {
		if rule.Path == path {
			if rule.IsDir != isDir {
				t.Fatalf("rule %s expected isDir=%v, got %+v", path, isDir, rule)
			}
			return
		}
	}
	t.Fatalf("expected rule path %s in %+v", path, rules)
}
