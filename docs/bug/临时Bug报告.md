## 概述

本报告分析AToSync项目中两个P0级别的Bug，均为编辑器模式相关的问题。

---

## Bug#1: setting edit -e后Token变成加密显示格式

### Bug信息
- 状态: 未解决
- 优先级: P0
- 终端: Windows
- 修复版本: V1.1

### 问题描述

使用ai-sync setting edit GLM_lite -e编辑配置后，数据库中保存的token值变成了加密显示格式（如4e5b****QEXh），导致实际使用时token无效。

### 根本原因分析

问题代码位置: internal/tui/setting_editor_model.go

问题流程:
1. 读取阶段: GetAISetting返回完整的token
2. 显示阶段: maskTokenForEditor将token转换为脱敏格式并显示
3. 编辑阶段: 用户如果没有修改Token字段，输入框中保持脱敏值
4. 保存阶段: saveChanges直接读取输入框值，将脱敏值保存到数据库

### 修复方案

在saveChanges函数中添加逻辑：当Token值与脱敏格式匹配时，使用原始完整token。

---

## Bug#2: get -e后编辑文件显示为二进制

### Bug信息
- 状态: 未解决
- 优先级: P0
- 终端: Windows
- 修复版本: V1.1

### 问题描述

执行ai-sync get claude-global settings.json -e后，编辑的文件内容会以异常方式显示。

### 可能原因

1. 特殊字符渲染问题
2. 行尾处理问题
3. 终端编码问题

### 修复方案

在读取文件后、传递给编辑器前，清理特殊字符。

---

## 修复优先级

- Bug#1: P0，修复难度低，建议优先修复
- Bug#2: P0，修复难度中，建议第二优先级
