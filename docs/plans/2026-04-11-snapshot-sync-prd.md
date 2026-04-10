# 快照同步功能 PRD

> 创建时间：2026-04-11
> 状态：待开发
> 优先级：P0 / P1
> 前置依赖：快照恢复功能（第一阶段）

---

## 1. 背景与目标

### 1.1 现状

- 快照数据仅存储在本地 SQLite 数据库中，无法跨设备同步
- 快照创建后不支持更新，用户修改配置后只能新建快照
- 产品名 AToSync 但同步功能完全缺失

### 1.2 目标

基于 Git 构建透明的版本控制和同步引擎，实现：

- 快照更新能力（覆盖本地数据）
- 多设备配置同步（通过 Git 远端仓库）
- 版本历史管理（Git commit 记录）
- 冲突自动检测与处理（Git 原生 merge）

### 1.3 核心理念

- **Git 对用户完全透明** — 用户不会直接接触任何 Git 命令
- **SQLite 是唯一真相源** — 存储最新快照数据（元数据 + 文件内容）
- **Git 是版本引擎** — 负责历史记录、冲突处理、远端同步
- **本地优先** — 没有 Git 远端仓库也能使用快照功能

---

## 2. 目标用户

- 在多台设备上使用 AI 工具的开发者
- 希望配置文件有版本历史的用户
- 团队协作中需要共享 AI 工具配置的场景

---

## 3. 用户场景

### 场景 1：配置改了，更新快照

> 小明修改了 Claude 的 settings.json，执行 `ai-sync snapshot update "日常配置"`，系统自动重新扫描并更新快照数据。旧版本通过 Git 历史保留。

### 场景 2：公司电脑推配置到家里电脑

> 小明在公司电脑执行 `ai-sync sync push`，配置推送到 GitHub 仓库。回家后在家里的电脑执行 `ai-sync sync pull`，配置自动同步。

### 场景 3：两台电脑都改了配置，有冲突

> 公司和家里都修改了 settings.json。家里电脑 pull 时系统检测到冲突，展示 diff 让小明选择保留哪个版本。

### 场景 4：多个工具推到同一个仓库

> 小红把 claude-global 和 codex-global 都推到同一个 GitHub 仓库，仓库里按项目名分目录，结构清晰。

### 场景 5：回退到上周的配置

> 小刚发现最近的配置改动有问题，执行 `ai-sync snapshot restore "日常配置" --history`，查看版本历史，选择上周的版本恢复。

---

## 4. 功能需求

### FR-01 快照更新

用户可以更新已有快照，用最新的配置文件覆盖数据库中的旧数据。

- 执行 update 时自动重新扫描关联 project 的配置文件
- 覆盖 SQLite 中的快照文件数据（元数据保留）
- 自动在工作目录执行 Git commit，保留历史版本
- update 前后如果文件内容无变化，提示"无变更"

### FR-02 远端仓库管理

用户可以配置 Git 远端仓库地址，作为同步通道。

- 支持添加远端仓库（remote add）
- 支持查看已配置的远端仓库（remote list）
- 支持删除远端仓库（remote remove）
- 支持 GitHub、Gitea、自建 Git 仓库
- 一个 project 绑定一个远端仓库
- 多个 project 可以共享同一个远端仓库（按子目录分开）

### FR-03 推送到远端

用户可以将本地快照数据推送到远端 Git 仓库。

- 从 SQLite 读出快照最新数据
- 写入到工作目录（`~/.ai-sync/repos/<project-name>/`）
- 执行 git add + commit + push（对用户透明）
- 推送失败时展示错误信息，不影响本地数据

### FR-04 从远端拉取

用户可以从远端 Git 仓库拉取最新配置并更新本地数据库。

- 执行 git pull（对用户透明）
- 检测变更文件列表
- 无冲突时自动合并，更新 SQLite 数据
- 有冲突时展示 diff，提示用户手动解决
- 拉取后提示用户"有 N 个文件更新"

### FR-05 版本历史查看

用户可以查看某个快照的版本历史。

- 从 Git commit 日志读取历史记录
- 展示每次更新的时间、说明、变更文件数
- SQLite 可选存历史索引（commit hash、时间、说明）用于快速查询

### FR-06 历史版本恢复

用户可以恢复到快照的某个历史版本。

- 从 Git 历史中选择特定版本
- 展示 diff（当前版本 vs 历史版本）
- 用户确认后恢复（走快照恢复流程）

---

## 5. 数据架构

### 5.1 双重存储模型

```
┌─────────────────────────────────────────────┐
│              SQLite（真相源）                 │
│  ┌─────────┐  ┌──────────────┐              │
│  │ 快照元数据 │  │ 文件内容（最新） │              │
│  └─────────┘  └──────────────┘              │
│  ┌─────────────────────┐                    │
│  │ 历史索引（可选）       │ ← commit hash + 时间 + 说明  │
│  └─────────────────────┘                    │
└─────────────────────────────────────────────┘
                    ↕ 同步
┌─────────────────────────────────────────────┐
│          Git 仓库（版本引擎）                  │
│  ┌──────────────────────────────┐           │
│  │ 工作目录: ~/.ai-sync/repos/   │           │
│  │  ├── project-a/              │           │
│  │  │   ├── settings.json       │           │
│  │  │   └── .claude/CLAUDE.md   │           │
│  │  └── project-b/              │           │
│  │      └── config.yaml         │           │
│  └──────────────────────────────┘           │
│  commit 历史: v1 → v2 → v3 → ...           │
└─────────────────────────────────────────────┘
                    ↕ push/pull
┌─────────────────────────────────────────────┐
│          远端仓库（同步通道）                  │
│  GitHub / Gitea / 自建 Git                   │
└─────────────────────────────────────────────┘
```

### 5.2 远端仓库与 project 关系

- 一个 project 绑定一个远端仓库
- 一个远端仓库可被多个 project 共用
- 共用同一仓库时，每个 project 推送到不同的子目录
- 暂不支持一个 project 推到多个远端

---

## 6. CLI 命令

### 6.1 快照更新

```
ai-sync snapshot update <id-or-name>           # 更新快照（自动重新扫描）
ai-sync snapshot update <id-or-name> -m "说明"  # 更新时附加说明
ai-sync snapshot history <id-or-name>          # 查看版本历史
```

### 6.2 远端仓库管理

```
ai-sync remote add <url>                       # 添加远端仓库
ai-sync remote add <url> --project <name>      # 添加并绑定到指定 project
ai-sync remote list                            # 查看已配置的远端仓库
ai-sync remote remove <name>                   # 删除远端仓库
```

### 6.3 同步操作

```
ai-sync sync push                              # 推送当前 project 到远端
ai-sync sync push --project <name>             # 推送指定 project
ai-sync sync push --all                        # 推送所有 project
ai-sync sync pull                              # 从远端拉取当前 project
ai-sync sync pull --project <name>             # 拉取指定 project
ai-sync sync pull --all                        # 拉取所有 project
```

### 6.4 历史版本恢复

```
ai-sync snapshot restore <id-or-name> --history        # 交互式选择历史版本
ai-sync snapshot restore <id-or-name> --version <hash>  # 恢复指定版本
```

---

## 7. 交互流程

### 7.1 快照更新

```
用户执行 snapshot update
       ↓
  找到快照？──否──→ 报错："快照 'xxx' 不存在"
       ↓ 是
  自动重新扫描关联 project 的配置文件
       ↓
  内容有变化？──否──→ 提示"无变更"
       ↓ 是
  更新 SQLite 数据
       ↓
  自动 Git commit（透明）
       ↓
  展示更新结果：N 个文件已更新
```

### 7.2 推送到远端

```
用户执行 sync push
       ↓
  已配置远端仓库？──否──→ 提示"请先执行 remote add"
       ↓ 是
  从 SQLite 读出最新快照数据
       ↓
  写入工作目录（~/.ai-sync/repos/<project>/）
       ↓
  Git add + commit + push（透明）
       ↓
  展示推送结果：N 个文件已推送
```

### 7.3 从远端拉取

```
用户执行 sync pull
       ↓
  已配置远端仓库？──否──→ 提示"请先执行 remote add"
       ↓ 是
  Git pull（透明）
       ↓
  有冲突？──否──→ 自动合并 → 更新 SQLite → 展示"N 个文件已更新"
       ↓ 是
  展示冲突文件列表和 diff
       ↓
  用户手动解决冲突
       ↓
  更新 SQLite → 展示结果
```

---

## 8. 工作目录结构

```
~/.ai-sync/
├── data.db                              # SQLite 数据库
├── backup/                              # 恢复备份目录
│   └── 20260411-143022/
└── repos/                               # Git 工作目录
    ├── claude-global/                   # project 名称
    │   ├── settings.json                # 保持原始目录结构
    │   ├── .claude/
    │   │   └── CLAUDE.md
    │   └── plugins/
    │       └── config.json
    └── codex-global/                    # 另一个 project
        └── config.yaml
```

远端仓库结构与工作目录一致（git push 后远端看到相同布局）。

---

## 9. 错误处理

| 场景 | 处理方式 |
|------|---------|
| 快照不存在 | 报错："快照 'xxx' 不存在" |
| 未配置远端仓库 | 提示："请先执行 ai-sync remote add <url>" |
| 远端仓库不可达 | 报错："无法连接远端仓库，请检查网络和仓库地址" |
| Push 失败 | 报错展示具体原因，本地数据不受影响 |
| Pull 有冲突 | 展示冲突文件和 diff，等待用户解决 |
| Git 操作失败 | 展示错误信息，建议用户检查工作目录 |
| 工作目录不存在 | 自动创建并初始化 Git 仓库 |

---

## 10. 非功能需求

### NFR-01 安全性

- Git 凭证使用系统原生 Git 凭证管理（credential helper）
- 不在 ai-sync 内存储任何 Git 密码/Token
- push 前自动检查敏感信息（后续阶段可接入"敏感信息检测"）

### NFR-02 性能

- 快照更新（100 个文件）在 3 秒内完成
- Push/Pull 操作主要耗时在网络传输，本地处理 < 2 秒
- 历史查询响应 < 1 秒

### NFR-03 兼容性

- 支持 Windows、macOS、Linux
- 支持 HTTP/HTTPS/SSH 协议的 Git 仓库
- 兼容 GitHub、Gitea、GitLab、自建 Git 服务

### NFR-04 可靠性

- Push 失败不影响本地 SQLite 数据
- Pull 冲突不自动覆盖，必须用户确认
- 工作目录损坏可从 SQLite 重建

---

## 11. 验收标准

| 编号 | 验收条件 | 验证方法 |
|------|---------|---------|
| AC-01 | 更新快照能覆盖 SQLite 中的文件数据 | 创建快照 → 修改配置 → update → 查看快照内容已更新 |
| AC-02 | 更新后 Git 有新的 commit 记录 | update 后查看 git log 有新 commit |
| AC-03 | 无变更时提示"无变更" | 未修改配置 → update → 看到"无变更"提示 |
| AC-04 | 能添加远端仓库 | remote add → remote list 能看到 |
| AC-05 | 能推送到远端 | push → 远端仓库有文件 |
| AC-06 | 能从远端拉取 | 设备 B pull → SQLite 数据更新 |
| AC-07 | 无冲突时自动合并 | 设备 A push → 设备 B pull → 自动合并成功 |
| AC-08 | 有冲突时展示 diff | 两设备改同一文件 → pull → 看到冲突提示 |
| AC-09 | 多 project 共享同一仓库 | 两个 project push 到同一仓库 → 仓库内按目录分开 |
| AC-10 | 历史版本可查看 | snapshot history → 看到多个版本记录 |
| AC-11 | 历史版本可恢复 | restore --history → 选择旧版本 → 配置恢复 |
| AC-12 | Push 失败不影响本地 | 断网 push → 本地 SQLite 数据不变 |

---

## 12. 分阶段交付

| 阶段 | 内容 | 依赖 |
|------|------|------|
| 第一阶段 | 快照恢复（snapshot restore） | 无 |
| 第二阶段 | 快照更新（snapshot update）+ Git 封装 | 第一阶段 |
| 第三阶段 | 远端仓库管理（remote add/list/remove） | 第二阶段 |
| 第四阶段 | 同步操作（sync push/pull）+ 冲突处理 | 第三阶段 |
| 第五阶段 | 历史版本（history + restore --history） | 第四阶段 |

---

## 13. 关联文档

- [快照恢复 PRD](./2026-04-11-snapshot-restore-prd.md) — 第一阶段产品需求
- [快照恢复技术设计](./2026-04-11-snapshot-restore-design.md) — 第一阶段技术架构
