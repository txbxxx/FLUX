# 快照对比功能 PRD

> 版本：v1.0 | 日期：2026-04-11

## 1. 背景与动机

快照恢复功能已完成（`snapshot restore`），用户可以在快照之间切换配置。但恢复前用户无法直观了解快照之间、或快照与当前系统之间的差异，导致不敢轻易恢复。

虽然 `restore --dry-run` 能预览恢复影响，但它只显示文件级别的"将被覆盖/跳过"，缺乏内容级差异展示。需要一个独立的对比工具，让用户在恢复前充分了解变更内容。

## 2. 目标用户

使用 AToSync 管理多台设备 AI 工具配置的用户，需要：
- 对比两个快照之间的配置差异
- 对比快照与当前磁盘状态之间的差异
- 在恢复前精确了解会变更哪些内容

## 3. 核心需求

### FR-01 快照对比命令

新增 `snapshot diff` 子命令，支持两种对比场景：

| 场景 | 命令 | 说明 |
|------|------|------|
| 快照 vs 快照 | `ai-sync snapshot diff <id1> <id2>` | 对比两个历史快照 |
| 快照 vs 文件系统 | `ai-sync snapshot diff <id>` | 对比快照与当前磁盘状态 |

参数个数自动推断对比类型，与 `git diff` 的体验一致。

### FR-02 分级输出

| 参数 | 输出内容 | 类比 |
|------|---------|------|
| （默认） | 摘要：变更文件列表 + 统计 | `git diff --stat` |
| `-v` | 内容级 diff：修改处 + 上下 5 行上下文 | `git diff -U5` |
| `-v --side-by-side` | 并排显示 | `git diff --side-by-side` |

**默认摘要模式**输出示例：

```
快照对比: my-config-v1 → my-config-v2

 claude/settings.json | 3 +--
 claude/CLAUDE.md     | 12 +++++++-----
 mcp/memory.json      | 5 +++--
 agents/review.md     | 8 ++++++++
 plugins/old.json     | 45 -------------------------
 5 files changed, 15 insertions(+), 50 deletions(-)
```

**`-v` 内容 diff 模式**输出示例：

```
diff --snapshot a/claude/settings.json b/claude/settings.json
--- a/claude/settings.json
+++ b/claude/settings.json
@@ -3,10 +3,10 @@
 {
   "version": 2,
-  "model": "sonnet",
+  "model": "opus",
   "tools": {
-    "temperature": 0.3
+    "temperature": 0.7
   }
 }
```

**`--side-by-side` 并排模式**：

- 自动检测终端宽度，左右各占一半
- 差异行用 `|` 标记并高亮显示
- 终端太窄时提示切换到上下对比模式
- 新增/删除文件只在一侧显示

### FR-03 过滤能力

| 参数 | 说明 | 示例 |
|------|------|------|
| `--tool` | 按工具类型过滤 | `--tool claude` 只看 claude 相关变更 |
| `--path` | 按路径模式过滤 | `--path "mcp/*"` 只看 mcp 目录下的变更 |

### FR-04 二进制文件处理

与 Git 行为一致：
- 摘要中正常显示（标明变更，带大小信息）
- 内容 diff 中只显示 `Binary files differ`，不做内容对比

### FR-05 颜色输出

标准 diff 着色：
- **绿色**：新增行（`+`）、新增文件名
- **红色**：删除行（`-`）、删除文件名
- **青色**：差异位置标记（`@@ ... @@`）
- **白色**：上下文行

支持 `--color <when>` 控制：`always`（始终着色）/ `auto`（检测终端，默认）/ `never`（不着色）

### FR-06 退出码

| 退出码 | 含义 |
|--------|------|
| 0 | 无差异 |
| 1 | 有差异 |
| 2 | 错误（快照不存在、参数错误等） |

退出码设计便于脚本集成。

### FR-07 restore --dry-run 改造

现有 `snapshot restore --dry-run` 增强：
- 复用 diff 的 `-v` 模式输出（修改处 + 上下 5 行上下文）
- 只做快照 vs 当前文件系统对比
- 末尾追加恢复摘要（覆盖/跳过文件数、备份路径）

## 4. 非目标

- 不支持 TUI 模式下的对比（后续版本考虑）
- 不支持三路对比（只对比两个对象）
- 不支持对比结果导出为文件
- 不支持忽略空白/空行等 diff 选项

## 5. 命令速查

```bash
# 摘要模式：快照 vs 快照
ai-sync snapshot diff abc123 def456

# 摘要模式：快照 vs 文件系统
ai-sync snapshot diff abc123

# 内容 diff
ai-sync snapshot diff abc123 def456 -v

# 并排显示
ai-sync snapshot diff abc123 def456 -v --side-by-side

# 按工具过滤
ai-sync snapshot diff abc123 def456 --tool claude

# 按路径过滤
ai-sync snapshot diff abc123 def456 --path "mcp/*"

# 组合使用
ai-sync snapshot diff abc123 --tool claude --path "settings*"

# 恢复预览（复用 diff 输出）
ai-sync snapshot restore abc123 --dry-run
```

## 6. 验收标准

| 场景 | 预期 |
|------|------|
| 两个无差异快照对比 | 退出码 0，输出"无差异" |
| 快照与文件系统一致 | 退出码 0，输出"无差异" |
| 默认摘要模式 | 显示变更文件列表和统计 |
| `-v` 内容 diff | 显示标准 unified diff，上下 5 行 |
| `--side-by-side` | 并排显示差异 |
| 二进制文件 | 摘要中显示，内容 diff 标记 Binary |
| `--tool` 过滤 | 只显示指定工具类型的文件变更 |
| `--path` 过滤 | 只显示匹配路径的文件变更 |
| 快照不存在 | 退出码 2，输出错误信息 |
| `restore --dry-run` | 显示 diff 内容 + 恢复摘要 |
