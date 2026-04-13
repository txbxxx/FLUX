# Flux 重命名实施计划

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task.

**Goal:** 将项目从 `fl-manager` / `fl` 重命名为 `Flux` / `fl`

**Architecture:** 简单的全局文本替换 + 目录重命名
- 使用 `sed` 或 IDE 批量替换
- 按依赖顺序修改（先 import，再目录名）

**Tech Stack:** Go, Bash, sed/Git Bash

---

## 任务概览

| 任务 | 内容 | 影响范围 |
|------|------|----------|
| 1 | 修改 go.mod module 名 | 1 file |
| 2 | 批量替换 Go 文件 import 路径 | ~90 files |
| 3 | 重命名 cmd/fl 目录 | 目录 |
| 4 | 更新 Makefile | 1 file |
| 5 | 更新 CLAUDE.md | 1 file |
| 6 | 更新 README.md | 1 file |
| 7 | 更新中文文档命令引用 | ~20 files |
| 8 | 更新 AGENTS.md | 1 file |
| 9 | 验证构建 | - |
| 10 | 提交并推送 | - |

---

## Task 1: 修改 go.mod module 名

**Files:**
- Modify: `go.mod:1`

**Step 1: 修改 module 声明**

```diff
-module fl-manager
+module flux
```

**Step 2: 验证**

```bash
go mod tidy
```

**Step 3: 提交**

```bash
git add go.mod go.sum
git commit -m "rename: 修改 go.mod module 名为 flux"
```

---

## Task 2: 批量替换 Go 文件 import 路径

**Files:**
- Modify: 所有含 `fl-manager` 的 Go 文件 (~90 files)

**Step 1: 使用 sed 批量替换**

```bash
# 在 Git Bash / Linux 下执行
find . -name "*.go" -type f -exec sed -i 's|fl-manager|flux|g' {} \;
```

**Step 2: 验证无遗漏**

```bash
grep -r "fl-manager" --include="*.go" .
# 期望输出：无
```

**Step 3: 验证构建**

```bash
go build ./...
```

**Step 4: 提交**

```bash
git add -A
git commit -m "rename: 替换所有 Go import 路径为 flux"
```

---

## Task 3: 重命名 cmd/fl 目录

**Files:**
- Rename: `cmd/fl` → `cmd/fl`

**Step 1: 重命名目录**

```bash
git mv cmd/fl cmd/fl
```

**Step 2: 更新 main.go 中的 import（如需要）**

```go
// 检查并确保 import 路径正确
```

**Step 3: 验证构建**

```bash
go build ./cmd/fl
```

**Step 4: 提交**

```bash
git commit -m "rename: 重命名 CLI 入口目录 fl -> fl"
```

---

## Task 4: 更新 Makefile

**Files:**
- Modify: `Makefile`

**Step 1: 更新默认值**

```diff
-CLI_NAME ?= fl
+CLI_NAME ?= fl
```

**Step 2: 更新注释**

```diff
-# AI Sync Manager
+# Flux
```

**Step 3: 验证**

```bash
make build
ls bin/
```

**Step 4: 提交**

```bash
git add Makefile
git commit -m "rename: 更新 Makefile 默认 CLI 名为 fl"
```

---

## Task 5: 更新 CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

**需要更新的内容：**
- 项目概述中的名字
- 项目结构路径 `cmd/fl` → `cmd/fl`
- CLI 命令速查中的 `fl` → `fl`

**Step 1: 提交**

```bash
git add CLAUDE.md
git commit -m "rename: 更新 CLAUDE.md 中的项目名和路径"
```

---

## Task 6: 更新 README.md

**Files:**
- Modify: `README.md`

**需要更新的内容：**
- 标题 `# AI Sync Manager` → `# Flux`
- 所有 `fl` → `fl` 命令引用
- `cmd/fl` → `cmd/fl`
- Tagline 更新

**Step 1: 提交**

```bash
git add README.md
git commit -m "rename: 更新 README.md 中的项目名和命令"
```

---

## Task 7: 更新中文文档命令引用

**Files:**
- Modify: `docs/` 下所有中文文档

**需要更新的文件（约 20 个）：**
- docs/使用指南/*.md
- docs/测试指南/*.md
- docs/plans/*.md（部分）
- docs/架构设计/*.md（部分）

**Step 1: 批量替换**

```bash
find docs -name "*.md" -type f -exec sed -i 's/fl/fl/g' {} \;
```

**Step 2: 验证**

```bash
grep -r "fl" docs/ --include="*.md" | grep -v "flux" | head -20
```

**Step 3: 提交**

```bash
git add docs/
git commit -m "rename: 更新文档中的命令引用"
```

---

## Task 8: 更新 AGENTS.md

**Files:**
- Modify: `AGENTS.md`

**Step 1: 检查并更新**

```bash
grep "fl" AGENTS.md
# 替换为 fl
```

**Step 2: 提交**

```bash
git add AGENTS.md
git commit -m "rename: 更新 AGENTS.md"
```

---

## Task 9: 验证构建

**Step 1: 完整构建测试**

```bash
go build ./...
make build
./bin/fl --help
```

**Step 2: 运行测试**

```bash
go test ./...
```

**Step 3: 确认无遗漏**

```bash
grep -r "fl" . --include="*.go" --include="*.md" | grep -v "flux" | grep -v "docs/plans/2026-04-13-flux"
```

---

## Task 10: 提交并推送

**Step 1: 推送分支**

```bash
git push -u origin flux/rename
```

**Step 2: 创建 PR**

```bash
gh pr create --title "rename: 项目重命名为 Flux" --body "..."
```

---

## 执行选项

**1. Subagent-Driven (this session)** - 我dispatch fresh subagent per task, review between tasks, fast iteration

**2. Parallel Session (separate)** - Open new session with executing-plans, batch execution with checkpoints

**Which approach?**
