# AI Sync Manager 命令行与终端界面使用指南

## 1. 文档目标

本文档面向当前终端化版本的 AI Sync Manager，说明以下内容：

- 项目当前能做什么
- 如何构建和运行 CLI
- 各命令的用途、参数和输出格式
- 如何使用 `ai-sync tui` 进入终端交互界面
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
go run -buildvcs=false ./cmd/ai-sync <command>
```

示例：

```powershell
go run -buildvcs=false ./cmd/ai-sync scan
```

### 4.2 构建为二进制

```powershell
go build -buildvcs=false -o ai-sync.exe ./cmd/ai-sync
```

构建完成后可以直接执行：

```powershell
.\ai-sync.exe <command>
```

如果你使用的是非 Windows 环境，可以把 `ai-sync.exe` 替换为你自己的目标文件名，例如：

```bash
go build -buildvcs=false -o ai-sync ./cmd/ai-sync
./ai-sync scan
```

## 5. 命令总览

当前 Cobra 命令树包含以下入口：

- `ai-sync scan`
- `ai-sync scan add`
- `ai-sync scan remove`
- `ai-sync scan list`
- `ai-sync get`
- `ai-sync snapshot create`
- `ai-sync snapshot list`
- `ai-sync tui`
- `ai-sync completion`

查看根帮助：

```powershell
.\ai-sync.exe --help
```

当前帮助输出结构如下：

```text
AI tool config snapshot manager

Usage:
  ai-sync [flags]
  ai-sync [command]

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
.\ai-sync.exe scan
.\ai-sync.exe scan codex
.\ai-sync.exe scan claude
.\ai-sync.exe scan codex claude
.\ai-sync.exe scan ai-sync-manager
.\ai-sync.exe scan list
.\ai-sync.exe scan list claude
.\ai-sync.exe scan add claude C:\Users\<user>\.claude.json
.\ai-sync.exe scan add --project codex demo D:\workspace\demo
```

### 6.2 功能说明

`scan` 现在基于统一规则源工作，会输出两类同级扫描对象：

- 全局扫描对象
- 已注册项目扫描对象

其中过滤参数既可以是应用名，也可以是已注册项目名：

- `scan codex` 会输出 Codex 全局对象和所有 Codex 项目对象
- `scan ai-sync-manager` 会只输出名为 `ai-sync-manager` 的已注册项目对象

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

### 6.6 `scan add/remove/list`

当前支持两类规则操作：

```powershell
.\ai-sync.exe scan add <app> <absolute-file-path>
.\ai-sync.exe scan remove <app> <absolute-file-path>
.\ai-sync.exe scan add --project <app> <project-name> <project-absolute-path>
.\ai-sync.exe scan remove --project <app> <project-absolute-path>
.\ai-sync.exe scan list
.\ai-sync.exe scan list <app>
```

说明：

- 默认规则由程序内置，不需要手动添加
- `scan add <app> <absolute-file-path>` 用于补充分散在其他位置的配置文件
- `scan add --project ...` 用于注册项目，注册后会按该工具的项目规则模板生成独立的项目扫描对象，并参与扫描和快照
- `scan list` 会展示默认全局规则、自定义绝对路径规则和已注册项目
- 只有存在已注册项目时，才会额外显示“已注册项目扫描模板”

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
.\ai-sync.exe get codex skills\
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
.\ai-sync.exe get codex skills\aiskills\
```

### 7.2 查看文件内容

```powershell
.\ai-sync.exe get codex skills\aiskills\README.md
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
.\ai-sync.exe get codex skills\aiskills\README.md --edit
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

## 8. `snapshot create`：创建本地快照

### 7.1 最小用法

```powershell
.\ai-sync.exe snapshot create --tools codex --message "backup before change"
```

### 8.2 完整示例

```powershell
.\ai-sync.exe snapshot create `
  --tools codex,claude `
  --message "backup before change" `
  --name "Daily backup" `
  --scope both
```

### 7.3 参数说明

`snapshot create --help` 当前输出的参数如下：

```text
--message string
--name string
--project-path string
--scope string
--tools string
```

含义说明：

- `--tools`
  逗号分隔的工具列表，例如 `codex`、`claude`
- `--message`
  本次快照说明，建议写明用途，例如“升级前备份”
- `--name`
  快照显示名称，可选
- `--scope`
  快照范围，默认是 `global`
- `--project-path`
  当前保留兼容；统一规则模式下，项目级快照以 `scan add --project` 已注册项目为准

### 7.4 参数约束

当前实现中，以下两个参数是业务层强校验：

- `--tools` 不能为空
- `--message` 不能为空

如果缺失，会返回用户可读错误。

### 7.5 成功输出

成功创建后，输出格式如下：

```text
created snapshot: 12345678-1234-1234-1234-1234567890ab
name: Daily backup
files: 8
size: 4096 bytes
```

字段说明：

- `created snapshot`：新建快照 ID
- `name`：快照名称
- `files`：收集到的文件数量
- `size`：快照总大小

### 7.6 失败场景

常见失败包括：

- 工具列表为空
- `message` 为空
- 目标工具存在，但没有可打包的配置文件

例如当 `scan` 输出“可同步项: 0 项”时，后续执行 `snapshot create` 很可能失败，因为当前统一规则下没有可归档内容。

## 9. `snapshot list`：列出本地快照

### 8.1 基础用法

```powershell
.\ai-sync.exe snapshot list
```

### 8.2 分页参数

```powershell
.\ai-sync.exe snapshot list --limit 20 --offset 0
```

当前帮助输出如下：

```text
--limit int
--offset int
```

参数含义：

- `--limit`
  最多返回多少条记录
- `--offset`
  从第几条之后开始返回

### 8.3 有结果时的输出

```text
12345678-1234-1234-1234-1234567890ab | Daily backup | backup before change
```

每一行依次表示：

- 快照 ID
- 快照名称
- 快照说明

### 8.4 无结果时的输出

```text
暂无本地快照
```

如果你刚初始化项目或刚清空本地数据库，这是正常现象。

## 10. `tui`：进入终端交互界面

### 9.1 启动命令

```powershell
.\ai-sync.exe tui
```

### 9.2 当前界面结构

当前 Bubble Tea TUI 包含以下页面：

- 首页
- 扫描结果页
- 创建快照页
- 快照列表页

### 9.3 首页操作

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

### 9.4 扫描结果页

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

### 9.5 创建快照页

从首页选择“创建快照”后，会进入创建表单页。

当前表单字段包括：

- `tools`
- `message`
- `name`
- `project-path`
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

### 9.6 快照列表页

从首页选择“查看快照”后，会进入快照列表页。

页面展示：

- 已存在快照的 ID 与说明
- 如果为空，显示 `暂无本地快照`

返回方式：

- `enter`
- `q`
- `esc`

### 9.7 TUI 的适用定位

当前 TUI 适合做这些事情：

- 快速查看当前能否扫描到工具
- 直接在终端中创建快照
- 快速浏览已有快照

当前仍建议优先使用 CLI 的场景：

- 想把命令放进脚本或 CI
- 需要显式指定参数
- 需要更稳定地复用命令输出

## 11. 数据目录与本地存储

程序会在本地创建数据目录，用于保存 SQLite 数据和日志。

默认位置：

- 用户主目录下的 `.ai-sync-manager`

如果当前环境无法解析用户主目录，则会退回到：

- 当前工作目录下的 `.ai-sync-manager`

常见内容通常包括：

- SQLite 数据库文件
- 日志文件

## 12. 输出与日志约定

当前 CLI 输出遵循以下原则：

- 标准输出只打印业务结果
- 标准错误打印用户可读错误
- 默认不把初始化日志混入业务输出

这意味着以下命令的输出应当尽量保持干净：

- `scan`
- `snapshot create`
- `snapshot list`

如果你在脚本中调用这些命令，可以按这个约定解析结果。

## 13. 推荐使用流程

如果你第一次使用，建议按下面顺序操作：

1. 先执行 `ai-sync scan`
2. 如果要进一步看配置内容，执行 `ai-sync get <app> <path>`
3. 如果要直接修改配置，执行 `ai-sync get <app> <path> --edit`
4. 执行 `ai-sync snapshot create --tools ... --message "..."`
5. 执行 `ai-sync snapshot list`
6. 如果希望交互式浏览，再执行 `ai-sync tui`

## 14. 常见问题

### 13.1 提示“未检测到任何可同步工具”

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

## 15. 文档维护说明

如果命令树、参数、输出格式或 TUI 页面行为发生变化，这份文档需要同步更新。更新时应优先以以下内容为准：

- `ai-sync --help`
- `ai-sync <command> --help`
- `internal/cli/cobra/`
- `internal/tui/`

不要继续沿用旧的 GUI / Wails 文档描述。
