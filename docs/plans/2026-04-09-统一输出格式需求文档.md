# CLI 统一输出层 PRD

## 背景

当前三个 list 命令（setting list、snapshot list、scan list）各自手工拼接字符串渲染输出，导致：

1. **视觉风格不统一**：setting list 是缩进列表，snapshot list 是表格，scan list 是分组段落
2. **代码重复**：displayWidth、printAlignedRow 等对齐逻辑分散在各命令中
3. **无色彩**：三个命令都是纯文本，缺乏视觉层次
4. **扩展成本高**：新增 list 命令时没有可复用的渲染组件

## 目标

1. 建立统一的 CLI 输出渲染层，所有 list 命令风格一致
2. 全部采用表格视图，保持整洁对齐
3. 极简色彩：表头加粗、状态/高亮蓝色、次要信息灰色
4. scan list 精简为表格，详情通过 `--verbose` flag 查看
5. 消除渲染代码重复，新命令只需组装数据

## 非目标

- 不改动 TUI 的渲染逻辑
- 不引入新的第三方依赖（使用已有的 lipgloss）
- 不改变命令参数和业务逻辑，只改渲染层
- 不做交互式表格（分页、排序等）

## 功能需求

### F1：通用表格渲染器

- 支持动态列宽自动计算（兼容中英文混排）
- 支持 Unicode box-drawing 边框（┌─┬┐ 系列）
- 支持行高亮（指定行用蓝色前景）
- 支持表头加粗
- 表格下方可追加汇总文字（灰色）

### F2：setting list 表格化

当前输出：
```
配置列表（共 2 个）

* 火山
    Base URL: https://ark.cn-beijing.volces.com/api/coding
    Opus 模型: ark-code-latest
    Sonnet 模型: glm-4.7
    (当前生效)

  glm-lite
    Base URL: https://open.bigmodel.cn/api/anthropic
    Opus 模型: glm-5.1
    Sonnet 模型: glm-4.7

当前生效配置: 火山
```

目标输出（示意）：
```
┌──────────┬──────────────────────────────────────────┬──────────────┬──────────────┐
│ 名称      │ Base URL                                  │ Opus 模型     │ Sonnet 模型  │
├──────────┼──────────────────────────────────────────┼──────────────┼──────────────┤
│ *火山     │ https://ark.cn-beijing.volces.com/api/... │ ark-code-... │ glm-4.7      │
│ glm-lite  │ https://open.bigmodel.cn/api/...          │ glm-5.1      │ glm-4.7      │
└──────────┴──────────────────────────────────────────┴──────────────┴──────────────┘
当前生效配置: 火山
```

- 当前生效行名称前加 `*`，整行高亮
- Token 不再单独列（已有脱敏逻辑，可保留在 detail 视图中）

### F3：snapshot list 统一

当前已是表格风格，迁移到新渲染器即可，视觉变化不大：
- 加边框
- 表头加粗

### F4：scan list 精简表格化

当前输出：
```
claude-global（Claude 项目）
  检测结果: 可同步
  配置目录: C:\Users\28652\.claude
  可同步项: 6 项
  关键配置:
    - 主配置: settings.json
  ...
```

目标输出（默认精简）：
```
┌───────────────┬────────┬───────────────────────┬────────┬──────────┐
│ 项目           │ 类型   │ 配置目录                │ 状态    │ 可同步项  │
├───────────────┼────────┼───────────────────────┼────────┼──────────┤
│ claude-global  │ Claude │ C:\Users\28652\.claude │ 可同步  │ 6 项     │
│ codex-global   │ Codex  │ C:\Users\28652\.codex  │ 可同步  │ 4 项     │
└───────────────┴────────┴───────────────────────┴────────┴──────────┘
```

`scan list --verbose` / `scan list -v`：表格下方追加每个项目的详细配置项列表（关键配置、扩展内容等）。

### F5：全局样式常量

| 用途 | 样式 |
|------|------|
| 表头 | 加粗 |
| 高亮/状态 | 蓝色前景 (color 12) |
| 次要信息 | 灰色前景 (color 241) |
| 边框 | 无颜色，Unicode box-drawing |

## 约束

- 仅使用项目已有的 lipgloss v1.1.0，不引入新依赖
- 兼容 Windows 终端（避免依赖特殊终端能力）
- 保持非交互命令的输出可通过管道传递

## 验收标准

- [ ] setting list、snapshot list、scan list 三个命令输出风格一致
- [ ] 所有表格有统一边框和对齐
- [ ] 高亮行、状态文字有颜色区分
- [ ] scan list 支持 `--verbose` / `-v` flag
- [ ] 现有测试全部通过
- [ ] 新增 output 包有单元测试覆盖
- [ ] displayWidth / printAlignedRow 从 root.go 迁移到 output 包
