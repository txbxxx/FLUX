# AI Sync Manager

AI Sync Manager 是一个本地优先的配置管理工具，用于扫描 AI 工具配置、创建本地快照，并通过 CLI 或轻量终端界面浏览这些快照。

## 当前范围

当前 MVP 只覆盖本地闭环：

- 扫描本机 Codex 和 Claude 配置
- 创建本地快照
- 列出本地快照
- 进入轻量终端界面

当前阶段不包含：

- 远端仓库同步
- push、pull、diff、apply、rollback
- 凭证管理界面

## 入口

桌面开发入口：

```powershell
wails dev
```

CLI 入口：

```powershell
go run -buildvcs=false ./cmd/ai-sync <command>
```

构建 CLI 二进制：

```powershell
go build -buildvcs=false -o ai-sync.exe ./cmd/ai-sync
```

## 使用指南

CLI / TUI 使用说明见：

- [docs/命令行与终端界面MVP使用指南.md](docs/命令行与终端界面MVP使用指南.md)

## 验证

```powershell
go test ./...
go build -buildvcs=false ./cmd/ai-sync
```
