# Bug 分析：推送远端仓库时报错/成功信息应暴露远端仓库地址

## 基本信息

| 字段 | 内容 |
|------|------|
| 优先级 | P0 |
| 状态 | 未解决 |
| 提交人 | bbbx |
| 创建时间 | 2026-04-13 |

## 问题描述

推送失败时，错误信息缺少远端仓库地址上下文：

```
推送失败：无法推送到远端: 推送失败: authentication required:
请检查网络连接和仓库权限。本地数据未受影响。
```

用户无法判断是哪个远端仓库失败，尤其在配置多个远端仓库时。

## 根因分析

### 代码追踪

**UseCase 层** (`sync_method.go:169-181`)：

```go
// 第七步：Git push
_, err = gitClient.Push(ctx, &git.PushOptions{...})
if err != nil {
    return nil, &UserError{
        Message:    "推送失败：无法推送到远端",
        Suggestion: "请检查网络连接和仓库权限。本地数据未受影响。",
        Err:        err,
    }
}
```

此时 `remoteConfig` 变量已在作用域中，包含 `remoteConfig.Name` 和 `remoteConfig.URL`，但未在错误信息中使用。

### 对比成功场景

成功时（`sync_method.go:189-195`），结果中包含了 `RemoteURL`：

```go
return &typesSync.SyncPushResult{
    RemoteURL: remoteConfig.URL,  // 已包含
}
```

CLI 层成功输出（`sync.go:121`）也显示了远端地址：

```go
fmt.Fprintf(w, "  远端:   %s\n", result.RemoteURL)
```

### 其他同类问题位置

`sync_method.go` 中还有几处类似的错误信息缺少远端上下文：

| 行号 | 操作 | 当前错误信息 | 是否包含仓库信息 |
|------|------|-------------|----------------|
| 119 | clone/init 失败 | "推送失败：无法创建工作目录" | 否 |
| 161-165 | commit 失败 | "推送失败：Git commit 失败" | 否 |
| 176-181 | push 失败 | "推送失败：无法推送到远端" | 否 |
| 284 | fetch 失败 | "拉取失败：无法获取远端更新" | 否 |
| 429-433 | pull 失败 | "拉取失败：Git pull 失败" | 否 |

## 修复方案

在所有推送/拉取相关的错误信息中，加入远端仓库名称和地址上下文。

### 修改文件

1. **sync_method.go** — 所有 `SyncPush` 和 `SyncPull` 中的 UserError 消息

### 具体修改

格式统一为：`操作 + 仓库信息 + 具体错误`

```go
// push 失败（Bug 原始问题）
return nil, &UserError{
    Message:    fmt.Sprintf("推送失败：无法推送到远端仓库 \"%s\" (%s)", remoteConfig.Name, remoteConfig.URL),
    Suggestion: "请检查网络连接和仓库权限。本地数据未受影响。",
    Err:        err,
}

// clone/init 失败
return nil, &UserError{
    Message:    fmt.Sprintf("推送失败：无法创建工作目录（远端: %s）", remoteConfig.URL),
    ...
}

// commit 失败
return nil, &UserError{
    Message:    fmt.Sprintf("推送失败：Git commit 失败（远端: %s）", remoteConfig.URL),
    ...
}
```

`SyncPull` 中同理，在 fetch 和 pull 失败时也加入远端仓库地址。

### 验收标准

1. push 失败时错误信息包含远端仓库名称和地址
2. pull 失败时错误信息包含远端仓库名称和地址
3. 成功信息不受影响
4. 现有功能不受影响
5. `go test ./...` 全部通过
