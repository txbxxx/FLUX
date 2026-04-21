# Flux

Flux 是一个本地优先的 AI 工具配置管理器，当前主线已经收敛为纯终端形态，提供 CLI 和轻量 TUI 两种使用方式。

它当前聚焦在本地闭环：

- 扫描本机 Codex / Claude 配置
- 创建本地快照
- 列出本地快照
- 浏览配置目录与文件
- 在终端内直接编辑受支持的配置文件

## 当前状态

仓库主入口：

- CLI：`cmd/fl`
- 终端界面：`fl tui`

当前已支持：

- 本地配置扫描与快照管理
- 远端仓库同步（remote add/list/remove、sync push/pull/status）
- 配置文件浏览与编辑（get、setting edit）
- 快照对比与恢复（snapshot diff、restore）
- 终端交互界面（TUI）

暂未覆盖：

- push/pull 时的冲突自动合并（需手动处理）
- 图形化凭证管理界面

## 快速开始

直接运行 CLI：

```powershell
go run -buildvcs=false ./cmd/fl <command>
```

使用 `Makefile` 构建本地二进制：

```powershell
make build
```

如果你想修改 CLI 名称，例如生成 `sync-tool.exe`：

```powershell
make build CLI_NAME=sync-tool
```

默认输出目录是 `bin/`，例如 Windows 下会生成 `bin/fl.exe`。
如果你需要非 Windows 产物，可以继续直接使用 `go build`，或者执行 `make build GOEXE=`。

如果你想直接运行 CLI，可以使用：

```powershell
make run
make run ARGS=scan
```

其中 `make run` 默认会执行 `.\bin\fl.exe --help`。

查看帮助：

```powershell
go run -buildvcs=false ./cmd/fl --help
```

## 主要命令

扫描本机配置：

```powershell
fl scan
```

创建快照：

```powershell
fl snapshot create --name "我的快照" --message "初始化配置"
```

列出快照：

```powershell
fl snapshot list
```

查看配置目录或文件：

```powershell
fl get codex skills\
fl get codex skills\README.md
```

进入终端编辑模式：

```powershell
fl get codex skills\README.md --edit
```

管理 AI 配置（setting 命令）：

```powershell
# 创建配置
fl setting create --name "claude-api" --token "sk-xxxx" --api "https://api.anthropic.com"

# 列出配置
fl setting list

# 获取配置详情
fl setting get claude-api

# 编辑配置
fl setting edit claude-api -t "sk-new-token"

# 删除配置
fl setting delete claude-api

# 切换生效配置
fl setting switch claude-api
```

管理远端仓库（remote 命令）：

```powershell
# 添加远端仓库
fl remote add https://github.com/user/config.git --name origin

# 列出远端仓库
fl remote list

# 删除远端仓库
fl remote remove origin
```

同步配置（sync 命令）：

```powershell
# 推送到远端
fl sync push --project claude-global

# 从远端拉取
fl sync pull --project claude-global

# 查看同步状态
fl sync status --project claude-global
```

启动轻量 TUI：

```powershell
fl tui
```

## 文档

详细使用说明见：

- [docs/使用指南/命令行与终端界面MVP使用指南.md](docs/使用指南/命令行与终端界面MVP使用指南.md)

架构说明见：

- [docs/架构设计/终端架构详解.md](docs/架构设计/终端架构详解.md)

文档分类总览见：

- [docs/文档索引.md](docs/文档索引.md)

## 开发与验证

常用验证命令：

```powershell
go test ./...
make build
```

如果你刚从旧分支切回主线，命令入口统一收敛到 `cmd/fl/main.go`。
