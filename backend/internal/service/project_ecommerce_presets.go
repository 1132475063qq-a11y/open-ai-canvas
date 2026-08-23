package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const EcommercePresetSchemaVersion = 2

const (
	EcommerceKernelModelInteraction = "MODEL_INTERACTION"
	EcommerceKernelStillLife        = "STILL_LIFE"
)

type EcommercePresetShotRole struct {
	Key         string                  `json:"key"`
	Title       string                  `json:"title"`
	Framing     string                  `json:"framing"`
	Direction   string                  `json:"direction"`
	Interaction string                  `json:"interaction"`
	DurationMS  int64                   `json:"durationMs"`
	Camera      EcommerceShotCameraSpec `json:"camera"`
}

type EcommercePresetConstraints struct {
	ProductFidelity []string `json:"productFidelity"`
	IdentitySafety  []string `json:"identitySafety"`
	Commercial      []string `json:"commercial"`
	Cost            []string `json:"cost"`
}

type EcommercePresetDefinition struct {
	SchemaVersion       int                        `json:"schemaVersion"`
	SkillRef            string                     `json:"skillRef"`
	Kernel              string                     `json:"kernel"`
	SupportedCategories []string                   `json:"supportedCategories"`
	SceneTemplate       string                     `json:"sceneTemplate"`
	InteractionTemplate string                     `json:"interactionTemplate"`
	ShotRoles           []EcommercePresetShotRole  `json:"shotRoles"`
	RequiredConstraints EcommercePresetConstraints `json:"requiredConstraints"`
	NegativePrompt      string                     `json:"negativePrompt"`
}

type EcommercePreset struct {
	ID          string                    `json:"id"`
	PresetKey   string                    `json:"presetKey"`
	Version     int                       `json:"version"`
	Name        string                    `json:"name"`
	Kernel      string                    `json:"kernel"`
	Category    string                    `json:"category"`
	Description string                    `json:"description"`
	System      bool                      `json:"system"`
	SourceID    string                    `json:"sourceId,omitempty"`
	Definition  EcommercePresetDefinition `json:"definition"`
	UpdatedAt   time.Time                 `json:"updatedAt"`
}

type EcommercePresetCatalog struct {
	SchemaVersion int               `json:"schemaVersion"`
	System        []EcommercePreset `json:"system"`
	Custom        []EcommercePreset `json:"custom"`
}

type SaveEcommercePresetRequest struct {
	PresetKey   string                     `json:"presetKey"`
	SourceID    string                     `json:"sourceId"`
	Name        string                     `json:"name"`
	Description string                     `json:"description"`
	Definition  *EcommercePresetDefinition `json:"definition"`
}

func (s *Service) ProjectEcommercePresets(userID string, projectID string) (EcommercePresetCatalog, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return EcommercePresetCatalog{}, err
	}
	rows, err := s.repo.ProjectEcommercePresetVersions(projectID)
	if err != nil {
		return EcommercePresetCatalog{}, err
	}
	custom := make([]EcommercePreset, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		if _, exists := seen[row.PresetKey]; exists {
			continue
		}
		preset, decodeErr := ecommercePresetFromRow(row)
		if decodeErr != nil {
			return EcommercePresetCatalog{}, decodeErr
		}
		seen[row.PresetKey] = struct{}{}
		custom = append(custom, preset)
	}
	return EcommercePresetCatalog{SchemaVersion: EcommercePresetSchemaVersion, System: systemEcommercePresets(), Custom: custom}, nil
}

func (s *Service) SaveProjectEcommercePreset(userID string, projectID string, req SaveEcommercePresetRequest) (EcommercePreset, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return EcommercePreset{}, err
	}
	catalog, err := s.ProjectEcommercePresets(userID, projectID)
	if err != nil {
		return EcommercePreset{}, err
	}
	sourceID := strings.TrimSpace(req.SourceID)
	presetKey := strings.TrimSpace(req.PresetKey)
	var source *EcommercePreset
	for _, candidate := range append(append([]EcommercePreset{}, catalog.System...), catalog.Custom...) {
		if candidate.ID == sourceID || candidate.PresetKey == sourceID || (presetKey != "" && candidate.PresetKey == presetKey) {
			copy := candidate
			source = &copy
			break
		}
	}
	if source == nil {
		return EcommercePreset{}, BadAuthRequest("请选择一个可用的系统或用户预设作为版本来源")
	}
	if presetKey == "" || source.System {
		presetKey = "custom:" + newID()
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = source.Name + " 副本"
	}
	definition := source.Definition
	if req.Definition != nil {
		definition = *req.Definition
	}
	definition.SchemaVersion = EcommercePresetSchemaVersion
	definition.SkillRef = "ecommerce.skill." + strings.TrimPrefix(presetKey, "custom:")
	definition.Kernel = source.Kernel
	definition.RequiredConstraints = immutableEcommerceConstraints()
	definition.SupportedCategories = uniqueNonEmpty(definition.SupportedCategories)
	if len(definition.SupportedCategories) == 0 {
		definition.SupportedCategories = append([]string(nil), source.Definition.SupportedCategories...)
	}
	definition = normalizeEcommercePresetDefinition(definition)
	if err := validateEcommercePresetDefinition(definition); err != nil {
		return EcommercePreset{}, err
	}
	encoded, err := json.Marshal(definition)
	if err != nil {
		return EcommercePreset{}, BadAuthRequest("电商预设定义格式无效")
	}
	now := time.Now()
	row := model.EcommercePresetVersion{
		ID: newID(), UserID: userID, ProjectID: projectID, PresetKey: presetKey,
		SourcePresetID: source.ID, Name: truncateRunes(name, 160), Kernel: source.Kernel,
		Category: source.Category, Description: truncateRunes(firstNonEmpty(strings.TrimSpace(req.Description), source.Description), 500),
		DefinitionJSON: string(encoded), CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.SaveEcommercePresetVersion(&row); err != nil {
		return EcommercePreset{}, err
	}
	return ecommercePresetFromRow(row)
}

func (s *Service) resolveEcommercePreset(userID string, projectID string, presetID string) (EcommercePreset, error) {
	presetID = strings.TrimSpace(presetID)
	for _, preset := range systemEcommercePresets() {
		if preset.ID == presetID || preset.PresetKey == presetID {
			return preset, nil
		}
	}
	row, err := s.repo.LatestEcommercePresetVersion(projectID, presetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return EcommercePreset{}, BadAuthRequest("电商预设不存在或已不可用")
		}
		return EcommercePreset{}, err
	}
	if row.UserID != userID {
		return EcommercePreset{}, BadAuthRequest("电商预设不属于当前项目")
	}
	return ecommercePresetFromRow(*row)
}

func ecommercePresetFromRow(row model.EcommercePresetVersion) (EcommercePreset, error) {
	definition := EcommercePresetDefinition{}
	if err := json.Unmarshal([]byte(row.DefinitionJSON), &definition); err != nil {
		return EcommercePreset{}, BadAuthRequest("用户电商预设定义损坏")
	}
	definition = normalizeEcommercePresetDefinition(definition)
	if err := validateEcommercePresetDefinition(definition); err != nil {
		return EcommercePreset{}, err
	}
	return EcommercePreset{
		ID: row.PresetKey, PresetKey: row.PresetKey, Version: row.Version, Name: row.Name,
		Kernel: row.Kernel, Category: row.Category, Description: row.Description,
		System: false, SourceID: row.SourcePresetID, Definition: definition, UpdatedAt: row.UpdatedAt,
	}, nil
}

func validateEcommercePresetDefinition(definition EcommercePresetDefinition) error {
	definition = normalizeEcommercePresetDefinition(definition)
	if definition.Kernel != EcommerceKernelModelInteraction && definition.Kernel != EcommerceKernelStillLife {
		return BadAuthRequest("电商预设内核必须是 MODEL_INTERACTION 或 STILL_LIFE")
	}
	if strings.TrimSpace(definition.SceneTemplate) == "" {
		return BadAuthRequest("电商预设必须包含场景模板")
	}
	if len(definition.ShotRoles) == 0 || len(definition.ShotRoles) > 12 {
		return BadAuthRequest("电商预设镜头结构必须包含 1-12 个镜头角色")
	}
	seen := map[string]struct{}{}
	for _, role := range definition.ShotRoles {
		key := strings.TrimSpace(role.Key)
		if key == "" || strings.TrimSpace(role.Title) == "" || strings.TrimSpace(role.Direction) == "" {
			return BadAuthRequest("电商预设镜头角色缺少 key、标题或导演指令")
		}
		if _, exists := seen[key]; exists {
			return BadAuthRequest("电商预设镜头角色 key 不能重复")
		}
		camera := role.Camera
		if strings.TrimSpace(camera.Azimuth) == "" || strings.TrimSpace(camera.Elevation) == "" ||
			strings.TrimSpace(camera.Lens) == "" || strings.TrimSpace(camera.Distance) == "" ||
			strings.TrimSpace(camera.SubjectRegion) == "" || strings.TrimSpace(camera.SubjectFill) == "" ||
			strings.TrimSpace(camera.Pose) == "" || strings.TrimSpace(camera.Composition) == "" {
			return BadAuthRequest("电商预设镜头角色缺少完整机位合同")
		}
		seen[key] = struct{}{}
	}
	return nil
}

func immutableEcommerceConstraints() EcommercePresetConstraints {
	return EcommercePresetConstraints{
		ProductFidelity: []string{
			"商品结构、轮廓、材质、颜色、包装与 Logo 必须与授权参考一致",
			"不得虚构参考中不存在的接口、配件、容量、功效、成分或认证",
			"商品不得被手指、衣物、道具或画面裁切遮挡关键识别信息",
		},
		IdentitySafety: []string{
			"使用授权真人时必须保持身份与年龄特征一致，AI 模特必须沿用身份卡",
			"人体结构、穿戴关系、手部接触和商品受力必须自然可信",
			"不得生成危险、歧视、色情或侵犯第三方权利的内容",
		},
		Commercial: []string{
			"系列图片必须保持商品、人物、场景、光线和色彩连续",
			"渠道比例、安全区与必须保留信息不得被镜头风格覆盖",
		},
		Cost: []string{
			"任何真实生成或付费重试都必须展示有效报价并由用户确认",
			"Agent 与 Skill 不得读取密钥、直连 Provider 或执行静默付费重试",
		},
	}
}

func systemEcommercePresets() []EcommercePreset {
	modelRoles := modelInteractionShotRoles()
	stillRoles := stillLifeShotRoles()
	items := []struct {
		id, name, kernel, category, description, scene, interaction string
		categories                                                  []string
		roles                                                       []EcommercePresetShotRole
	}{
		{"model.top-wear", "上装模特入景", EcommerceKernelModelInteraction, "apparel", "锁定上装版型与面料，让同一模特在生活场景中完成六镜头系列。", "自然光生活方式空间，可结合用户 Scene Pack 调整", "准确穿着上装，领口、肩线、袖长、门襟和下摆关系真实", []string{"apparel"}, modelRoles},
		{"model.bottom-wear", "下装穿搭", EcommerceKernelModelInteraction, "apparel", "强调腰线、裤型或裙摆，并保持步态与穿着关系。", "有纵深的城市或室内生活场景", "完整呈现腰头、侧缝、裤脚或裙摆动态", []string{"apparel"}, modelRoles},
		{"model.footwear", "鞋履上脚", EcommerceKernelModelInteraction, "shoes_bags", "从全身穿搭到鞋面、鞋底和落地受力的系列镜头。", "适合行走与停留的真实地面环境", "鞋履正确上脚，脚踝、鞋带、鞋底与地面接触可信", []string{"shoes_bags"}, modelRoles},
		{"model.bag-accessory", "包袋配饰", EcommerceKernelModelInteraction, "shoes_bags", "覆盖手提、肩背、容量感与五金细节。", "通勤、旅行或休闲生活方式空间", "肩带长度、提手受力、包体比例与五金结构准确", []string{"shoes_bags"}, modelRoles},
		{"model.jewelry-wear", "珠宝佩戴", EcommerceKernelModelInteraction, "jewelry", "控制佩戴位置、尺度、金属和宝石高光。", "干净克制的高端人像环境", "珠宝尺寸、扣合、贴肤关系与左右方向准确", []string{"jewelry"}, modelRoles},
		{"model.beauty-use", "美妆使用", EcommerceKernelModelInteraction, "beauty", "展示拿取、开启、涂抹与妆效氛围，避免虚假功效。", "明亮梳妆台、浴室或日常随身场景", "手部与容器接触自然，包装文字和色号保持一致", []string{"beauty"}, modelRoles},
		{"model.product-handheld", "手持商品", EcommerceKernelModelInteraction, "general", "适用于数码、食品、美妆与小件商品的人物互动套图。", "与商品用途匹配的真实生活环境", "握持尺度、手指遮挡和使用姿态符合商品结构", []string{"electronics", "food", "beauty", "home", "general"}, modelRoles},
		{"model.multi-model", "多人合拍", EcommerceKernelModelInteraction, "apparel", "为双人或多人 Campaign 规划主次关系和互动镜头。", "可容纳多人走位的生活方式场景", "人物身份稳定，商品归属清楚，互动不遮挡关键卖点", []string{"apparel", "shoes_bags", "general"}, modelRoles},
		{"model.lifestyle-campaign", "生活方式 Campaign", EcommerceKernelModelInteraction, "general", "从主题自动组织人物、环境、互动和细节的完整 Campaign。", "围绕品牌主题构建具有连续空间关系的 Scene Pack", "同一人物、商品和造型贯穿整组，自然动作而非僵硬摆拍", []string{"apparel", "shoes_bags", "jewelry", "beauty", "electronics", "home", "food", "general"}, modelRoles},
		{"still.clean-commercial", "纯净商拍", EcommerceKernelStillLife, "general", "用干净背景、准确材质和清晰结构完成标准商品套图。", "无干扰棚拍背景或品牌色背景", "商品独立陈列，结构、尺度和投影真实", []string{"apparel", "shoes_bags", "jewelry", "beauty", "electronics", "home", "food", "general"}, stillRoles},
		{"still.lifestyle-tabletop", "生活方式桌面", EcommerceKernelStillLife, "general", "在真实桌面场景中用道具和光线表达使用情境。", "可触达的生活方式桌面，前中后景关系清晰", "商品与道具尺度正确，摆放符合真实使用逻辑", []string{"beauty", "electronics", "home", "food", "jewelry", "general"}, stillRoles},
		{"still.hero-pedestal", "主视觉展台", EcommerceKernelStillLife, "general", "以展台、轮廓光和品牌色建立强主视觉。", "简洁展台与可控商业灯光，不使用无意义装饰球体", "商品是唯一主角，底部接触、投影和反射可信", []string{"beauty", "electronics", "jewelry", "shoes_bags", "general"}, stillRoles},
		{"still.ingredient-story", "成分叙事", EcommerceKernelStillLife, "beauty", "用已确认成分、原料和质地组织卖点镜头。", "与真实成分或原料相关的克制叙事场景", "只使用 ProductDNA 已记录成分，不暗示未证实功效", []string{"beauty", "food"}, stillRoles},
		{"still.in-context-use", "使用场景", EcommerceKernelStillLife, "general", "在家居、厨房、办公或户外环境中说明真实用途。", "与商品用途和用户 Scene Pack 一致的真实空间", "摆放、连接、开合与周边物体关系符合实际使用", []string{"electronics", "home", "food", "beauty", "general"}, stillRoles},
		{"still.detail-macro", "微距细节", EcommerceKernelStillLife, "general", "突出纹理、工艺、接口、标签与局部结构。", "受控微距棚拍环境，景深不遮蔽关键结构", "微距只放大真实存在的细节，不重绘 Logo 或文字", []string{"apparel", "shoes_bags", "jewelry", "beauty", "electronics", "home", "food", "general"}, stillRoles},
		{"still.social-campaign", "社媒套图", EcommerceKernelStillLife, "general", "为小红书和抖音封面组织统一但有节奏变化的套图。", "兼顾竖屏安全区和真实生活感的连续场景", "每张承担不同信息角色，整体色彩和商品状态连续", []string{"apparel", "shoes_bags", "jewelry", "beauty", "electronics", "home", "food", "general"}, stillRoles},
	}
	now := time.Date(2026, time.August, 21, 0, 0, 0, 0, time.UTC)
	result := make([]EcommercePreset, 0, len(items))
	for _, item := range items {
		definition := EcommercePresetDefinition{
			SchemaVersion: EcommercePresetSchemaVersion, SkillRef: ecommerceSkillRefForPreset(item.id),
			Kernel: item.kernel, SupportedCategories: item.categories, SceneTemplate: item.scene,
			InteractionTemplate: item.interaction, ShotRoles: cloneEcommerceShotRoles(item.roles),
			RequiredConstraints: immutableEcommerceConstraints(),
			NegativePrompt:      "商品结构改变，错误 Logo，错误文字，错误颜色，多余部件，人体畸形，手指错误，穿模，悬浮，塑料质感，过度磨皮，杂乱背景，低清晰度",
		}
		definition = normalizeEcommercePresetDefinition(definition)
		result = append(result, EcommercePreset{ID: item.id, PresetKey: item.id, Version: 1, Name: item.name, Kernel: item.kernel, Category: item.category, Description: item.description, System: true, Definition: definition, UpdatedAt: now})
	}
	return result
}

func ecommerceSkillRefForPreset(presetID string) string {
	switch presetID {
	case "model.top-wear":
		return "model-interaction.top-wear@1"
	case "still.lifestyle-tabletop":
		return "still-life.lifestyle-tabletop@1"
	default:
		return "ecommerce.skill." + presetID
	}
}

func modelInteractionShotRoles() []EcommercePresetShotRole {
	return []EcommercePresetShotRole{
		{Key: "hero", Title: "主视觉", Framing: "全身或三分之二身", Direction: "建立商品、模特与场景的第一视觉层级", Interaction: "自然站立或轻微行进，商品完整可辨", DurationMS: 2500},
		{Key: "environment", Title: "环境全景", Framing: "广角全景", Direction: "交代真实空间、光线方向和生活状态", Interaction: "人物融入环境，仍保持商品可辨识", DurationMS: 2200},
		{Key: "medium", Title: "关系中景", Framing: "中景", Direction: "展示人物姿态、商品比例和主要卖点", Interaction: "动作放松，接触和穿戴关系自然", DurationMS: 2400},
		{Key: "action", Title: "动作/使用", Framing: "动态中近景", Direction: "捕捉一个符合用途的自然动作瞬间", Interaction: "商品受力、开合、穿戴或使用方式准确", DurationMS: 2600},
		{Key: "detail", Title: "商品特写", Framing: "近景或微距", Direction: "突出材质、结构、Logo 或关键工艺", Interaction: "避免手指遮挡，不改变真实细节", DurationMS: 2200},
		{Key: "supplement", Title: "补充镜头", Framing: "侧面或背面变化", Direction: "补足前五张未覆盖的角度与使用信息", Interaction: "保持同一人物、造型、商品和场景连续", DurationMS: 2300},
	}
}

func stillLifeShotRoles() []EcommercePresetShotRole {
	return []EcommercePresetShotRole{
		{Key: "hero", Title: "主视觉", Framing: "标准主图景别", Direction: "建立商品轮廓、品牌层级和主要卖点", Interaction: "商品稳定陈列并完整可辨", DurationMS: 2500},
		{Key: "environment", Title: "环境全景", Framing: "场景全景", Direction: "交代商品使用空间和道具关系", Interaction: "道具服务于用途，不喧宾夺主", DurationMS: 2200},
		{Key: "medium", Title: "陈列中景", Framing: "中景", Direction: "展示商品尺度、包装和周边关系", Interaction: "摆放、投影和反射符合物理逻辑", DurationMS: 2400},
		{Key: "action", Title: "使用状态", Framing: "使用过程近景", Direction: "呈现开启、倾倒、连接或取用后的可信状态", Interaction: "只展示商品真实支持的使用方式", DurationMS: 2600},
		{Key: "detail", Title: "商品特写", Framing: "微距", Direction: "突出材质、接口、纹理、标签或成分质地", Interaction: "保留原始结构与文字，不凭空增加细节", DurationMS: 2200},
		{Key: "supplement", Title: "补充镜头", Framing: "俯拍、侧拍或背面", Direction: "补足套图的信息密度和构图节奏", Interaction: "保持商品状态、场景光线和色彩连续", DurationMS: 2300},
	}
}

func cloneEcommerceShotRoles(values []EcommercePresetShotRole) []EcommercePresetShotRole {
	result := make([]EcommercePresetShotRole, len(values))
	copy(result, values)
	for index := range result {
		result[index].Camera.AvoidReuseOf = append([]string(nil), values[index].Camera.AvoidReuseOf...)
	}
	return result
}

func (s *Service) requireEcommerceProject(userID string, projectID string) (*model.Project, error) {
	project, err := s.repo.ProjectForUser(userID, strings.TrimSpace(projectID))
	if err != nil {
		return nil, err
	}
	if project.Type != model.ProjectTypeEcommerce {
		return nil, BadAuthRequest("电商生产接口只能用于 ecommerce 项目")
	}
	if project.Status == model.ProjectStatusArchived {
		return nil, BadAuthRequest("项目已归档，不能创建或修改电商生产任务")
	}
	return project, nil
}
