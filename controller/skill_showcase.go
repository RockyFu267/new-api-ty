package controller

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const showcaseSkillID = "showcase-designer"

var showcaseConfigMu sync.Mutex

type showcaseMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Trend string `json:"trend,omitempty"`
}

type showcaseConfig struct {
	Version      int64            `json:"version"`
	Template     string           `json:"template"`
	Title        string           `json:"title"`
	Subtitle     string           `json:"subtitle"`
	Announcement string           `json:"announcement"`
	Metrics      []showcaseMetric `json:"metrics"`
	Highlights   []string         `json:"highlights"`
	UpdatedAt    string           `json:"updated_at"`
}

type showcaseChanges struct {
	Title        *string           `json:"title"`
	Subtitle     *string           `json:"subtitle"`
	Announcement *string           `json:"announcement"`
	Metrics      *[]showcaseMetric `json:"metrics"`
	Highlights   *[]string         `json:"highlights"`
}

type showcaseDirective struct {
	Action   string          `json:"action"`
	Template string          `json:"template"`
	Changes  showcaseChanges `json:"changes"`
}

type showcaseSkillDefinition struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Description  string `json:"description"`
	SystemPrompt string `json:"system_prompt"`
	Enabled      bool   `json:"enabled"`
}

var showcaseTemplates = map[string]showcaseConfig{
	"cyber-blue":       {Template: "cyber-blue", Title: "TIANYAN 智能运维中心", Subtitle: "让企业基础设施状态一目了然", Announcement: "所有核心服务运行正常", Metrics: []showcaseMetric{{"在线集群", "12", "+2 本月"}, {"运行工作负载", "1,286", "99.98% 可用"}, {"AI 自动化任务", "368", "+24%"}}, Highlights: []string{"多云资源统一治理", "智能告警关联分析", "自然语言驱动运维"}},
	"executive-dark":   {Template: "executive-dark", Title: "企业数字化运营驾驶舱", Subtitle: "从数据到决策，只需一次对话", Announcement: "本季度运营效率提升 23%", Metrics: []showcaseMetric{{"受管资产", "8,642", "+12%"}, {"自动化覆盖率", "87%", "+9%"}, {"节省工时", "2,480h", "本季度"}}, Highlights: []string{"管理层全景视图", "关键指标实时洞察", "业务风险提前预警"}},
	"operations-green": {Template: "operations-green", Title: "绿色智能基础设施", Subtitle: "稳定、高效、可持续的运维体验", Announcement: "今日自动修复 18 项潜在故障", Metrics: []showcaseMetric{{"健康节点", "256", "100%"}, {"能源效率", "PUE 1.18", "优秀"}, {"故障恢复", "42s", "平均 MTTR"}}, Highlights: []string{"容量趋势智能预测", "故障自愈编排", "能耗与成本协同优化"}},
	"aurora-purple":    {Template: "aurora-purple", Title: "AI 能力创新展示中心", Subtitle: "知识、模型与 Skill 的企业级融合", Announcement: "TIANYAN Demo 已进入实时演示模式", Metrics: []showcaseMetric{{"接入模型", "16", "多模型"}, {"企业知识", "3.2M", "文档片段"}, {"可用 Skill", "28", "+6"}}, Highlights: []string{"模型按需调用", "企业知识增强", "受控 Skill 自动执行"}},
}

func showcaseConfigPath() string {
	return showcaseDemoFilePath("skill-showcase.json")
}

func showcaseSkillDefinitionPath() string {
	return showcaseDemoFilePath("showcase-skill.json")
}

func showcaseDemoFilePath(name string) string {
	for _, root := range []string{"demo", filepath.Join("..", "demo")} {
		path := filepath.Join(root, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join("demo", name)
}

func loadShowcaseSkillDefinition() (showcaseSkillDefinition, error) {
	var definition showcaseSkillDefinition
	data, err := os.ReadFile(showcaseSkillDefinitionPath())
	if err != nil {
		return definition, err
	}
	err = common.Unmarshal(data, &definition)
	return definition, err
}

func saveShowcaseSkillDefinition(definition showcaseSkillDefinition) error {
	data, err := common.Marshal(definition)
	if err != nil {
		return err
	}
	return os.WriteFile(showcaseSkillDefinitionPath(), data, 0o644)
}

func GetShowcaseSkillAdmin(c *gin.Context) {
	showcaseConfigMu.Lock()
	defer showcaseConfigMu.Unlock()
	definition, err := loadShowcaseSkillDefinition()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, definition)
}

func UpdateShowcaseSkillAdmin(c *gin.Context) {
	var input showcaseSkillDefinition
	if err := c.ShouldBindJSON(&input); err != nil {
		common.ApiError(c, err)
		return
	}
	input.ID = showcaseSkillID
	input.Name = strings.TrimSpace(input.Name)
	input.Description = strings.TrimSpace(input.Description)
	input.SystemPrompt = strings.TrimSpace(input.SystemPrompt)
	if input.Name == "" || input.SystemPrompt == "" {
		common.ApiError(c, errors.New("Skill 名称和系统提示词不能为空"))
		return
	}
	if validateShowcaseText(input.Name, 60) != nil || validateShowcaseText(input.Description, 300) != nil || validateShowcaseText(input.SystemPrompt, 4000) != nil {
		common.ApiError(c, errors.New("Skill 配置超过长度限制"))
		return
	}
	showcaseConfigMu.Lock()
	defer showcaseConfigMu.Unlock()
	if err := saveShowcaseSkillDefinition(input); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, input)
}

func GetAvailableShowcaseSkill(c *gin.Context) {
	showcaseConfigMu.Lock()
	defer showcaseConfigMu.Unlock()
	definition, err := loadShowcaseSkillDefinition()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !definition.Enabled {
		common.ApiSuccess(c, nil)
		return
	}
	common.ApiSuccess(c, gin.H{"id": definition.ID, "name": definition.Name, "description": definition.Description})
}

func validateShowcaseText(value string, max int) error {
	if len([]rune(strings.TrimSpace(value))) > max {
		return errors.New("展示内容超过长度限制")
	}
	return nil
}

func loadShowcaseConfig() (showcaseConfig, error) {
	var config showcaseConfig
	data, err := os.ReadFile(showcaseConfigPath())
	if err != nil {
		return config, err
	}
	err = common.Unmarshal(data, &config)
	return config, err
}

func saveShowcaseConfig(config showcaseConfig) error {
	config.Version = time.Now().UnixMilli()
	config.UpdatedAt = time.Now().Format(time.RFC3339)
	data, err := common.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(showcaseConfigPath(), data, 0o644)
}

func GetSkillShowcase(c *gin.Context) {
	showcaseConfigMu.Lock()
	defer showcaseConfigMu.Unlock()
	config, err := loadShowcaseConfig()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"config": config, "templates": []string{"cyber-blue", "executive-dark", "operations-green", "aurora-purple"}})
}

func ApplySkillShowcase(c *gin.Context) {
	var directive showcaseDirective
	if err := c.ShouldBindJSON(&directive); err != nil {
		common.ApiError(c, err)
		return
	}
	if directive.Action != "update_showcase" {
		common.ApiError(c, errors.New("不支持的 Skill 动作"))
		return
	}
	showcaseConfigMu.Lock()
	defer showcaseConfigMu.Unlock()
	config, err := loadShowcaseConfig()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if directive.Template != "" {
		preset, ok := showcaseTemplates[directive.Template]
		if !ok {
			common.ApiError(c, errors.New("不存在该展示模板"))
			return
		}
		config = preset
	}
	if directive.Changes.Title != nil {
		config.Title = strings.TrimSpace(*directive.Changes.Title)
	}
	if directive.Changes.Subtitle != nil {
		config.Subtitle = strings.TrimSpace(*directive.Changes.Subtitle)
	}
	if directive.Changes.Announcement != nil {
		config.Announcement = strings.TrimSpace(*directive.Changes.Announcement)
	}
	if directive.Changes.Metrics != nil {
		config.Metrics = *directive.Changes.Metrics
	}
	if directive.Changes.Highlights != nil {
		config.Highlights = *directive.Changes.Highlights
	}
	if err = validateShowcaseText(config.Title, 60); err == nil {
		err = validateShowcaseText(config.Subtitle, 120)
	}
	if err == nil {
		err = validateShowcaseText(config.Announcement, 120)
	}
	if err != nil || len(config.Metrics) > 6 || len(config.Highlights) > 8 {
		if err == nil {
			err = errors.New("展示项目数量超过限制")
		}
		common.ApiError(c, err)
		return
	}
	for _, metric := range config.Metrics {
		if validateShowcaseText(metric.Label, 30) != nil || validateShowcaseText(metric.Value, 30) != nil || validateShowcaseText(metric.Trend, 40) != nil {
			common.ApiError(c, errors.New("指标内容超过长度限制"))
			return
		}
	}
	for _, item := range config.Highlights {
		if validateShowcaseText(item, 60) != nil {
			common.ApiError(c, errors.New("亮点内容超过长度限制"))
			return
		}
	}
	if err = saveShowcaseConfig(config); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, config)
}

func injectPlaygroundSkill(c *gin.Context) error {
	var request struct {
		SkillID string `json:"skill_id"`
	}
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		return err
	}
	if request.SkillID == "" {
		return nil
	}
	if request.SkillID != showcaseSkillID {
		return errors.New("未知的 Skill")
	}
	showcaseConfigMu.Lock()
	definition, err := loadShowcaseSkillDefinition()
	showcaseConfigMu.Unlock()
	if err != nil {
		return err
	}
	if !definition.Enabled {
		return errors.New("该 Skill 已被管理员停用")
	}
	var raw map[string]interface{}
	if err := common.UnmarshalBodyReusable(c, &raw); err != nil {
		return err
	}
	messages, _ := raw["messages"].([]interface{})
	prompt := definition.SystemPrompt + `\n\n平台执行协议（不可省略）：可选模板为 cyber-blue（科技蓝）、executive-dark（管理驾驶舱）、operations-green（绿色运维）、aurora-purple（AI 创新）。严禁只口头声称已经修改；只有输出下列机器指令后平台才会真正执行。正常回复用户后，必须在回复末尾输出一个如下格式的代码块：` + "\n\n```tianyan-skill\n" + `{"action":"update_showcase","template":"cyber-blue","changes":{"title":"标题","subtitle":"副标题","announcement":"提示","metrics":[{"label":"指标","value":"数值","trend":"趋势"}],"highlights":["亮点"]}}` + "\n```\n" + `action 固定为 update_showcase；template 必须为可选模板之一；changes 可只包含用户要求修改的字段。不要输出其他动作、路径、脚本或命令。`
	raw["messages"] = append([]interface{}{map[string]interface{}{"role": "system", "content": prompt}}, messages...)
	delete(raw, "skill_id")
	data, err := common.Marshal(raw)
	if err != nil {
		return err
	}
	return common.ReplaceRequestBody(c, data)
}
