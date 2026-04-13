# AI Sync Manager 命令行与终端界面使用指南

## 1. 文档目标

本文档面向当前终端化版本的 AI Sync Manager，说明以下内容：

- 项目当前能做什么
- 如何构建和运行 CLI
- 各命令的用途、参数和输出格式
- 如何使用 `fl tui` 进入终端交互界面
- 常见报错的定位思路

当前版本是本地优先 MVP，只覆盖本地扫描与本地快照流程，不包含远端同步能力。

## 2. 当前能力范围

当前版本支持：

- 扫描本机 AI 工具配置
- 浏览工具配置目录与文件
- 在终端内直接编辑配置文件
- 创建本地配置快照
- 列出本地已创建快照
- 通过终端交互界面执行基础操作

当前版本不支持：

- 远端仓库同步
- push、pull、diff、apply、rollback
- GUI 桌面界面
- 凭证管理界面

## 3. 运行前提

建议准备以下环境：

- Go 1.25 或兼容版本
- 本机存在至少一个可识别的 AI 工具配置目录

当前扫描逻辑主要面向：

- `~/.codex`
- `~/.claude`

如果这些目录不存在，`scan` 可能会返回“未检测到任何可同步工具”。

## 4. 构建与启动

### 4.1 直接运行源码

```powershell
go run -buildvcs=false ./cmd/fl <command>
```

示例：

```powershell
go run -buildvcs=false ./cmd/fl scan
```

### 4.2 构建为二进制

```powershell
make build
```

构建完成后可以直接执行：

```powershell
.\bin\fl.exe <command>
```

如果你想修改 CLI 名称，例如生成 `sync-tool.exe`：

```powershell
make build CLI_NAME=sync-tool
.\bin\sync-tool.exe <command>
```

如果你使用的是非 Windows 环境，可以继续直接使用 `go build`，或者在 `make` 中显式去掉扩展名：

```bash
make build CLI_NAME=sync-tool GOEXE=
./bin/sync-tool scan
```

如果你想直接构建并运行 CLI，可以使用：

```powershell
make run
make run ARGS=scan
```

其中 `make run` 默认会执行 `.\bin\fl.exe --help`。

## 5. 命令总览

当前 Cobra 命令树包含以下入口：

- `fl scan`
- `fl scan add`
- `fl scan remove`
- `fl scan list`
- `fl scan rules`
- `fl get`
- `fl setting`
- `fl setting create`
- `fl setting list`
- `fl setting get`
- `fl setting delete`
- `fl setting switch`
- `fl snapshot create`
- `fl snapshot list`
- `fl snapshot delete`
- `fl snapshot restore`
- `fl tui`
- `fl completion`

查看根帮助：

```powershell
.\fl.exe --help
```

当前帮助输出结构如下：

```text
AI tool config snapshot manager

Usage:
  fl [flags]
  fl [command]

Available Commands:
  completion
  get
  help
  scan
  snapshot
  tui
```

## 6. `scan`：扫描本地工具配置

### 6.1 命令

```powershell
.\fl.exe scan
.\fl.exe scan codex
.\fl.exe scan claude
.\fl.exe scan codex claude
.\fl.exe scan fl-manager
.\fl.exe scan list
.\fl.exe scan list claude
.\fl.exe scan rules
.\fl.exe scan rules claude
.\fl.exe scan add claude C:\Users\<user>\.claude.json
.\fl.exe scan add --project codex demo D:\workspace\demo
```

### 6.2 功能说明

`scan` 现在基于统一规则源工作，会输出两类同级扫描对象：

- 全局扫描对象
- 已注册项目扫描对象

其中过滤参数既可以是应用名，也可以是已注册项目名：

- `scan codex` 会输出 Codex 全局对象和所有 Codex 项目对象
- `scan fl-manager` 会只输出名为 `fl-manager` 的已注册项目对象

输出信息包括：

- 扫描对象名称
- 检测结果
- 配置目录
- 可同步项数量
- 原因
- 关键配置与扩展内容

### 6.3 输出格式

典型输出格式如下：

```text
Codex（全局）
  检测结果: 可同步
  配置目录: C:\Users\<user>\.codex
  可同步项: 4 项

  关键配置:
    - 主配置: config.toml

  扩展内容:
    - 技能目录: skills\
    - 规则目录: rules\
    - 超能力目录: superpowers\
```

### 6.4 状态说明

- `可同步`
  已找到关键文件，并识别到至少一项最终规则命中项
- `暂不可同步`
  找到了配置目录，但缺少关键文件，或当前最终规则下没有命中项
- `不可同步`
  没找到对应工具的配置目录

如果不可同步，输出里会直接说明原因，例如：

```text
Claude
  检测结果: 暂不可同步
  配置目录: C:\Users\<user>\.claude
  原因: 找到了配置目录，但未识别到可同步的配置文件
```

### 6.5 常见失败

如果没有检测到任何工具，CLI 会返回友好错误，例如：

```text
未检测到任何可同步工具
请先确认 ~/.codex 或 ~/.claude 是否存在
```

这类报错通常意味着：

- 本机尚未安装对应工具
- 配置目录不存在
- 当前用户无法访问目标目录

### 6.6 `scan add/remove/list/rules`

当前支持两类规则操作：

```powershell
.\fl.exe scan add <app> <absolute-file-path>
.\fl.exe scan remove <app> <absolute-file-path>
.\fl.exe scan add --project <app> <project-name> <project-absolute-path>
.\fl.exe scan remove --project <app> <project-absolute-path>
.\fl.exe scan list
.\fl.exe scan list <app-or-project>
.\fl.exe scan rules
.\fl.exe scan rules <app-or-project>
```

说明：

- 默认规则由程序内置，不需要手动添加
- `scan add <app> <absolute-file-path>` 用于补充分散在其他位置的配置文件
- `scan add --project ...` 用于注册项目，注册后会按该工具的项目规则模板生成独立的项目扫描对象，并参与扫描和快照
- `scan` 和 `scan list` 都用于查看当前扫描结果
- `scan rules` 用于查看默认全局规则、自定义绝对路径规则和已注册项目
- 只有存在已注册项目时，`scan rules` 才会额外显示“已注册项目扫描模板”

如果某个已注册项目下实际识别到了配置文件，`scan` 会把它作为与全局对象同级的结果输出，例如：

```text
demo（Codex 项目）
  检测结果: 可同步
  配置目录: D:\workspace\demo
  可同步项: 1 项

  关键配置:
    - 代理规则: AGENTS.md
```

这表示该条结果是项目级扫描对象，不会再并入 `Codex（全局）` 或 `Claude（全局）` 的计数中。

## 7. `get`：查看或编辑配置内容

### 7.1 查看目录

```powershell
.\fl.exe get codex skills\
```

这个命令会：

- 以 `Codex` 配置根目录为起点
- 找到 `skills\`
- 只列出当前这一层的目录和文件

典型输出如下：

```text
Codex > skills

类型    名称                 路径
目录    aiskills             skills\aiskills\
文件    README.md            skills\README.md
```

如果你想继续向下看某一层，继续输入更深一级路径即可，例如：

```powershell
.\fl.exe get codex skills\aiskills\
```

### 7.2 查看文件内容

```powershell
.\fl.exe get codex skills\aiskills\README.md
```

这个命令会：

- 自动判断目标是文件还是目录
- 如果是文件，直接把文件完整内容输出到 stdout

典型输出如下：

```text
Codex > skills\aiskills\README.md

<文件正文原样输出>
```

### 7.3 进入编辑模式

```powershell
.\fl.exe get codex skills\aiskills\README.md --edit
```

`--edit` 的行为是：

- 进入全屏终端编辑界面
- 显示行号
- 支持滚动
- 保存后直接修改本机原文件

当前编辑模式的基础快捷键为：

- `Ctrl+S`：保存
- `Esc`：退出
- `Ctrl+C`：强制退出

### 7.4 路径规则

`get` 不再是固定白名单，而是严格跟随统一规则源。

当前支持两种输入方式：

- 默认全局规则路径：使用相对于工具全局配置目录的相对路径
- 已登记规则路径：使用已登记的绝对路径

例如：

- `get codex skills\`
  实际针对 `C:\Users\<user>\.codex\skills`
- `get claude commands\`
  实际针对 `C:\Users\<user>\.claude\commands`

例如：

- `get codex skills\`
- `get claude settings.json`
- `get claude C:\Users\<user>\.claude.json`

当前只允许读取“最终规则命中的路径”，不允许越界访问未登记的系统其他文件。

### 7.5 常见失败

典型失败包括：

- 工具名不支持
- 请求路径不存在
- 请求路径超出允许配置范围
- 对目录使用了 `--edit`
- 目标文件是二进制文件

---

## 8. `setting`：管理 Claude AI 配置

### 8.1 命令概览

```powershell
.\fl.exe setting create --name <name> --token <token> --api <api> [--opus-model <model>] [--sonnet-model <model>]
.\fl.exe setting list [--limit <n>] [--offset <n>]
.\fl.exe setting get <name> [name...]
.\fl.exe setting delete <name> [name...]
.\fl.exe setting switch <name>
.\fl.exe setting edit <name> [flags]
```

### 8.2 setting create：创建配置

创建新的 AI 配置并保存到本地数据库：

```powershell
.\fl.exe setting create --name glm3 --token sk-ant-xxx --api https://api.anthropic.com --opus-model claude-3-opus-20240229 --sonnet-model claude-3-sonnet-20240229
```

**必填参数**：
- `--name`：配置名称（必填）
- `--token`：Auth token（必填）
- `--api`：API base URL（必填）

**可选参数**：
- `--opus-model`：Opus 模型（可选）
- `--sonnet-model`：Sonnet 模型（可选）
- 至少需要指定一个模型

### 8.3 setting list：列出配置

列出所有已保存的配置，当前生效的配置会显示 `*` 标记：

```powershell
.\fl.exe setting list
.\fl.exe setting list --limit 10 --offset 0
```

### 8.4 setting get：获取配置详情（支持批量）

**单个获取**：
```powershell
.\fl.exe setting get glm3
```

**批量获取**：
```powershell
.\fl.exe setting get glm3 glm5 test_ok
```

批量获取时：
- 每个配置用 `---` 分隔
- 最后显示汇总（成功 X 个，失败 Y 个）
- 不存在的配置会列在"获取失败的配置"中

**输出示例**：
```text
配置: glm3
配置名称: glm3
配置 ID: 1234567890
Token: sk-a****xxxx
Base URL: https://api.anthropic.com
Opus 模型: claude-3-opus-20240229
当前生效: 是
---
配置: glm5
配置名称: glm5
配置 ID: 1234567891
Token: sk-a****yyyy
Base URL: https://api.anthropic.com
Sonnet 模型: claude-3-sonnet-20240229
当前生效: 否

获取失败的配置: notexist

汇总：成功 2 个，失败 1 个（共 3 个）
```

### 8.5 setting delete：删除配置（支持批量）

**单个删除**：
```powershell
.\fl.exe setting delete glm3
```

**批量删除**（带确认）：
```powershell
.\fl.exe setting delete glm3 glm5
```

批量删除时：
1. 先显示将要删除的配置列表
2. 提示用户确认（输入 `y` 或 `yes` 确认）
3. 确认后执行删除，显示汇总结果

**输出示例**：
```text
将删除以下配置：
- glm3
- glm5
确认删除？(y/yes): y
已删除: glm3, glm5
汇总：成功 2 个，失败 0 个（共 2 个）
```

### 8.6 setting switch：切换配置

切换到指定的配置，将配置写入 `~/.claude/settings.json`：

```powershell
.\fl.exe setting switch glm3
```

### 8.7 setting edit：编辑配置

编辑已保存的 AI 配置，支持命令行参数和编辑器两种交互模式。

#### 命令行参数模式

通过参数直接修改配置：

```powershell
# 编辑单个字段
.\fl.exe setting edit glm3 -t sk-ant-new-token
.\fl.exe setting edit glm3 -a https://new-api.example.com

# 编辑多个字段
.\fl.exe setting edit glm3 -a https://new-api.example.com -o claude-3-opus-20240229

# 修改配置名称
.\fl.exe setting edit glm3 -n glm3-updated
```

**参数说明**：

| 参数 | 简写 | 说明 |
|------|------|------|
| `--name` | `-n` | 新名称（可选） |
| `--token` | `-t` | 新 Token（可选） |
| `--api` | `-a` | 新 API 地址（可选） |
| `--opus-model` | `-o` | 新 Opus 模型（可选） |
| `--sonnet-model` | `-s` | 新 Sonnet 模型（可选） |

未指定的字段将保持原值。

**输出示例**：
```text
配置已更新: glm3

变更内容：
- token: sk-a****xxxx → sk-a****yyyy （已修改）
更新时间: 2024-04-10T14:30:00+08:00
```

#### 编辑器模式

使用外部编辑器可视化编辑配置：

```powershell
# 使用默认 YAML 格式
.\fl.exe setting edit glm3 -e

# 使用 JSON 格式
.\fl.exe setting edit glm3 -e -f json
```

编辑器模式会：
1. 导出当前配置到临时文件（YAML 或 JSON 格式）
2. 自动打开系统编辑器（Windows 默认 notepad，Linux 默认 vim）
3. 保存后自动解析并应用变更

**临时文件格式**：

YAML 格式示例：
```yaml
# AI 配置编辑
#
# 重要：请直接保存（Ctrl+S），不要使用"另存为"
# 字段值为 <unchanged> 时保持原值
# 留空（空字符串）表示清空该字段
#
name: glm3
token: <unchanged>
base_url: https://api.anthropic.com
opus_model: claude-3-opus-20240229
sonnet_model: claude-3-sonnet-20240229
```

JSON 格式示例：
```json
{
  "name": "glm3",
  "token": "<unchanged>",
  "base_url": "https://api.anthropic.com",
  "opus_model": "claude-3-opus-20240229",
  "sonnet_model": "claude-3-sonnet-20240229"
}
```

**字段说明**：

| 字段值 | 含义 |
|--------|------|
| `<unchanged>` | 保持原值不变 |
| 空字符串 `""` | 清空该字段 |
| 其他值 | 更新为新值 |

> **注意**：`token` 字段默认显示为 `<unchanged>`，敏感信息不会显示在编辑器中。

**自定义编辑器**：

通过设置 `EDITOR` 环境变量指定编辑器：
```powershell
# Windows
set EDITOR=code
.\fl.exe setting edit glm3 -e

# Linux/macOS
export EDITOR=nano
./fl setting edit glm3 -e
```

---

## 9. `snapshot create`：创建本地快照

### 8.1 最小用法

必须指定 `--project`（项目名称）和 `--message`（备份说明）：

```powershell
.\fl.exe snapshot create --project codex-global --message “backup before change”
```

> `--tools` 参数可以省略，系统会从项目名称自动推导工具类型。

### 8.2 完整示例

```powershell
.\fl.exe snapshot create `
  --project codex-global `
  --tools codex,claude `
  --message “backup before change” `
  --name “Daily backup”
```

### 8.3 参数说明

`snapshot create --help` 当前输出的参数如下：

```text
-t, --tools string     指定要备份的工具，多个用逗号分隔（如 codex,claude）
-m, --message string   快照说明（必填）
-n, --name string      快照名称（可选）
-p, --project string   项目名称（必填，如 codex-global、claude-global 或用户注册的项目）
```

含义说明：

- `-p / --project`
  项目名称（必填），指定要备份哪个项目的配置。可用项目包括：
  - 全局项目：`codex-global`（~/.codex）、`claude-global`（~/.claude）
  - 用户项目：通过 `scan add --project` 注册的项目
- `-t / --tools`
  逗号分隔的工具列表，例如 `codex`、`claude`。省略时从项目名称自动推导
- `-m / --message`
  本次快照说明，建议写明用途，例如”升级前备份”
- `-n / --name`
  快照显示名称，可选，不填则自动生成

### 8.4 参数约束

当前实现中，`--project` 是必填参数。`--tools` 可省略（从项目名自动推导）。

### 8.5 成功输出

成功创建后，输出格式如下：

```text
快照已创建: 12345678-1234-1234-1234-1234567890ab
名称: Daily backup
文件数: 8
大小: 4096 字节
```

字段说明：

- `快照已创建`：新建快照 ID
- `名称`：快照名称
- `文件数`：收集到的文件数量
- `大小`：快照总大小

### 8.6 失败场景

常见失败包括：

- 未指定项目名称（`--project`）
- `message` 为空
- 目标项目存在，但没有可打包的配置文件
- 项目名称不存在

例如当 `scan` 输出”可同步项: 0 项”时，后续执行 `snapshot create` 很可能失败，因为当前统一规则下没有可归档内容。

## 10. `snapshot list`：列出本地快照

### 10.1 基础用法

```powershell
.\fl.exe snapshot list
```

### 10.2 分页参数

```powershell
.\fl.exe snapshot list --limit 20 --offset 0
```

当前帮助输出如下：

```text
-l, --limit int
-o, --offset int
```

参数含义：

- `-l / --limit`
  最多返回多少条记录
（0 表示不限制）
- `-o / --offset`
  从第几条之后开始返回

### 10.3 有结果时的输出

以表格形式展示快照信息：

```
ID                  | 名称           | 项目        | 说明       | 文件数 | 创建时间
550e8400-...       | 升级前备份      | codex-global | 升级前备份 | 8       | 2026-04-11 14:30
660e8400-...       | 自动备份        | claude-global | 每周备份   | 5       | 2026-04-10 09:15
```

表格包含以下列：

| 列名 | 说明 |
|------|------|
| ID | 快照唯一标识符（UUID 前缀） |
| 名称 | 用户指定的快照名称 |
| 项目 | 关联的项目名称（如 `codex-global`、`claude-global` 或用户项目） |
| 说明 | 快照创建时填写的说明信息 |
| 文件数 | 该快照包含的文件数量 |
| 创建时间 | 快照创建时间 |

底部还会显示总数统计：`共 X 条快照`

### 10.4 无结果时的输出

```text
暂无本地快照
```

如果你刚初始化项目或刚清空本地数据库，这是正常现象。

## 12. `snapshot restore`：恢复快照

### 12.1 预览模式（推荐先预览）

在正式恢复前，建议先用 `--dry-run` 查看会变更哪些文件：

```powershell
.\fl.exe snapshot restore "升级前备份" --dry-run
```

输出示例：

```text
快照: 升级前备份 (550e8400-...)
即将恢复 3 个文件:
  更新: C:\Users\xxx\.claude\settings.json
  更新: C:\Users\xxx\.claude\CLAUDE.md
  新增: C:\Users\xxx\.claude\plugins\config.json
  跳过: C:\Users\xxx\.claude\skills\README.md (内容相同)
```

### 12.2 全量恢复

```powershell
.\fl.exe snapshot restore "升级前备份"
```

系统会先展示变更摘要，输入 `y` 确认后执行恢复。恢复前自动备份当前配置到 `~/.fl-manager/backup/<timestamp>/`。

### 12.3 选择性恢复

只恢复指定的文件：

```powershell
.\fl.exe snapshot restore "升级前备份" --files settings.json,CLAUDE.md
```

### 12.4 跳过确认

```powershell
.\fl.exe snapshot restore "升级前备份" --force
```

使用 `--force` 跳过确认步骤（仍会自动备份）。

### 12.5 参数说明

```text
用法: fl snapshot restore <id-or-name> [flags]

Flags:
      --files string   指定要恢复的文件路径，逗号分隔
      --dry-run        仅预览变更，不实际写入
      --force          跳过确认步骤，但仍自动备份
```

参数含义：

- `<id-or-name>`：快照 ID 或名称（必填）
- `--files`：指定要恢复的文件路径，逗号分隔。不指定则恢复全部文件
- `--dry-run`：仅预览变更，不实际写入文件
- `--force`：跳过确认步骤，但仍自动备份当前配置

### 12.6 恢复输出示例

```text
快照: 升级前备份 (550e8400-...)

备份已创建: C:\Users\xxx\.fl-manager\backup\20260411-143022\

恢复完成！
  成功: 3 个文件
  跳过: 9 个文件
  失败: 0 个文件
  备份: C:\Users\xxx\.fl-manager\backup\20260411-143022\
```

### 12.7 常见失败

- 快照 ID 或名称不存在
- 快照中没有可恢复的文件
- 指定的文件路径不在快照中
- 目标路径无写入权限

## 12. `tui`：进入终端交互界面

### 10.1 启动命令

```powershell
.\fl.exe tui
```

### 10.2 当前界面结构

当前 Bubble Tea TUI 包含以下页面：

- 首页
- 扫描结果页
- 创建快照页
- 快照列表页
- Setting 编辑页

### 10.3 首页操作

首页默认展示：

- 程序标题
- 当前数据目录
- 主菜单项
- 底部快捷键提示

首页菜单项当前为：

- 扫描工具
- 创建快照
- 查看快照
- 退出

首页快捷键：

- `↑ / k`：上移
- `↓ / j`：下移
- `enter`：确认
- `q`：退出

注意：

- 首页按 `q` 会直接退出程序
- 不会显示 `EOF`
- 不会把退出动作当成错误

### 10.4 扫描结果页

从首页选择“扫描工具”后，会进入扫描结果页。

页面展示：

- 检测到的扫描对象
- 中文状态
- 原因
- 可同步项数量

退出方式：

- `enter`
- `q`
- `esc`

这些按键都会返回首页。

### 10.5 创建快照页

从首页选择”创建快照”后，会进入创建表单页。

当前表单字段包括：

- `project`
- `tools`
- `message`
- `name`
- 提交创建

当前交互方式：

- `tab` 或 `↓`：切换到下一个字段
- `shift+tab` 或 `↑`：切换到上一个字段
- `enter`
  在输入字段上会跳到下一个字段
  在“提交创建”上会真正发起创建
- `esc`：返回首页

创建成功后：

- 会自动刷新本地快照列表
- 会跳转到“快照列表”页
- 首页再次进入时会看到成功状态提示

### 10.6 快照列表页

从首页选择”查看快照”后，会进入快照列表页。

页面展示：

- 以表格形式展示快照信息
- 包含 ID、名称、项目、说明、文件数、创建时间
- 如果为空，显示 `暂无本地快照`

返回方式：

- `enter`
- `q`
- `esc`

### 10.7 Setting 编辑页

通过 CLI 命令 `fl setting edit <name> -e` 进入 setting 编辑页面。

#### 页面结构

编辑器页面展示以下内容：

```
AI 配置编辑器

配置名称: <name> | 当前生效: <是/否>

配置名称: <输入框>
Token: <输入框>
API 地址: <输入框>
Opus 模型: <输入框>
Sonnet 模型: <输入框>

[快捷键提示]
```

#### 快捷键操作

| 快捷键 | 功能 |
|--------|------|
| `Tab` / `↓` | 切换到下一个字段 |
| `Shift+Tab` / `↑` | 切换到上一个字段 |
| `Ctrl+S` | 保存修改并退出 |
| `Esc` / `Ctrl+C` | 不保存直接退出 |

#### 字段说明

| 字段 | 说明 |
|------|------|
| 配置名称 | 修改配置的名称（必须唯一） |
| Token | 认证 Token，显示 `<unchanged>` 时表示保持原值 |
| API 地址 | API base URL |
| Opus 模型 | Opus 模型名称 |
| Sonnet 模型 | Sonnet 模型名称 |

#### Token 特殊显示

- Token 字段默认显示为脱敏格式：`sk-a****xxxx`
- 保持原值时显示为 `<unchanged>` 占位符
- 新输入的值将完整保存

#### 保存行为

- 按 `Ctrl+S` 保存修改：
  - 自动跳过空值字段（不修改）
  - 输入 `<unchanged>` 的字段保持原值
  - 保存成功后自动退出
- 保存失败时会在状态栏显示错误信息

#### 自定义编辑器

通过设置 `EDITOR` 环境变量可使用外部编辑器：

```powershell
# Windows
set EDITOR=code
.\fl.exe setting edit <name> -e

# Linux/macOS
export EDITOR=nano
./fl setting edit <name> -e
```

### 10.8 TUI 的适用定位

当前 TUI 适合做这些事情：

- 快速查看当前能否扫描到工具
- 直接在终端中创建快照
- 快速浏览已有快照

当前仍建议优先使用 CLI 的场景：

- 想把命令放进脚本或 CI
- 需要显式指定参数
- 需要更稳定地复用命令输出

## 13. 数据目录与本地存储

程序会在本地创建数据目录，用于保存 SQLite 数据和日志。

默认位置：

- 用户主目录下的 `.fl-manager`

如果当前环境无法解析用户主目录，则会退回到：

- 当前工作目录下的 `.fl-manager`

常见内容通常包括：

- SQLite 数据库文件
- 日志文件

## 14. 输出与日志约定

当前 CLI 输出遵循以下原则：

- 标准输出只打印业务结果
- 标准错误打印用户可读错误
- 默认不把初始化日志混入业务输出

这意味着以下命令的输出应当尽量保持干净：

- `scan`
- `snapshot create`
- `snapshot list`

如果你在脚本中调用这些命令，可以按这个约定解析结果。

## 15. 推荐使用流程

如果你第一次使用，建议按下面顺序操作：

1. 先执行 `fl scan`
2. 如果要进一步看配置内容，执行 `fl get <project> [path]`
3. 如果要直接修改配置，执行 `fl get <project> <path> --edit`
4. 执行 `fl snapshot create -p <project> -m "..."`
5. 执行 `fl snapshot list`
6. 配置出问题时，先 `fl snapshot restore <name> --dry-run` 预览变更
7. 确认后执行 `fl snapshot restore <name>` 恢复配置
8. 如果希望交互式浏览，再执行 `fl tui`

## 16. 常见问题

### 14.1 提示”未检测到任何可同步工具”

优先检查：

- `~/.codex` 是否存在
- `~/.claude` 是否存在
- 当前用户是否有权限读取这些目录

### 14.2 `scan` 显示“暂不可同步”

这通常表示：

- 配置目录存在
- 但当前规则下没有找到可收集的配置文件

此时 `snapshot create` 往往也会失败。

### 14.3 `get` 读取失败

优先检查：

- 工具名是否写成了 `codex` 或 `claude`
- 路径是否是相对于工具根目录的相对路径
- 目标是否在允许访问的配置范围内

### 14.4 `snapshot list` 一直为空

这通常表示：

- 还没有成功创建过快照
- 或当前程序使用的是另一份数据目录

可以先重新执行一次带 `--message` 的 `snapshot create`。

### 14.5 TUI 能启动但没有任何数据

TUI 只是另一层交互界面，底层仍然依赖同一套扫描与快照逻辑。

如果 CLI 的 `scan` 和 `snapshot list` 没有结果，TUI 中通常也不会有结果。

## 17. 文档维护说明

如果命令树、参数、输出格式或 TUI 页面行为发生变化，这份文档需要同步更新。更新时应优先以以下内容为准：

- `fl --help`
- `fl <command> --help`
- `internal/cli/cobra/`
- `internal/tui/`

不要继续沿用旧的 GUI / Wails 文档描述。
