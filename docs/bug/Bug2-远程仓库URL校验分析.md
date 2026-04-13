# Bug 分析：添加远程仓库时没有校验命令参数

## 基本信息

| 字段 | 内容 |
|------|------|
| 优先级 | P0 |
| 状态 | 未解决 |
| 提交人 | bbbx |
| 创建时间 | 2026-04-13 |

## 问题描述

使用 `remote add` 命令时，传入无效参数（如 `-`）也能成功添加远端仓库：

```powershell
PS G:\ProjectCode\AToSync> .\bin\ai-sync.exe remote add -
远端仓库已添加

  名称:   -
  地址:   -
  分支:   main
  连通:   成功
```

## 根因分析

### 代码追踪

**CLI 层** (`remote.go:46-63`)：
```go
command := &spcobra.Command{
    Use:  "add <url>",
    Args: spcobra.ExactArgs(1),  // 只校验参数数量，不校验内容
    RunE: func(cmd *spcobra.Command, args []string) error {
        result, err := deps.Workflow.AddRemote(cmd.Context(), typesRemote.AddRemoteInput{
            URL: args[0],  // 直接传递，无校验
        })
    },
}
```

**UseCase 层** (`remote_method.go:74-81`)：
```go
url := strings.TrimSpace(input.URL)
if url == "" {
    return nil, &UserError{
        Message:    "添加远端仓库失败：请提供仓库地址",
        Suggestion: "例如: ai-sync remote add https://github.com/user/configs.git",
    }
}
```

### 问题

1. CLI 层 `ExactArgs(1)` 只检查参数数量，`-` 通过了数量校验
2. UseCase 层只检查空字符串，`-` 不为空所以也通过了
3. 连通性测试（`ValidateRemoteConfig`）可能不够严格，`-` 也通过了
4. URL 格式完全没有校验：不检查协议前缀、域名结构等

## 修复方案

### 方案：在 UseCase 层添加 URL 格式校验

在 `AddRemote` 方法的参数校验阶段，增加 URL 格式验证：

```go
// 第一步：参数校验
url := strings.TrimSpace(input.URL)
if url == "" {
    return nil, &UserError{...}
}

// 新增：URL 格式校验
if !isValidGitURL(url) {
    return nil, &UserError{
        Message:    "添加远端仓库失败：仓库地址格式不正确",
        Suggestion: "支持的格式：\n  HTTPS: https://github.com/user/repo.git\n  SSH:   git@github.com:user/repo.git",
    }
}
```

### URL 校验规则

支持以下格式：
- HTTPS: `https://github.com/user/repo.git`
- HTTP: `http://...`
- SSH: `git@github.com:user/repo.git`
- 自定义域名: `https://git.example.com:9999/user/repo.git`

拒绝以下格式：
- 单个字符如 `-`, `.`, `a`
- 无协议前缀且非 SSH 格式
- 空路径

### 修改文件

1. **remote_method.go** — 在 `AddRemote` 方法中添加 `isValidGitURL` 校验

### 验收标准

1. `remote add -` 报错并提示正确格式
2. `remote add abc` 报错并提示正确格式
3. `remote add https://github.com/user/repo.git` 正常工作
4. `remote add git@github.com:user/repo.git` 正常工作
5. 现有功能不受影响
6. `go test ./...` 全部通过
