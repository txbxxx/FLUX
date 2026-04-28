package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	typesCommon "flux/internal/types/common"
	typesSetting "flux/internal/types/setting"
)

// AISettingManager AI 配置管理接口。
type AISettingManager interface {
	Create(setting *typesSetting.AISettingRecord) error
	GetByName(name string) (*typesSetting.AISettingRecord, error)
	List() ([]*typesSetting.AISettingRecord, error)
	ListPaginated(limit, offset int) ([]*typesSetting.AISettingRecord, int, error)
	Delete(name string) error
	UpdateByName(oldName string, record *typesSetting.AISettingRecord) error
}

// CreateAISettingInput 创建 AI 配置的输入。
type CreateAISettingInput struct {
	Name    string   // 配置名称，必填
	Token   string   // Auth token，必填
	BaseURL string   // API base URL，必填
	Models  []string // 模型列表，至少1个，最多6个
}

// CreateAISettingResult 创建配置的返回值。
type CreateAISettingResult typesSetting.AISettingCreateResult

// ListAISettingsInput 列出配置的输入。
type ListAISettingsInput struct {
	Limit  int // 分页大小，<=0 时返回全部
	Offset int // 偏移量
}

// ListAISettingsResult 列出配置的返回值。
type ListAISettingsResult typesSetting.AISettingListResult

// GetAISettingInput 获取配置详情的输入。
type GetAISettingInput struct {
	Name string // 配置名称，必填
}

// GetAISettingResult 获取配置详情的返回值。
type GetAISettingResult struct {
	typesSetting.AISettingDetail
	IsCurrent bool // 是否为当前生效配置
}

// DeleteAISettingInput 删除配置的输入。
type DeleteAISettingInput struct {
	Name string // 配置名称，必填
}

// SwitchAISettingInput 切换配置的输入。
type SwitchAISettingInput struct {
	Name   string   // 要切换到的配置名称，必填
	Models []string // 可选：覆盖要激活的模型（最多3个），默认使用配置中存储的模型
}

// SwitchAISettingResult 切换配置的返回值。
type SwitchAISettingResult typesSetting.AISwitchResult

// ClaudeSettingsFile Claude settings.json 文件路径。
const ClaudeSettingsFile = ".claude/settings.json"

// CreateAISetting 创建 AI 配置并保存到数据库。
func (w *LocalWorkflow) CreateAISetting(_ context.Context, input CreateAISettingInput) (*CreateAISettingResult, error) {
	// 参数校验
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, &UserError{
			Message:    "创建配置失败：名称不能为空",
			Suggestion: "请通过 --name 参数指定配置名称",
			Err:        errors.New("empty name"),
		}
	}

	token := strings.TrimSpace(input.Token)
	if token == "" {
		return nil, &UserError{
			Message:    "创建配置失败：token 不能为空",
			Suggestion: "请通过 --token 参数指定认证 token",
			Err:        errors.New("empty token"),
		}
	}

	baseURL := strings.TrimSpace(input.BaseURL)
	if baseURL == "" {
		return nil, &UserError{
			Message:    "创建配置失败：base_url 不能为空",
			Suggestion: "请通过 --api 参数指定 API 地址",
			Err:        errors.New("empty base_url"),
		}
	}

	// 校验 baseURL 格式：必须以 http:// 或 https:// 开头
	// 为什么：离线场景也需可用，因此只做格式校验，不验证域名可达性
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		return nil, &UserError{
			Message:    "创建配置失败：API 地址格式不正确",
			Suggestion: "API 地址必须以 http:// 或 https:// 开头，例如 https://api.anthropic.com",
			Err:        errors.New("invalid base_url format"),
		}
	}

	// 校验模型：至少填一个，最多6个
	models, err := typesSetting.NewModelListFromInput(input.Models)
	if err != nil {
		return nil, &UserError{
			Message:    "创建配置失败：" + err.Error(),
			Suggestion: "请通过 --model 参数指定至少一个模型（最多6个）",
			Err:        err,
		}
	}

	// 检查配置是否已存在
	if w.aiSettingManager != nil {
		existing, err := w.aiSettingManager.GetByName(name)
		if err == nil && existing != nil {
			return nil, &UserError{
				Message:    "创建配置失败：配置名称已存在",
				Suggestion: fmt.Sprintf("配置 %q 已存在，请使用其他名称或先删除现有配置", name),
				Err:        errors.New("duplicate name"),
			}
		}
	}

	// 创建配置
	setting := &typesSetting.AISettingRecord{
		ID:        0, // GORM 自动生成自增 ID
		Name:      name,
		Token:     token,
		BaseURL:   baseURL,
		Models:    models,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if w.aiSettingManager == nil {
		return nil, &UserError{
			Message:    "创建配置失败：数据库未初始化",
			Suggestion: "请检查应用数据目录是否正常",
			Err:        errors.New("ai setting manager not initialized"),
		}
	}

	if err := w.aiSettingManager.Create(setting); err != nil {
		return nil, &UserError{
			Message:    "创建配置失败",
			Suggestion: "请检查数据库连接后重试",
			Err:        err,
		}
	}

	return &CreateAISettingResult{ID: setting.ID}, nil
}

// ListAISettings 列出所有已保存的 AI 配置。
func (w *LocalWorkflow) ListAISettings(_ context.Context, input ListAISettingsInput) (*ListAISettingsResult, error) {
	if w.aiSettingManager == nil {
		return nil, &UserError{
			Message:    "读取配置列表失败：数据库未初始化",
			Suggestion: "请检查应用数据目录是否正常",
			Err:        errors.New("ai setting manager not initialized"),
		}
	}

	// 获取当前生效配置的 token + base URL，用于和数据库中的配置逐一匹配
	currentInfo, _ := w.getCurrentSettingInfo()

	// 使用 SQL 层分页查询，避免全量加载到内存
	limit := input.Limit
	offset := input.Offset
	if offset < 0 {
		offset = 0
	}

	settings, total, err := w.aiSettingManager.ListPaginated(limit, offset)
	if err != nil {
		return nil, &UserError{
			Message:    "读取配置列表失败",
			Suggestion: "请检查数据库连接后重试",
			Err:        err,
		}
	}

	// 转换为返回结构体
	var currentMatchedName string
	items := make([]typesSetting.AISettingListItem, 0, len(settings))
	for _, setting := range settings {
		isCurrent := currentInfo != nil && setting.Token == currentInfo.Token && setting.BaseURL == currentInfo.BaseURL
		if isCurrent {
			currentMatchedName = setting.Name
		}

		items = append(items, typesSetting.AISettingListItem{
			ID:          setting.ID,
			Name:        setting.Name,
			BaseURL:     setting.BaseURL,
			Models:    setting.Models,
			IsCurrent:   isCurrent,
		})
	}

	// 如果当前页未匹配到 current，从全部配置中查找
	// 为什么：分页查询可能不包含当前生效的配置，但调用方始终需要 Current 字段
	if currentMatchedName == "" {
		currentMatchedName = w.matchCurrentSettingName()
	}

	return &ListAISettingsResult{
		Items:   items,
		Total:   total,
		Current: currentMatchedName,
	}, nil
}

// GetAISetting 获取指定配置的详情。
func (w *LocalWorkflow) GetAISetting(_ context.Context, input GetAISettingInput) (*GetAISettingResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, &UserError{
			Message:    "获取配置失败：名称不能为空",
			Suggestion: "请指定配置名称",
			Err:        errors.New("empty name"),
		}
	}

	if w.aiSettingManager == nil {
		return nil, &UserError{
			Message:    "获取配置失败：数据库未初始化",
			Suggestion: "请检查应用数据目录是否正常",
			Err:        errors.New("ai setting manager not initialized"),
		}
	}

	setting, err := w.aiSettingManager.GetByName(name)
	if err != nil {
		return nil, &UserError{
			Message:    "获取配置失败：配置不存在",
			Suggestion: "请检查配置名称是否正确",
			Err:        err,
		}
	}

	// 判断是否为当前配置（通过 token + base URL 匹配）
	currentInfo, _ := w.getCurrentSettingInfo()
	isCurrent := currentInfo != nil && setting.Token == currentInfo.Token && setting.BaseURL == currentInfo.BaseURL

	return &GetAISettingResult{
		AISettingDetail: typesSetting.AISettingDetail{
			ID:          setting.ID,
			Name:        setting.Name,
			Token:       setting.Token,
			BaseURL:     setting.BaseURL,
			Models: setting.Models,
			CreatedAt:   setting.CreatedAt,
			UpdatedAt:   setting.UpdatedAt,
		},
		IsCurrent: isCurrent,
	}, nil
}

// DeleteAISetting 删除指定的 AI 配置。
func (w *LocalWorkflow) DeleteAISetting(_ context.Context, input DeleteAISettingInput) error {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return &UserError{
			Message:    "删除配置失败：名称不能为空",
			Suggestion: "请指定配置名称",
			Err:        errors.New("empty name"),
		}
	}

	if w.aiSettingManager == nil {
		return &UserError{
			Message:    "删除配置失败：数据库未初始化",
			Suggestion: "请检查应用数据目录是否正常",
			Err:        errors.New("ai setting manager not initialized"),
		}
	}

	if err := w.aiSettingManager.Delete(name); err != nil {
		if errors.Is(err, typesCommon.ErrRecordNotFound) {
			return &UserError{
				Message:    "删除配置失败：配置不存在",
				Suggestion: "请检查配置名称是否正确",
			}
		}
		return &UserError{
			Message:    "删除配置失败",
			Suggestion: "请检查数据库连接后重试",
			Err:        err,
		}
	}

	return nil
}

// SwitchAISetting 切换到指定的 AI 配置。
// 过程：1. 备份当前 settings.json 为 .json.ats；2. 从数据库读取新配置；3. 写入 settings.json。
func (w *LocalWorkflow) SwitchAISetting(_ context.Context, input SwitchAISettingInput) (*SwitchAISettingResult, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, &UserError{
			Message:    "切换配置失败：名称不能为空",
			Suggestion: "请指定配置名称",
			Err:        errors.New("empty name"),
		}
	}

	if w.aiSettingManager == nil {
		return nil, &UserError{
			Message:    "切换配置失败：数据库未初始化",
			Suggestion: "请检查应用数据目录是否正常",
			Err:        errors.New("ai setting manager not initialized"),
		}
	}

	// 第一步：获取目标配置
	target, err := w.aiSettingManager.GetByName(name)
	if err != nil {
		return nil, &UserError{
			Message:    "切换配置失败：配置不存在",
			Suggestion: "请检查配置名称是否正确",
			Err:        err,
		}
	}

	// 第二步：读取当前配置（用于返回 previous_name）
	settingsPath, err := w.getClaudeSettingsPath()
	if err != nil {
		return nil, &UserError{
			Message:    "切换配置失败：无法定位 settings.json",
			Suggestion: "请确认 Claude 配置目录存在",
			Err:        err,
		}
	}

	// 通过 token + base URL 匹配找到之前的配置名称
	previousName := w.matchCurrentSettingName()

	// 第三步：备份当前 settings.json
	backupPath := settingsPath + ".ats"
	if err := backupSettingsFile(settingsPath, backupPath); err != nil {
		return nil, &UserError{
			Message:    "切换配置失败：备份失败",
			Suggestion: "请检查文件写入权限",
			Err:        err,
		}
	}

	// 确定要激活的模型列表
	targetModels := target.Models
	if len(input.Models) > 0 {
		var err error
		targetModels, err = typesSetting.NewSwitchModelListFromInput(input.Models)
		if err != nil {
			return nil, &UserError{
				Message:    "切换配置失败：" + err.Error(),
				Suggestion: "请指定有效的模型列表",
				Err:        err,
			}
		}
	}

	// 第四步：构建新的 settings.json 内容
	newSettings := map[string]any{
		"env": map[string]string{
			"ANTHROPIC_AUTH_TOKEN": target.Token,
			"ANTHROPIC_BASE_URL":   target.BaseURL,
		},
	}
	// 添加模型环境变量
	for key, value := range targetModels.ToEnvVars() {
		newSettings["env"].(map[string]string)[key] = value
	}

	// 第五步：读取现有配置并保留其他字段
	if rawContent, err := os.ReadFile(settingsPath); err == nil {
		// 清理可能存在的空字节和控制字符，防止 JSON 解析失败导致合并逻辑被跳过
		// 为什么：get -e 编辑器可能在文件中引入空字节，导致 json.Unmarshal 失败，
		// 而 if err == nil 会静默跳过合并，造成其他配置字段丢失。
		content := []byte(sanitizeContent(string(rawContent)))
		var existing map[string]any
		if err := json.Unmarshal(content, &existing); err == nil {
			// 合并 env 字段
			newEnv, ok := newSettings["env"].(map[string]string)
			if !ok {
				newEnv = make(map[string]string)
			}
			if env, ok := existing["env"].(map[string]any); ok {
				for k, v := range env {
					if _, exists := newEnv[k]; !exists {
						newEnv[k] = fmt.Sprint(v)
					}
				}
			}
			newSettings["env"] = newEnv
			// 保留其他字段（如 enabledPlugins, language 等）
			for k, v := range existing {
				if k != "env" {
					newSettings[k] = v
				}
			}
		}
	}

	// 第六步：写入新配置
	newContent, err := json.MarshalIndent(newSettings, "", "  ")
	if err != nil {
		return nil, &UserError{
			Message:    "切换配置失败：生成配置内容失败",
			Suggestion: "请检查配置数据是否有效",
			Err:        err,
		}
	}

	if err := atomicWriteFile(settingsPath, newContent, 0644); err != nil {
		return nil, &UserError{
			Message:    "切换配置失败：写入配置文件失败",
			Suggestion: "请检查文件写入权限",
			Err:        err,
		}
	}

	return &SwitchAISettingResult{
		PreviousName: previousName,
		NewName:      target.Name,
		BackupPath:   backupPath,
	}, nil
}
// WithAISettingManager 以链式方式补充 AI 配置管理依赖。
func (w *LocalWorkflow) WithAISettingManager(manager AISettingManager) *LocalWorkflow {
	w.aiSettingManager = manager
	return w
}

// getClaudeSettingsPath 获取 Claude settings.json 文件的绝对路径。
func (w *LocalWorkflow) getClaudeSettingsPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户主目录失败: %w", err)
	}

	settingsPath := filepath.Join(homeDir, ClaudeSettingsFile)
	return settingsPath, nil
}

// currentSettingInfo holds the token and base URL read from the active settings.json.
type currentSettingInfo struct {
	Token   string
	BaseURL string
}

// getCurrentSettingInfo reads the active settings.json and extracts token + base URL.
func (w *LocalWorkflow) getCurrentSettingInfo() (*currentSettingInfo, error) {
	settingsPath, err := w.getClaudeSettingsPath()
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil, err
	}

	// 清理可能存在的空字节和控制字符，防止 JSON 解析失败
	content = []byte(sanitizeContent(string(content)))
	return parseCurrentSettingInfo(content)
}

// parseCurrentSettingInfo extracts token and base URL from settings.json content.
func parseCurrentSettingInfo(content []byte) (*currentSettingInfo, error) {
	var settings map[string]any
	if err := json.Unmarshal(content, &settings); err != nil {
		return nil, err
	}

	env, ok := settings["env"].(map[string]any)
	if !ok {
		return nil, nil
	}

	info := &currentSettingInfo{}
	if token, ok := env["ANTHROPIC_AUTH_TOKEN"].(string); ok {
		info.Token = token
	}
	if baseURL, ok := env["ANTHROPIC_BASE_URL"].(string); ok {
		info.BaseURL = baseURL
	}

	if info.Token == "" {
		return nil, nil
	}

	return info, nil
}

// matchCurrentSettingName reads settings.json and finds the matching config name from DB.
func (w *LocalWorkflow) matchCurrentSettingName() string {
	info, err := w.getCurrentSettingInfo()
	if err != nil || info == nil {
		return ""
	}

	settings, err := w.aiSettingManager.List()
	if err != nil {
		return ""
	}

	for _, s := range settings {
		if s.Token == info.Token && s.BaseURL == info.BaseURL {
			return s.Name
		}
	}

	return ""
}

// backupSettingsFile 备份 settings.json 文件。
func backupSettingsFile(src, dst string) error {
	// 如果源文件不存在，不报错（首次切换时可能没有）
	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	content, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, content, 0644)
}

// atomicWriteFile writes data to path atomically by writing to a temp file then renaming.
// 为什么：直接 os.WriteFile 在崩溃时会截断文件，先写临时文件再 rename 保证原子性。
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("创建临时文件失败: %w", err)
	}
	tempPath := tempFile.Name()
	defer func() {
		_ = os.Remove(tempPath)
	}()

	if _, err := tempFile.Write(data); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tempFile.Chmod(perm); err != nil {
		_ = tempFile.Close()
		return fmt.Errorf("设置文件权限失败: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("关闭临时文件失败: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("替换原文件失败: %w", err)
	}

	return nil
}

// GetAISettingsBatchInput 批量获取配置的输入。
type GetAISettingsBatchInput struct {
	Names []string // 配置名称列表，至少一个
}

// GetAISettingsBatchResult 批量获取配置的返回值。
type GetAISettingsBatchResult struct {
	Items  []*GetAISettingResult // 成功获取的配置列表
	Failed []string              // 获取失败的配置名称列表
}

// DeleteAISettingsBatchInput 批量删除配置的输入。
type DeleteAISettingsBatchInput struct {
	Names []string // 配置名称列表，至少一个
}

// DeleteAISettingsBatchResult 批量删除配置的返回值。
type DeleteAISettingsBatchResult struct {
	Deleted []string // 成功删除的配置名称列表
	Failed  []string // 删除失败的配置名称列表（含原因）
}

// GetAISettingsBatch 批量获取多个配置的详情。
// 循环调用 GetAISetting，收集成功和失败的结果。
func (w *LocalWorkflow) GetAISettingsBatch(ctx context.Context, input GetAISettingsBatchInput) (*GetAISettingsBatchResult, error) {
	// 参数校验：至少一个名称
	if len(input.Names) == 0 {
		return nil, &UserError{
			Message:    "批量获取配置失败：至少需要指定一个配置名称",
			Suggestion: "请提供至少一个配置名称",
			Err:        errors.New("empty names"),
		}
	}

	// 去重
	nameMap := make(map[string]bool)
	for _, name := range input.Names {
		nameMap[name] = true
	}
	uniqueNames := make([]string, 0, len(nameMap))
	for name := range nameMap {
		uniqueNames = append(uniqueNames, name)
	}

	result := &GetAISettingsBatchResult{
		Items:  make([]*GetAISettingResult, 0),
		Failed: make([]string, 0),
	}

	for _, name := range uniqueNames {
		singleResult, err := w.GetAISetting(ctx, GetAISettingInput{Name: name})
		if err != nil {
			result.Failed = append(result.Failed, name)
			continue
		}
		result.Items = append(result.Items, singleResult)
	}

	return result, nil
}

// DeleteAISettingsBatch 批量删除多个配置。
// 循环调用 DeleteAISetting，收集成功和失败的结果。
func (w *LocalWorkflow) DeleteAISettingsBatch(ctx context.Context, input DeleteAISettingsBatchInput) (*DeleteAISettingsBatchResult, error) {
	// 参数校验：至少一个名称
	if len(input.Names) == 0 {
		return nil, &UserError{
			Message:    "批量删除配置失败：至少需要指定一个配置名称",
			Suggestion: "请提供至少一个配置名称",
			Err:        errors.New("empty names"),
		}
	}

	// 去重
	nameMap := make(map[string]bool)
	for _, name := range input.Names {
		nameMap[name] = true
	}
	uniqueNames := make([]string, 0, len(nameMap))
	for name := range nameMap {
		uniqueNames = append(uniqueNames, name)
	}

	result := &DeleteAISettingsBatchResult{
		Deleted: make([]string, 0),
		Failed:  make([]string, 0),
	}

	for _, name := range uniqueNames {
		err := w.DeleteAISetting(ctx, DeleteAISettingInput{Name: name})
		if err != nil {
			result.Failed = append(result.Failed, fmt.Sprintf("%s: %s", name, extractErrorMessage(err)))
			continue
		}
		result.Deleted = append(result.Deleted, name)
	}

	return result, nil
}

// extractErrorMessage 从错误中提取用户可读的错误信息
func extractErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	var userErr *UserError
	if errors.As(err, &userErr) {
		return userErr.Message
	}
	return err.Error()
}

// EditAISettingInput 编辑配置的输入。
type EditAISettingInput struct {
	Name        string // 配置名称，必填
	NewName string   // 新名称，可选
	Token   string   // 新 Token，可选
	BaseURL string   // 新 API 地址，可选
	Models  []string // 新模型列表，可选
}

// EditAISettingResult 编辑配置的返回值。
type EditAISettingResult struct {
	ID        uint
	Name      string
	UpdatedAt time.Time
	Changes   []typesSetting.FieldChange
	IsCurrent bool
}

// EditAISetting 编辑 AI 配置。
func (w *LocalWorkflow) EditAISetting(_ context.Context, input EditAISettingInput) (*EditAISettingResult, error) {
	// 参数校验
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, &UserError{
			Message:    "编辑配置失败：名称不能为空",
			Suggestion: "请指定要编辑的配置名称",
			Err:        errors.New("empty name"),
		}
	}

	if w.aiSettingManager == nil {
		return nil, &UserError{
			Message:    "编辑配置失败：数据库未初始化",
			Suggestion: "请检查应用数据目录是否正常",
			Err:        errors.New("ai setting manager not initialized"),
		}
	}

	// 读取现有配置
	existing, err := w.aiSettingManager.GetByName(name)
	if err != nil {
		return nil, &UserError{
			Message:    "编辑配置失败：配置不存在",
			Suggestion: "请检查配置名称是否正确",
			Err:        err,
		}
	}

	// 判断是否为当前配置
	currentInfo, _ := w.getCurrentSettingInfo()
	isCurrent := currentInfo != nil && existing.Token == currentInfo.Token && existing.BaseURL == currentInfo.BaseURL

	// 构建更新数据
	updated := &typesSetting.AISettingRecord{
		ID:        existing.ID,
		Name:      existing.Name,
		Token:     existing.Token,
		BaseURL:   existing.BaseURL,
		Models:    existing.Models,
		CreatedAt: existing.CreatedAt,
		UpdatedAt: time.Now(),
	}

	changes := make([]typesSetting.FieldChange, 0)

	// 处理新名称
	if input.NewName != "" {
		newName := strings.TrimSpace(input.NewName)
		if newName != existing.Name {
			// 检查新名称是否已存在
			if conflict, _ := w.aiSettingManager.GetByName(newName); conflict != nil && conflict.ID != existing.ID {
				return nil, &UserError{
					Message:    "编辑配置失败：配置名称已存在",
					Suggestion: fmt.Sprintf("配置 %q 已存在，请使用其他名称", newName),
					Err:        errors.New("duplicate name"),
				}
			}
			changes = append(changes, typesSetting.FieldChange{
				Field:    "name",
				OldValue: existing.Name,
				NewValue: newName,
			})
			updated.Name = newName
		}
	}

	// 处理 Token
	// <unchanged> 占位符表示保持原值（编辑器模式）
	// 空字符串表示不修改（命令行模式，保持向后兼容）
	if input.Token != "" && input.Token != "<unchanged>" {
		// 新值：比较是否真的有变化
		if input.Token != existing.Token {
			changes = append(changes, typesSetting.FieldChange{
				Field:    "token",
				OldValue: maskTokenForDisplay(existing.Token),
				NewValue: maskTokenForDisplay(input.Token),
			})
			updated.Token = input.Token
		}
	} else {
		// <unchanged> 或空字符串：保持原值
		updated.Token = existing.Token
	}

	// 处理 BaseURL
	if input.BaseURL != "" && input.BaseURL != "<unchanged>" {
		baseURL := strings.TrimSpace(input.BaseURL)
		// 新值：比较是否真的有变化
		if baseURL != "" && baseURL != existing.BaseURL {
			if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
				return nil, &UserError{
					Message:    "编辑配置失败：API 地址格式不正确",
					Suggestion: "API 地址必须以 http:// 或 https:// 开头",
					Err:        errors.New("invalid base_url format"),
				}
			}
			changes = append(changes, typesSetting.FieldChange{
				Field:    "base_url",
				OldValue: existing.BaseURL,
				NewValue: baseURL,
			})
			updated.BaseURL = baseURL
		}
	} else {
		// <unchanged> 或空字符串：保持原值
		updated.BaseURL = existing.BaseURL
	}

	// 处理 Models
	if len(input.Models) > 0 {
		newModels, err := typesSetting.NewModelListFromInput(input.Models)
		if err != nil {
			return nil, &UserError{
				Message:    "编辑配置失败：" + err.Error(),
				Suggestion: "请指定有效的模型列表（1-6个）",
				Err:        err,
			}
		}
		// 比较是否真的有变化
		if !modelsEqual(newModels, existing.Models) {
			changes = append(changes, typesSetting.FieldChange{
				Field:    "models",
				OldValue: strings.Join(existing.Models, ", "),
				NewValue: strings.Join(newModels, ", "),
			})
			updated.Models = newModels
		}
	} else {
		// 空列表：保持原值
		updated.Models = existing.Models
	}

	// 如果没有任何变更
	if len(changes) == 0 {
		return &EditAISettingResult{
			ID:        existing.ID,
			Name:      existing.Name,
			UpdatedAt: existing.UpdatedAt,
			Changes:   nil,
			IsCurrent: isCurrent,
		}, nil
	}

	// 执行更新
	if err := w.aiSettingManager.UpdateByName(name, updated); err != nil {
		return nil, &UserError{
			Message:    "编辑配置失败",
			Suggestion: "请检查数据库连接后重试",
			Err:        err,
		}
	}

	return &EditAISettingResult{
		ID:        updated.ID,
		Name:      updated.Name,
		UpdatedAt: updated.UpdatedAt,
		Changes:   changes,
		IsCurrent: isCurrent,
	}, nil
}

// maskTokenForDisplay 脱敏显示 token（复用现有逻辑）。
func maskTokenForDisplay(token string) string {
	if len(token) > 8 {
		return token[:4] + "****" + token[len(token)-4:]
	}
	if len(token) > 0 {
		return "****"
	}
	return ""
}

// modelsEqual 比较两个模型列表是否相等。
func modelsEqual(a, b typesSetting.ModelList) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
