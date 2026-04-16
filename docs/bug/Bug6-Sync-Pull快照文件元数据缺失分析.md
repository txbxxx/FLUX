# Bug 分析：sync pull 后快照文件元数据字段全部为空

## 基本信息

| 字段 | 内容 |
|------|------|
| 优先级 | P0 |
| 状态 | 待修复 |
| 提交人 | TanChang |
| 创建时间 | 2026-04-16 |

## 问题描述

A 机器 `sync push` 后，B 机器 `sync pull` 拉取远端快照并写入 SQLite。写入后 `snapshot_files` 表中的元数据字段全部为空/零值：

```
id=6018  snapshot_id=5  path=plugins\...\NothingYouCouldDo-OFL.txt
original_path=(空)  size=0  hash=(空)  modified_at=0001-01-01
tool_type=(空)  category=(空)  is_binary=false
content=Copyright (c) 2010, Kimberly Geswein...  ← 有内容
```

导致后续 `snapshot restore` 时因 `original_path` 为空，无法将文件恢复到正确位置，文件被跳过。

## 根因分析

### 数据流对比

#### A 机器 push 时写入 git 仓库的数据

`sync_method.go` SyncPush 第六步（约 169 行）：

```go
for _, file := range snapshot.Files {
    targetPath := filepath.Join(repoPath, projectName, file.Path)
    writeFile(targetPath, file.Content)  // 只写了 path + content
}
```

git 仓库中只保存了：
- `path`：相对路径（如 `skills/gstack/unfreeze/SKILL.md`）
- `content`：文件内容

其余 7 个字段（`original_path`、`size`、`hash`、`modified_at`、`tool_type`、`category`、`is_binary`）不存在于 git 仓库中。

#### B 机器 pull 时更新 SQLite 的逻辑

`sync_method.go` SyncPull 第九步（约 598-620 行）：

```go
for _, sf := range safeFiles {
    content, readErr := os.ReadFile(filepath.Join(repoPath, sf.Path))

    if localSnapshot != nil {
        relativePath := strings.TrimPrefix(sf.Path, projectPrefix)
        for i, f := range localSnapshot.Files {
            if f.Path == relativePath {
                localSnapshot.Files[i].Content = content  // ← 只更新了 Content
                _ = w.snapshots.UpdateSnapshot(localSnapshot)
                break
            }
        }
    }
}
```

### 两个 Bug

**Bug 1：远端新增的文件被完全忽略**

如果远端有新文件（在 `localSnapshot.Files` 中找不到匹配），该文件不会被添加到本地快照中。代码只处理了"匹配到的文件"，没有处理"新增文件"。

**Bug 2：匹配到的文件只更新了 Content，未重算 Size 和 Hash**

即使文件匹配成功，也只设置了 `Content`，没有同步更新 `Size`（文件大小变了）和 `Hash`（内容变了哈希必然变）。

## 修复方案

### 各字段的数据来源

| 字段 | 来源 | 说明 |
|------|------|------|
| `path` | git 仓库 | 已有，相对路径 |
| `content` | git 仓库 | 已有，从 git 仓库读取文件内容 |
| `original_path` | B 机器本地项目注册信息 | 通过 `w.rules.ListRegisteredProjects()` 查询本地项目基础路径，拼接相对路径得到 |
| `tool_type` | B 机器本地项目注册信息 | 项目注册时已记录在 SQLite `registered_projects` 表中 |
| `size` | 从 content 计算 | `len(content)` |
| `hash` | 从 content 计算 | `crypto.SHA256Hash(content)` |
| `is_binary` | 从 content 检测 | 检查前 512 字节是否包含 `\x00`（复用 collector 现有逻辑） |
| `category` | 从 path 推断 | 按文件名/后缀分类（复用 collector 的 `categorizeFile` 逻辑） |
| `modified_at` | 当前时间 | `time.Now()`，表示拉取时间 |

### 修改位置

`internal/app/usecase/sync_method.go` SyncPull 第九步（约 598-620 行）

### 修改内容

1. **通过 `w.rules.ListRegisteredProjects()` 获取 B 机器的项目基础路径和 tool_type**

```go
// 查询本地项目信息，用于构建 original_path 和填充 tool_type
var projectBasePath string
var projectToolType string
projects, _ := w.rules.ListRegisteredProjects(nil)
for _, p := range projects {
    if p.ProjectName == projectName {
        projectBasePath = p.ProjectPath
        projectToolType = p.ToolType
        break
    }
}
```

2. **分离处理已匹配文件和新增文件**

```go
// 构建已有文件的路径集合
existingPaths := make(map[string]int)
for i, f := range localSnapshot.Files {
    existingPaths[f.Path] = i
}

for _, sf := range safeFiles {
    relativePath := strings.TrimPrefix(sf.Path, projectPrefix)
    content, _ := os.ReadFile(filepath.Join(repoPath, sf.Path))

    if idx, found := existingPaths[relativePath]; found {
        // 已有文件：更新 Content + 重算 Size/Hash
        localSnapshot.Files[idx].Content = content
        localSnapshot.Files[idx].Size = int64(len(content))
        localSnapshot.Files[idx].Hash = crypto.SHA256Hash(content)
    } else {
        // 新增文件：构建完整的 SnapshotFile
        localSnapshot.Files = append(localSnapshot.Files, models.SnapshotFile{
            Path:         relativePath,
            OriginalPath: filepath.Join(projectBasePath, relativePath),
            Content:      content,
            Size:         int64(len(content)),
            Hash:         crypto.SHA256Hash(content),
            ModifiedAt:   time.Now(),
            ToolType:     projectToolType,
            Category:     categorizeFile(relativePath, isBinaryContent(content)),
            IsBinary:     isBinaryContent(content),
        })
    }
}
```

3. **`UpdateSnapshot` 移到循环外，只调用一次**

```go
_ = w.snapshots.UpdateSnapshot(localSnapshot)
```

4. **复用 collector 的辅助函数**（`categorizeFile`、`isBinaryFile` 提取为包级函数或复制到 usecase 层）

### 前置条件

B 机器必须已注册对应项目（执行过 `fl scan` 或 `fl scan add`），`registered_projects` 表中有该项目的 `ProjectPath` 记录。否则无法构建 `original_path`，新增文件无法恢复到正确位置。

## 测试场景

| 测试场景 | 输入/操作 | 预期结果 | 实际结果 |
|----------|-----------|----------|----------|
| A push 后 B pull 同一项目 | B 执行 sync pull | 快照文件元数据字段全部填充 | 待验证 |
| 远端有新增文件 | B pull 后查看 snapshot_files | 新文件被添加，original_path 为 B 机器本地路径 | 待验证 |
| 远端有修改文件 | B pull 后查看 snapshot_files | 已有文件的 Content/Size/Hash 更新 | 待验证 |
| B 机器未注册项目 | B pull 未注册的项目 | 新增文件缺少 original_path，应有明确提示 | 待验证 |
| pull 后 snapshot restore | B pull 后执行 restore | 所有文件恢复到 B 机器正确路径 | 待验证 |
