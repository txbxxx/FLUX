\n> **已废弃** — 项目已从 Wails GUI 重构为 CLI/TUI 终端应用，前端 TypeScript API 不再存在。

# 前端 API 参考文档

> 生成日期：2026-03-20 | Wails 版本：v2 | Go 版本：1.25+

---

## 概述

本文档描述 AI Sync Manager 前端可调用的所有 Go 后端接口。

**Wails 调用方式**：
```typescript
import { CreateSnapshot, ListSnapshots } from './wailsjs/go/main/App'

// 直接调用 Go 方法
const result = await CreateSnapshot({ message: "test", tools: ["codex"] })
const snapshots = await ListSnapshots(10, 0)
```

---

## 目录

- [基础接口](#基础接口)
- [工具检测接口](#工具检测接口)
- [快照管理接口](#快照管理接口)
- [同步接口](#同步接口)
- [加密接口](#加密接口)
- [Git 操作接口](#git-操作接口)
- [类型定义](#类型定义)

---

## 基础接口

### Hello

返回问候语（测试用）

```typescript
function Hello(name: string): string
```

**示例**：
```typescript
const greeting = await Hello("World")
// 返回: "Hello, World!"
```

---

### GetVersion

获取应用版本

```typescript
function GetVersion(): string
```

**示例**：
```typescript
const version = await GetVersion()
// 返回: "1.0.0-alpha"
```

---

### GetSystemInfo

获取系统信息

```typescript
function GetSystemInfo(): SystemInfo
```

**返回类型**：
```typescript
interface SystemInfo {
  os: string           // 操作系统
  arch: string         // CPU 架构
  go_version: string   // Go 版本
  app_version: string  // 应用版本
}
```

**示例**：
```typescript
const info = await GetSystemInfo()
// { os: "windows", arch: "amd64", go_version: "go1.25.5", app_version: "1.0.0-alpha" }
```

---

### HealthCheck

健康检查

```typescript
function HealthCheck(): HealthInfo
```

**返回类型**：
```typescript
interface HealthInfo {
  status: string        // 状态（"ok"）
  version: string       // 应用版本
  os: string           // 操作系统
  arch: string         // CPU 架构
}
```

---

## 工具检测接口

### ScanTools

扫描本地 AI 工具（仅全局配置）

```typescript
function ScanTools(): ToolInstallation[]
```

**返回类型**：
```typescript
interface ToolInstallation {
  type: string           // 工具类型（codex, claude, cursor, windsurf）
  name: string           // 工具名称
  installed: boolean      // 是否已安装
  global_path: string    // 全局配置路径
  project_configs: ProjectConfig[]  // 项目配置
  config_files: ConfigFile[]        // 配置文件列表
}

interface ProjectConfig {
  path: string           // 项目路径
  config_path: string    // 配置文件路径
  exists: boolean        // 配置是否存在
}

interface ConfigFile {
  path: string           // 文件路径
  category: string      // 文件类别
  size: number           // 文件大小
  modified_at: string   // 修改时间
}
```

****示例**：
```typescript
const tools = await ScanTools()
// [
//   {
//     type: "codex",
//     name: "Codex",
//     installed: true,
//     global_path: "C:\\Users\\User\\AppData\\Roaming\\Codex\\config",
//     config_files: [...]
//   },
//   ...
// ]
```

---

### ScanToolsWithProjects

扫描工具（包含项目配置）

```typescript
function ScanToolsWithProjects(projectPaths: string[]): ToolInstallation[]
```

**参数**：
- `projectPaths`: 项目路径数组

**示例**：
```typescript
const tools = await ScanToolsWithProjects(["D:\\projects\\my-project"])
```

---

### GetToolConfigPath

获取工具配置路径

```typescript
function GetToolConfigPath(toolType: string): { path: string, error?: string }
```

**参数**：
- `toolType`: 工具类型（codex, claude, cursor, windsurf）

**示例**：
```typescript
const result = await GetToolConfigPath("codex")
// { path: "C:\\Users\\...\\Codex\\config" }
```

---

## 快照管理接口

### CreateSnapshot

创建配置快照

```typescript
function CreateSnapshot(input: CreateSnapshotInput): SnapshotInfo
```

**参数类型**：
```typescript
interface CreateSnapshotInput {
  message: string     // 快照描述（必填）
  tools: string[]     // 包含的工具（必填）[codex, claude, cursor, windsurf]
  name?: string        // 快照名称（可选）
  project_path?: string  // 项目路径（可选）
}
```

**返回类型**：
```typescript
interface SnapshotInfo {
  id: string           // 快照 ID
  name: string         // 快照名称
  description: string   // 快照描述
  created_at: string    // 创建时间（ISO 8601）
  tools: string[]       // 包含的工具
  file_count: number    // 文件数量
  size: number          // 总大小（字节）
}
```

**示例**：
```typescript
const snapshot = await CreateSnapshot({
  message: "保存 Claude 配置",
  tools: ["claude"],
  name: "Claude Backup"
})
// {
//   id: "550e8400-1234-5678-9012-123456789abc",
//   name: "Claude Backup",
//   description: "保存 Claude 配置",
//   created_at: "2026-03-20T11:17:21Z",
//   tools: ["claude"],
//   file_count: 1030,
//   size: 179969425
// }
```

---

### ListSnapshots

列出本地快照

```typescript
function ListSnapshots(limit: number, offset: number): SnapshotInfo[]
```

**参数**：
- `limit`: 返回数量限制（0 = 无限制）
- `offset`: 偏移量

**示例**：
```typescript
const snapshots = await ListSnapshots(10, 0)
```

---

### GetSnapshotDetail

获取快照详情

```typescript
function GetSnapshotDetail(id: string): SnapshotDetail
```

**返回类型**：
```typescript
interface SnapshotDetail {
  id: string
  name: string
  description: string
  message: string
  created_at: string
  tools: string[]
  metadata: SnapshotMetadata
  files: SnapshotFile[]
  commit_hash?: string
  tags: string[]
}

interface SnapshotMetadata {
  os_version?: string
  app_version?: string
  project_path?: string    // 项目路径（用于标识快照来源）
  extra?: Record<string, string>
}

interface SnapshotFile {
  path: string           // 文件相对路径
  original_path: string  // 原始路径
  size: number          // 文件大小
  hash?: string         // 文件哈希
  modified_at: string   // 修改时间
  tool_type: string     // 所属工具
  category: string      // 文件类别
  is_binary: boolean    // 是否为二进制文件
}
```

**文件类别 (`category`)**：
- `config`: 配置文件
- `skills`: 技能文件
- `commands`: 命令文件
- `plugins`: 插件文件
- `mcp`: MCP 配置
- `agents`: Agent 文件
- `rules`: 规则文件
- `docs`: 文档文件
- `output`: 输出样式
- `other`: 其他文件

---

### DeleteSnapshot

删除快照

```typescript
function DeleteSnapshot(id: string): void
```

**示例**：
```typescript
await DeleteSnapshot("550e8400-1234-5678-9012-123456789abc")
```

---

### ApplySnapshot

应用快照到本地

```typescript
function ApplySnapshot(input: ApplySnapshotInput): ApplySnapshotInfo
```

**参数类型**：
```typescript
interface ApplySnapshotInput {
  snapshot_id: string   // 快照 ID（必填）
  create_backup: boolean // 是否创建备份（默认 false）
  force: boolean         // 是否强制覆盖（默认 false）
  dry_run: boolean       // 是否仅预览（默认 false）
}
```

**返回类型**：
```typescript
interface ApplySnapshotInfo {
  success: boolean
  applied_count: number
  skipped_count: number
  errors?: string[]
  created_files?: string[]
  updated_files?: string[]
  summary: ChangeSummary
}

interface ChangeSummary {
  total_files: number
  created: number
  updated: number
  deleted: number
  skipped: number
  files_by_tool: Record<string, number>
  files_by_category: Record<string, number>
}
```

**示例**：
```typescript
const result = await ApplySnapshot({
  snapshot_id: "550e8400-1234-5678-9012-123456789abc",
  create_backup: true,
  force: false,
  dry_run: false  // 设为 true 可预览变更
})

if (result.dry_run) {
  console.log("预览模式，不会实际修改文件")
}
```

---

## 同步接口

### PushSnapshot

推送快照到远程仓库

```typescript
function PushSnapshot(input: PushSnapshotInput): PushResultInfo
```

**参数类型**：
```typescript
interface PushSnapshotInput {
  snapshot_id: string  // 快照 ID（必填）
  remote_url: string  // 远程仓库 URL（必填）
  branch?: string     // 分支名（默认 main）
  force_push: boolean // 是否强制推送（默认 false）
}
```

**返回类型**：
```typescript
interface PushResultInfo {
  success: boolean
  snapshot_id: string
  commit_hash?: string
  branch: string
  pushed_at?: string
  message: string
}
```

**示例**：
```typescript
const result = await PushSnapshot({
  snapshot_id: "550e8400-1234-5678-9012-123456789abc",
  remote_url: "https://github.com/user/ai-config-sync.git",
  branch: "main"
})
```

---

### PullSnapshots

从远程仓库拉取快照

```typescript
function PullSnapshots(input: PullSnapshotsInput): PullResultInfo
```

**参数类型**：
```typescript
interface PullSnapshotsInput {
  remote_url: string  // 远程仓库 URL（必填）
  branch?: string     // 分支名（默认 main）
  repo_path?: string  // 本地仓库路径
}
```

**返回类型**：
```typescript
interface PullResultInfo {
  success: boolean
  branch: string
  new_snapshots: number
  message: string
}
```

**示例**：
```typescript
const result = await PullSnapshots({
  remote_url: "https://github.com/user/ai-config-sync.git",
  branch: "main"
})
// { success: true, new_snapshots: 2, message: "成功拉取，新增 2 个快照" }
```

---

## 加密接口

### GetEncryptionStatus

获取加密状态

```typescript
function GetEncryptionStatus(): EncryptionStatus
```

**返回类型**：
```typescript
interface EncryptionStatus {
  enabled: boolean      // 是否启用
  algorithm: string     // 加密算法
  has_key: boolean      // 是否有密钥
  key_source: string     // 密钥与其他（"file", "env", "generated"）
}
```

**示例**：
```typescript
const status = await GetEncryptionStatus()
// { enabled: false, algorithm: "none", has_key: false, key_source: "not_initialized" }
```

---

### ConfigureEncryption

配置加密

```typescript
function ConfigureEncryption(input: EncryptionConfigInput): void
```

**参数类型**：
```typescript
interface EncryptionConfigInput {
  enabled: boolean      // 是否启用
  algorithm?: string    // 加密算法（默认 aes256-gcm）
  key_path?: string      // 密钥文件路径
  key_env_var?: string  // 密钥环境变量名
}
```

**示例**：
```typescript
// 使用环境变量密钥
await ConfigureEncryption({
  enabled: true,
  algorithm: "aes256-gcm",
  key_env_var: "MY_ENCRYPTION_KEY"
})

// 使用文件密钥
await ConfigureEncryption({
  enabled: true,
  key_path: "~/.ai-sync-manager/key"
})

// 自动生成密钥
await ConfigureEncryption({
  enabled: true
})
```

---

### EncryptData / DecryptData

加密/解密数据

```typescript
function EncryptData(plaintext: string): Promise<string>
function DecryptData(ciphertext: string): Promise<string>
```

**示例**：
```typescript
const encrypted = await EncryptData("my secret data")
const decrypted = await DecryptData(encrypted)
```

---

### EncryptPassword / DecryptPassword

加密/解密密码

```typescript
function EncryptPassword(password: string): Promise<string>
function DecryptPassword(encrypted: string): Promise<string>
```

**示例**：
```typescript
const encryptedPassword = await EncryptPassword("my-password-123")
const decryptedPassword = await DecryptPassword(encryptedPassword)
```

---

### EncryptToken / DecryptToken

加密/解密 Token

```typescript
function EncryptToken(token: string): Promise<string>
function DecryptToken(encrypted: string): Promise<string>
```

**示例**：
```typescript
const encryptedToken = await EncryptToken("ghp_xxxxxxxxxxxxx")
const decryptedToken = await DecryptToken(encryptedToken)
```

---

### GenerateEncryptionKey

生成新的加密密钥

```typescript
function GenerateEncryptionKey(): Promise<string>
```

**返回**：
- Base64 编码的 32 字节随机密钥

**示例**：
```typescript
const newKey = await GenerateEncryptionKey()
// 保存密钥到安全位置
```

---

## Git 操作接口

### GitClone

克隆 Git 仓库

```typescript
function GitClone(url: string, path: string, branch?: string): RepositoryInfo
```

**返回类型**：
```typescript
interface RepositoryInfo {
  path: string           // 仓库路径
  is_repository: boolean // 是否为 Git 仓库
  head?: BranchInfo     // 当前分支信息
  branches: BranchInfo[] // 所有分支
  current_commit?: string // 当前提交哈希
}

interface BranchInfo {
  name: string           // 分支名
  is_remote: boolean     // 是否为远程分支
  is_head: boolean      // 是否为当前 HEAD
  commit?: string        // 提交哈希
  message?: string      // 提交消息
  author?: string       // 作者
  date?: string         // 日期
}
```

---

### GitPull

拉取 Git 仓库更新

```typescript
function GitPull(path: string): OperationResult
```

**返回类型**：
```typescript
interface OperationResult {
  success: boolean
  message?: string
  files_changed?: number
  insertions?: number
  deletions?: number
}
```

---

### GitPush

推送到 Git 远程仓库

```typescript
function GitPush(path: string, branch?: string, force?: boolean): OperationResult
```

---

### GitGetStatus

获取 Git 仓库状态

```typescript
function GitGetStatus(path: string): RepositoryStatus
```

**返回类型**：
```typescript
interface RepositoryStatus {
  path: string           // 仓库路径
  is_clean: boolean     // 工作区是否干净
  branch?: string       // 当前分支
  staged: FileStatus[]   // 已暂存文件
  unstaged: FileStatus[] // 未暂存文件
}

interface FileStatus {
  path: string           // 文件路径
  status: string        // 状态
}
```

---

### GitCommit

提交更改到 Git 仓库

```typescript
function GitCommit(path: string, message: string): CommitResult
```

**返回类型**：
```typescript
interface CommitResult {
  success: boolean
  hash?: string         // 提交哈希
  message?: string
  files_changed?: number
}
```

---

### GitGetRepositoryInfo

获取 Git 仓库信息

```typescript
function GitGetRepositoryInfo(path: string): RepositoryInfo
```

---

### GitListBranches

列出 Git 仓库的所有分支

```typescript
function GitListBranches(path: string): BranchInfo[]
```

---

### GitIsRepository

检查路径是否为 Git 仓库

```typescript
function GitIsRepository(path: string): boolean
```

---

### GitValidateAuth

验证 Git 认证配置

```typescript
function GitValidateAuth(authType: string, username?: string, password?: string): void
```

---

## 类型定义

### 工具类型

```typescript
type ToolType = "codex" | "claude" | "cursor" | "windsurf"
```

### 文件类别

```typescript
type FileCategory =
  | "config"    // 配置文件
  | "skills"    // 技能文件
    | "commands"   // 命令文件
  | "plugins"    // 插件文件
  | "mcp"        // MCP 配置
  | "agents"     // Agent 文件
  | "rules"      // 规则文件
  | "docs"       // 文档文件
  | "output"     // 输出样式
  | "other"      // 其他文件
```

### 项目模型

快照现在基于项目模型创建，不再区分 global/project/both scope。

**全局项目**（自动注册）：
- `codex-global` - Codex 全局配置 (~/.codex)
- `claude-global` - Claude 全局配置 (~/.claude)

**用户项目**：
- 通过 `scan add` 命令注册的自定义项目

### 认证类型

```typescript
type AuthType = "" | "ssh" | "token" | "basic"
```

---

## 错误处理

所有接口调用可能抛出错误，前端应进行错误处理：

```typescript
try {
  const snapshot = await CreateSnapshot({ message: "test", tools: ["claude"] })
  // 处理成功
} catch (error) {
  // 处理错误
  console.error("创建快照失败:", error)
  // 显示错误提示给用户
  showError(error.message || "操作失败")
}
```

---

## 完整示例

```typescript
// 导入 Wails 生成的绑定
import {
  ScanTools,
  CreateSnapshot,
  ListSnapshots,
  ApplySnapshot,
  PushSnapshot,
  PullSnapshots,
  GetEncryptionStatus,
  ConfigureEncryption,
  EncryptPassword,
  DecryptPassword
} from './wailsjs/go/main/App'

// 1. 扫描描工具
const tools = await ScanTools()
console.log("检测到的工具:", tools)

// 2. 创建快照
const snapshot = await CreateSnapshot({
  message: "初始配置备份",
  tools: ["claude", "codex"],
  name: "Initial Backup"
})

// 3. 列出快照
const snapshots = await ListSnapshots(10, 0)
console.log("本地快照:", snapshots)

// 4. 应用快照
const applyResult = await ApplySnapshot({
  snapshot_id: snapshot.id,
  create_backup: true,
  force: false
})

// 5. 推送到远程仓库
const pushResult = await PushSnapshot({
  snapshot_id: snapshot.id,
  remote_url: "https://github.com/user/config-sync.git",
  branch: "main"
})

// 6. 配置并使用加密
await ConfigureEncryption({
  enabled: true,
  key_env_var: "MY_SECRET_KEY"
})

const encrypted = await EncryptPassword("my-password")
const decrypted = await DecryptPassword(encrypted)
```

---

## 注意事项

1. **异步调用**：所有 Go 方法都是异步的，必须使用 `await` 或 `.then()`
2. **数据序列化**：复杂对象会自动序列化
3. **错误处理**：务必使用 try-catch 处理错误
4. **性能考虑**：大文件操作可能较慢，建议显示加载状态
5. **密钥安全**：加密密钥不要在日志中输出，不要存储在不安全的地方

---

## 待实现接口

以下接口已定义但尚未实现：

- `CreateSnapshot`（完整版本）
- `ListRemoteSnapshots` - 列出远程快照
- `PullAndApply` - 拉取并应用快照
- `CompareWithRemote` - 比较本地与远程差异
- `ConfigureGitRemote` - 配置 Git 远程仓库
- `SetSyncScope` - 设置同步范围
