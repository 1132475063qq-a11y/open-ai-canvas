package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

const (
	EcommerceRunStatusPlanning       = "planning"
	EcommerceRunStatusAwaitingReview = "awaiting_review"
	EcommerceRunStatusAwaitingCost   = "awaiting_cost"
	EcommerceRunStatusGenerating     = "generating"
	EcommerceRunStatusQA             = "qa"
	EcommerceRunStatusNeedsYou       = "needs_you"
	EcommerceRunStatusReady          = "ready"
	EcommerceRunStatusFailed         = "failed"
	EcommerceRunStatusCancelled      = "cancelled"

	EcommerceSlotStatusPlanned   = "planned"
	EcommerceSlotStatusScheduled = "scheduled"
	EcommerceSlotStatusQueued    = "queued"
	EcommerceSlotStatusRunning   = "running"
	EcommerceSlotStatusQA        = "qa"
	EcommerceSlotStatusAccepted  = "accepted"
	EcommerceSlotStatusFailed    = "failed"
	EcommerceSlotStatusCancelled = "cancelled"

	EcommerceQAStatusPending   = "PENDING"
	EcommerceQAStatusPass      = "PASS"
	EcommerceQAStatusUncertain = "UNCERTAIN"
	EcommerceQAStatusFail      = "FAIL"
)

const ecommerceQuoteTTL = 15 * time.Minute

type EcommerceProviderRoute struct {
	ChannelID             string   `json:"channelId"`
	ChannelName           string   `json:"channelName"`
	ChannelModelID        string   `json:"channelModelId"`
	Model                 string   `json:"model"`
	ModelDisplayName      string   `json:"modelDisplayName"`
	Protocol              string   `json:"protocol"`
	BillingMode           string   `json:"billingMode"`
	UnitPriceMicrocredits int64    `json:"unitPriceMicrocredits,omitempty"`
	PriceVersion          int64    `json:"priceVersion"`
	CapabilityVersion     int64    `json:"capabilityVersion"`
	MaxReferenceImages    int      `json:"maxReferenceImages"`
	SupportsImageEdit     bool     `json:"supportsImageEdit"`
	ProviderReady         bool     `json:"providerReady"`
	BillingReady          bool     `json:"billingReady"`
	RouteReady            bool     `json:"routeReady"`
	Blockers              []string `json:"blockers"`
}

type EcommerceQuote struct {
	Fingerprint       string    `json:"fingerprint"`
	ExpiresAt         time.Time `json:"expiresAt"`
	ChannelID         string    `json:"channelId"`
	ChannelModelID    string    `json:"channelModelId"`
	Model             string    `json:"model"`
	Resolution        string    `json:"resolution"`
	PixelSize         string    `json:"pixelSize"`
	PriceVersion      int64     `json:"priceVersion"`
	UnitMicrocredits  int64     `json:"unitMicrocredits"`
	Count             int       `json:"count"`
	TotalMicrocredits int64     `json:"totalMicrocredits"`
}

type CreateEcommerceRunRequest struct {
	IdempotencyKey         string         `json:"idempotencyKey"`
	ProductAssetIDs        []string       `json:"productAssetIds"`
	SupportingAssetIDs     []string       `json:"supportingAssetIds"`
	ModelAssetIDs          []string       `json:"modelAssetIds"`
	SceneAssetIDs          []string       `json:"sceneAssetIds"`
	BrandAssetIDs          []string       `json:"brandAssetIds"`
	PresetID               string         `json:"presetId"`
	Category               string         `json:"category"`
	TargetChannel          string         `json:"targetChannel"`
	AspectRatio            string         `json:"aspectRatio"`
	Resolution             string         `json:"resolution"`
	OutputCount            int            `json:"outputCount"`
	ReviewBeforeGeneration bool           `json:"reviewBeforeGeneration"`
	UserGoal               string         `json:"userGoal"`
	ModelMode              string         `json:"modelMode"`
	ModelBrief             string         `json:"modelBrief"`
	SceneBrief             string         `json:"sceneBrief"`
	BrandBrief             string         `json:"brandBrief"`
	ProductFacts           map[string]any `json:"productFacts"`
	Advanced               map[string]any `json:"advanced"`
	ChannelID              string         `json:"channelId"`
	Model                  string         `json:"model"`
}

type RefreshEcommerceQuoteRequest struct {
	ChannelID string `json:"channelId"`
	Model     string `json:"model"`
}

type SubmitEcommerceRunRequest struct {
	QuoteFingerprint string `json:"quoteFingerprint"`
}

type RetryEcommerceSlotRequest struct {
	AttemptID        string `json:"attemptId"`
	QuoteFingerprint string `json:"quoteFingerprint"`
	Kind             string `json:"kind"`
	PromptPatch      string `json:"promptPatch"`
	ChannelID        string `json:"channelId"`
	Model            string `json:"model"`
}

type ReviewEcommerceSlotRequest struct {
	ReviewID   string                           `json:"reviewId"`
	AttemptID  string                           `json:"attemptId"`
	ResultID   string                           `json:"resultId"`
	Decision   string                           `json:"decision"`
	Action     string                           `json:"action"`
	IssueCodes []string                         `json:"issueCodes"`
	Note       string                           `json:"note"`
	Dimensions []EcommerceQADimensionAssessment `json:"dimensions"`
}

type CreateEcommerceVideoSequenceRequest struct {
	SlotIDs         []string `json:"slotIds"`
	Title           string   `json:"title"`
	AspectRatio     string   `json:"aspectRatio"`
	DurationSeconds int      `json:"durationSeconds"`
	MusicResourceID string   `json:"musicResourceId"`
}

func (s *Service) CancelProjectEcommerceRun(ctx context.Context, userID string, projectID string, runID string) (*EcommerceRunView, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return nil, err
	}
	run, err := s.repo.EcommerceProductionRunForUser(userID, projectID, runID)
	if err != nil {
		return nil, err
	}
	if run.Status == EcommerceRunStatusCancelled {
		return s.ProjectEcommerceRun(userID, projectID, runID)
	}
	if run.Status == EcommerceRunStatusReady {
		return nil, Conflict("已完成并接受的系列不能取消")
	}
	taskIDs, err := s.repo.ActiveEcommerceRunTaskIDs(userID, projectID, runID)
	if err != nil {
		return nil, err
	}
	for _, taskID := range taskIDs {
		if _, cancelErr := s.CancelTask(ctx, userID, taskID); cancelErr != nil {
			latest, latestErr := s.repo.TaskForUser(userID, taskID)
			if latestErr != nil || (latest.Status != model.TaskStatusSucceeded && latest.Status != model.TaskStatusFailed && latest.Status != model.TaskStatusCancelled) {
				return nil, cancelErr
			}
		}
	}
	if err := s.repo.FinalizeEcommerceRunCancellation(userID, projectID, runID, time.Now()); err != nil {
		return nil, mapEcommerceRuntimeError(err)
	}
	return s.ProjectEcommerceRun(userID, projectID, runID)
}

type EcommerceAttemptView struct {
	Attempt model.EcommerceProductionAttempt `json:"attempt"`
	Task    *TaskSummary                     `json:"task,omitempty"`
}

type EcommerceSlotView struct {
	Slot     model.EcommerceProductionSlot `json:"slot"`
	Attempts []EcommerceAttemptView        `json:"attempts"`
	Reviews  []EcommerceQAReviewView       `json:"reviews"`
}

type EcommerceRunView struct {
	Run    model.EcommerceProductionRun `json:"run"`
	Quote  *EcommerceQuote              `json:"quote,omitempty"`
	Slots  []EcommerceSlotView          `json:"slots"`
	Routes []EcommerceProviderRoute     `json:"routes"`
}

type EcommerceWorkspace struct {
	SchemaVersion  int                            `json:"schemaVersion"`
	Artifacts      []model.EcommerceArtifact      `json:"artifacts"`
	Presets        EcommercePresetCatalog         `json:"presets"`
	ProviderRoutes []EcommerceProviderRoute       `json:"providerRoutes"`
	Runs           []model.EcommerceProductionRun `json:"runs"`
	ActiveRun      *EcommerceRunView              `json:"activeRun,omitempty"`
}

type EcommerceRetryResponse struct {
	Attempt model.EcommerceProductionAttempt `json:"attempt"`
	Quote   *EcommerceQuote                  `json:"quote,omitempty"`
	Run     *EcommerceRunView                `json:"run,omitempty"`
}

type ecommerceProviderRouteResolution struct {
	config       providerConfig
	channel      *model.ModelChannel
	channelModel *model.ChannelModel
	profile      *ImageCapabilityConfig
}

func (s *Service) ProjectEcommerceWorkspace(userID string, projectID string) (EcommerceWorkspace, error) {
	project, err := s.ecommerceProjectForRead(userID, projectID)
	if err != nil {
		return EcommerceWorkspace{}, err
	}
	artifacts, err := s.repo.ProjectEcommerceArtifacts(project.ID)
	if err != nil {
		return EcommerceWorkspace{}, err
	}
	presets, err := s.projectEcommercePresetsForRead(userID, project.ID)
	if err != nil {
		return EcommerceWorkspace{}, err
	}
	routes, err := s.EcommerceProviderRoutes()
	if err != nil {
		return EcommerceWorkspace{}, err
	}
	runs, err := s.repo.ProjectEcommerceProductionRuns(userID, project.ID, 30)
	if err != nil {
		return EcommerceWorkspace{}, err
	}
	workspace := EcommerceWorkspace{SchemaVersion: 1, Artifacts: artifacts, Presets: presets, ProviderRoutes: routes, Runs: runs}
	if len(runs) > 0 {
		view, viewErr := s.ecommerceRunView(userID, project.ID, runs[0].ID, routes)
		if viewErr != nil {
			return EcommerceWorkspace{}, viewErr
		}
		workspace.ActiveRun = view
	}
	return workspace, nil
}

func (s *Service) ProjectEcommerceRun(userID string, projectID string, runID string) (*EcommerceRunView, error) {
	if _, err := s.ecommerceProjectForRead(userID, projectID); err != nil {
		return nil, err
	}
	routes, err := s.EcommerceProviderRoutes()
	if err != nil {
		return nil, err
	}
	return s.ecommerceRunView(userID, projectID, runID, routes)
}

func (s *Service) CreateProjectEcommerceRun(userID string, projectID string, req CreateEcommerceRunRequest) (*EcommerceRunView, error) {
	project, err := s.requireEcommerceProject(userID, projectID)
	if err != nil {
		return nil, err
	}
	idempotencyKey := strings.TrimSpace(req.IdempotencyKey)
	if idempotencyKey == "" {
		idempotencyKey = newID()
	}
	if len(idempotencyKey) > 160 {
		return nil, BadAuthRequest("电商 Run 幂等键过长")
	}
	if existing, existingErr := s.repo.EcommerceProductionRunByIdempotency(userID, project.ID, idempotencyKey); existingErr == nil {
		return s.ProjectEcommerceRun(userID, project.ID, existing.ID)
	} else if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return nil, existingErr
	}
	preset, err := s.resolveEcommercePreset(userID, project.ID, req.PresetID)
	if err != nil {
		return nil, err
	}
	outputCount := req.OutputCount
	if outputCount == 0 {
		outputCount = 6
	}
	if outputCount < 1 || outputCount > 12 {
		return nil, BadAuthRequest("系列套图数量必须在 1-12 张之间")
	}
	targetChannel := normalizeEcommerceTargetChannel(req.TargetChannel)
	if targetChannel == "" {
		return nil, BadAuthRequest("首批渠道仅支持淘宝/京东、小红书和抖音")
	}
	aspectRatio := normalizeEcommerceAspectRatio(firstNonEmpty(strings.TrimSpace(req.AspectRatio), project.AspectRatio))
	if aspectRatio == "" {
		return nil, BadAuthRequest("电商输出比例仅支持 1:1、3:4、4:3、9:16 和 16:9")
	}
	resolution := normalizeEcommerceResolution(req.Resolution)
	if resolution == "" {
		return nil, BadAuthRequest("电商图片分辨率仅支持 1K、2K 和 4K")
	}
	pixelSize := ecommercePixelSize(aspectRatio, resolution)
	if pixelSize == "" {
		return nil, BadAuthRequest("无法为所选比例计算电商图片尺寸")
	}
	category := normalizeEcommerceCategory(req.Category)
	productIDs := uniqueNonEmpty(req.ProductAssetIDs)
	if len(productIDs) == 0 {
		return nil, BadAuthRequest("至少选择一个主商品资产")
	}
	productAssets, err := s.ecommerceAssetFacts(userID, project.ID, productIDs)
	if err != nil {
		return nil, err
	}
	if category == "general" && len(productAssets) > 0 {
		category = categoryFromAsset(productAssets[0])
	}
	supportingAssets, err := s.ecommerceAssetFacts(userID, project.ID, uniqueNonEmpty(req.SupportingAssetIDs))
	if err != nil {
		return nil, err
	}
	modelAssets, err := s.ecommerceAssetFacts(userID, project.ID, uniqueNonEmpty(req.ModelAssetIDs))
	if err != nil {
		return nil, err
	}
	sceneAssets, err := s.ecommerceAssetFacts(userID, project.ID, uniqueNonEmpty(req.SceneAssetIDs))
	if err != nil {
		return nil, err
	}
	brandAssets, err := s.ecommerceAssetFacts(userID, project.ID, uniqueNonEmpty(req.BrandAssetIDs))
	if err != nil {
		return nil, err
	}
	modelMode := "none"
	if preset.Kernel == EcommerceKernelModelInteraction {
		modelMode = "ai"
		if len(modelAssets) > 0 || req.ModelMode == "uploaded" {
			modelMode = "uploaded"
		}
	}
	now := time.Now()
	run := model.EcommerceProductionRun{
		ID: newID(), UserID: userID, ProjectID: project.ID, IdempotencyKey: idempotencyKey,
		Status: EcommerceRunStatusPlanning, Kernel: preset.Kernel, Category: category,
		PresetID: preset.ID, PresetVersion: preset.Version, TargetChannel: targetChannel,
		AspectRatio: aspectRatio, Resolution: resolution, PixelSize: pixelSize,
		OutputCount: outputCount, ReviewBeforeGeneration: req.ReviewBeforeGeneration,
		ProductAssetIDsJSON: jsonStringArray(productIDs), SupportingAssetIDsJSON: jsonStringArray(req.SupportingAssetIDs),
		ModelAssetIDsJSON: jsonStringArray(req.ModelAssetIDs), SceneAssetIDsJSON: jsonStringArray(req.SceneAssetIDs), BrandAssetIDsJSON: jsonStringArray(req.BrandAssetIDs),
		UserGoal: strings.TrimSpace(req.UserGoal), ModelMode: modelMode, ModelBrief: strings.TrimSpace(req.ModelBrief),
		SceneBrief: strings.TrimSpace(req.SceneBrief), BrandBrief: strings.TrimSpace(req.BrandBrief), AdvancedJSON: jsonObject(req.Advanced),
		CreatedAt: now, UpdatedAt: now,
	}
	planning := ecommerceRunPlanningInput{
		Run: run, Preset: preset, ProductAssets: productAssets, SupportingAssets: supportingAssets,
		ModelAssets: modelAssets, SceneAssets: sceneAssets, BrandAssets: brandAssets,
		ProductFacts: req.ProductFacts, Advanced: req.Advanced, Now: now,
	}
	plan, err := buildEcommerceRunPlan(planning)
	if err != nil {
		return nil, err
	}
	routes, err := s.EcommerceProviderRoutes()
	if err != nil {
		return nil, err
	}
	route := selectEcommerceRoute(routes, req.ChannelID, req.Model)
	var quote *EcommerceQuote
	if route != nil && route.RouteReady {
		quote, err = s.newEcommerceQuote(userID, run, *route, now, "run")
		if err != nil {
			return nil, err
		}
		applyQuoteToRun(&run, *quote)
		if run.ReviewBeforeGeneration {
			run.Status = EcommerceRunStatusAwaitingReview
		} else {
			run.Status = EcommerceRunStatusAwaitingCost
		}
	} else {
		run.Status = EcommerceRunStatusNeedsYou
		run.Error = "当前没有同时满足多参考图、图片编辑、服务端配置和计费要求的图片模型"
	}
	attempts := make([]model.EcommerceProductionAttempt, 0, len(plan.Slots))
	for index := range plan.Slots {
		attempt := model.EcommerceProductionAttempt{
			ID: newID(), UserID: userID, ProjectID: project.ID, RunID: run.ID, SlotID: plan.Slots[index].ID,
			AttemptNumber: 1, Kind: "initial", Status: EcommerceSlotStatusPlanned,
			Prompt: plan.Slots[index].Prompt, NegativePrompt: plan.Slots[index].NegativePrompt,
			CreatedAt: now, UpdatedAt: now,
		}
		if quote != nil {
			applyQuoteToAttempt(&attempt, *quote)
			attempt.Status = EcommerceRunStatusAwaitingCost
		}
		plan.Slots[index].ActiveAttemptID = attempt.ID
		attempts = append(attempts, attempt)
	}
	if err := s.repo.CreateEcommerceProductionRunAtomic(repository.EcommerceRunAtomicCreateInput{Run: &run, Slots: plan.Slots, Attempts: attempts, Artifacts: plan.Artifacts}); err != nil {
		if errors.Is(err, repository.ErrEcommerceRunStateConflict) {
			if existing, existingErr := s.repo.EcommerceProductionRunByIdempotency(userID, project.ID, idempotencyKey); existingErr == nil {
				return s.ProjectEcommerceRun(userID, project.ID, existing.ID)
			}
			return nil, Conflict("电商 Run 已由另一个请求创建，请刷新工作台")
		}
		return nil, err
	}
	return s.ecommerceRunView(userID, project.ID, run.ID, routes)
}

func (s *Service) RefreshProjectEcommerceRunQuote(userID string, projectID string, runID string, req RefreshEcommerceQuoteRequest) (*EcommerceRunView, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return nil, err
	}
	run, err := s.repo.EcommerceProductionRunForUser(userID, projectID, runID)
	if err != nil {
		return nil, err
	}
	if err := s.validateEcommerceRunEvidence(userID, projectID, *run, false); err != nil {
		return nil, err
	}
	if run.SubmittedAt != nil {
		return nil, Conflict("该系列已经提交，不能重新报价")
	}
	routes, err := s.EcommerceProviderRoutes()
	if err != nil {
		return nil, err
	}
	route := selectEcommerceRoute(routes, req.ChannelID, req.Model)
	if route == nil || !route.RouteReady {
		return nil, BadAuthRequest("请选择一个已完成服务端配置和计费配置的多参考图片模型")
	}
	quote, err := s.newEcommerceQuote(userID, *run, *route, time.Now(), "run")
	if err != nil {
		return nil, err
	}
	applyQuoteToRun(run, *quote)
	run.UpdatedAt = time.Now()
	attempts, err := s.repo.EcommerceProductionAttempts(run.ID)
	if err != nil {
		return nil, err
	}
	for index := range attempts {
		if attempts[index].TaskID == "" {
			applyQuoteToAttempt(&attempts[index], *quote)
		}
	}
	if err := s.repo.ApplyEcommerceRunQuote(run, attempts); err != nil {
		return nil, mapEcommerceRuntimeError(err)
	}
	return s.ecommerceRunView(userID, projectID, runID, routes)
}

func (s *Service) ApproveProjectEcommerceRun(userID string, projectID string, runID string) (*EcommerceRunView, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return nil, err
	}
	if err := s.repo.ApproveEcommerceRunPlan(userID, projectID, runID, time.Now()); err != nil {
		return nil, mapEcommerceRuntimeError(err)
	}
	return s.ProjectEcommerceRun(userID, projectID, runID)
}

func (s *Service) SubmitProjectEcommerceRun(userID string, projectID string, runID string, req SubmitEcommerceRunRequest) (*EcommerceRunView, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return nil, err
	}
	run, err := s.repo.EcommerceProductionRunForUser(userID, projectID, runID)
	if err != nil {
		return nil, err
	}
	if err := s.validateEcommerceRunEvidence(userID, projectID, *run, false); err != nil {
		return nil, err
	}
	if run.Status == EcommerceRunStatusGenerating || run.Status == EcommerceRunStatusQA || run.Status == EcommerceRunStatusNeedsYou || run.Status == EcommerceRunStatusReady {
		if run.SubmittedAt != nil && run.QuoteFingerprint == strings.TrimSpace(req.QuoteFingerprint) {
			return s.ProjectEcommerceRun(userID, projectID, runID)
		}
	}
	if run.Status == EcommerceRunStatusAwaitingReview {
		return nil, Conflict("请先确认 Agent 方案，再进入费用确认")
	}
	if run.Status != EcommerceRunStatusAwaitingCost {
		return nil, Conflict("当前 Run 不在待费用确认状态")
	}
	if strings.TrimSpace(req.QuoteFingerprint) == "" || req.QuoteFingerprint != run.QuoteFingerprint {
		return nil, Conflict("报价已变化，请刷新后重新确认")
	}
	resolution, err := s.resolveEcommerceProviderRoute(run.QuoteChannelID, run.QuoteModel)
	if err != nil {
		return nil, BadAuthRequest("报价对应的图片模型已不可用，请重新报价")
	}
	slots, err := s.repo.EcommerceProductionSlots(run.ID)
	if err != nil {
		return nil, err
	}
	attempts, err := s.repo.EcommerceProductionAttempts(run.ID)
	if err != nil {
		return nil, err
	}
	attemptByID := make(map[string]model.EcommerceProductionAttempt, len(attempts))
	for _, attempt := range attempts {
		attemptByID[attempt.ID] = attempt
	}
	references, err := s.ecommerceRunProviderReferences(userID, *run, resolution.profile.References.MaxImages)
	if err != nil {
		return nil, err
	}
	config, err := ecommerceImageProviderConfig(*run, resolution)
	if err != nil {
		return nil, err
	}
	creditsEnabled, err := s.FeatureEnabled(FeatureCredits)
	if err != nil {
		return nil, err
	}
	bindings := make([]repository.EcommerceTaskBinding, 0, len(slots))
	now := time.Now()
	for _, slot := range slots {
		attempt, exists := attemptByID[slot.ActiveAttemptID]
		if !exists || attempt.Status != EcommerceRunStatusAwaitingCost || attempt.TaskID != "" {
			return nil, Conflict("电商槽位 Attempt 链不完整，请刷新后重试")
		}
		task, input, buildErr := ecommerceImageTask(*run, slot, attempt, config, references, resolution, now)
		if buildErr != nil {
			return nil, buildErr
		}
		if err := s.ValidateTaskCapability(input); err != nil {
			return nil, err
		}
		var order *model.BillingOrder
		if creditsEnabled {
			order, err = s.newBillingOrder(userID, task.ID, "ecommerce_attempt:"+attempt.ID, resolution.channel.ID, resolution.channelModel.ModelKey, "image", "ecommerce_image", 1, tokenBillingEstimate{})
			if err != nil {
				return nil, err
			}
			if order.AmountMicrocredits != attempt.QuoteAmountMicrocredits {
				return nil, Conflict("模型价格已变化，请重新报价")
			}
		}
		bindings = append(bindings, repository.EcommerceTaskBinding{SlotID: slot.ID, AttemptID: attempt.ID, Task: task, Order: order})
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		return nil, err
	}
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	usage, err := s.repo.UserStorageUsage(userID)
	if err != nil {
		return nil, err
	}
	if usage.TaskCount+int64(len(bindings)) > policy.Resource.TaskCount {
		return nil, BadAuthRequest(fmt.Sprintf("账号任务历史最多 %d 条，本次系列没有足够任务名额", policy.Resource.TaskCount))
	}
	incoming := int64(0)
	for _, binding := range bindings {
		incoming += int64(len(binding.Task.Prompt) + len(binding.Task.InputJSON))
	}
	if err := validateTaskDataGrowthQuotaWithPolicy(usage, incoming, policy.Resource); err != nil {
		return nil, err
	}
	err = s.repo.SubmitEcommerceProductionRunAtomic(repository.EcommerceRunAtomicSubmitInput{
		UserID: userID, RunID: run.ID, QuoteFingerprint: req.QuoteFingerprint,
		ChannelModel: *resolution.channelModel, Bindings: bindings, ActiveTaskLimit: policy.Task.ActiveTaskLimit,
	})
	if err != nil {
		return nil, mapEcommerceRuntimeError(err)
	}
	for _, binding := range bindings {
		s.recordActivity(userID, "task", 1)
		_ = s.log(userID, binding.Task.ID, "info", "电商系列槽位已进入后端编排", "")
	}
	return s.ProjectEcommerceRun(userID, projectID, runID)
}

func (s *Service) RetryProjectEcommerceSlot(userID string, projectID string, runID string, slotID string, req RetryEcommerceSlotRequest) (*EcommerceRetryResponse, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return nil, err
	}
	run, err := s.repo.EcommerceProductionRunForUser(userID, projectID, runID)
	if err != nil {
		return nil, err
	}
	if err := s.validateEcommerceRunEvidence(userID, projectID, *run, false); err != nil {
		return nil, err
	}
	slot, err := s.repo.EcommerceProductionSlotForUser(userID, runID, slotID)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(req.AttemptID) == "" || strings.TrimSpace(req.QuoteFingerprint) == "" {
		return s.createEcommerceRetryQuote(userID, *run, *slot, req)
	}
	attempt, err := s.repo.EcommerceProductionAttemptForUser(userID, runID, slotID, req.AttemptID)
	if err != nil {
		return nil, err
	}
	if attempt.QuoteFingerprint != req.QuoteFingerprint {
		return nil, Conflict("重试报价已变化，请重新确认")
	}
	resolution, err := s.resolveEcommerceProviderRoute(attempt.QuoteChannelID, attempt.QuoteModel)
	if err != nil {
		return nil, BadAuthRequest("重试报价对应的图片模型已不可用")
	}
	references, err := s.ecommerceRunProviderReferences(userID, *run, resolution.profile.References.MaxImages)
	if err != nil {
		return nil, err
	}
	// Only a local repair may inherit the current frame. A retry or variation
	// must get a fresh composition from the slot camera contract.
	if attempt.Kind == "repair" {
		current, ok := ecommerceGeneratedImageReference(slot.ResultPayloadJSON)
		if ok && len(references) < resolution.profile.References.MaxImages {
			encoded, _ := json.Marshal(current)
			var media providerMedia
			if json.Unmarshal(encoded, &media) == nil {
				references = append(references, media)
			}
		}
	}
	config, err := ecommerceImageProviderConfig(*run, resolution)
	if err != nil {
		return nil, err
	}
	task, input, err := ecommerceImageTask(*run, *slot, *attempt, config, references, resolution, time.Now())
	if err != nil {
		return nil, err
	}
	if err := s.ValidateTaskCapability(input); err != nil {
		return nil, err
	}
	creditsEnabled, err := s.FeatureEnabled(FeatureCredits)
	if err != nil {
		return nil, err
	}
	var order *model.BillingOrder
	if creditsEnabled {
		order, err = s.newBillingOrder(userID, task.ID, "ecommerce_attempt:"+attempt.ID, resolution.channel.ID, resolution.channelModel.ModelKey, "image", "ecommerce_"+attempt.Kind, 1, tokenBillingEstimate{})
		if err != nil {
			return nil, err
		}
		if order.AmountMicrocredits != attempt.QuoteAmountMicrocredits {
			return nil, Conflict("模型价格已变化，请重新获取重试报价")
		}
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		return nil, err
	}
	if err := s.repo.SubmitEcommerceRetryAtomic(repository.EcommerceRetryAtomicSubmitInput{
		UserID: userID, RunID: runID, SlotID: slotID, AttemptID: attempt.ID, QuoteFingerprint: req.QuoteFingerprint,
		ChannelModel: *resolution.channelModel, Task: task, Order: order, ActiveTaskLimit: policy.Task.ActiveTaskLimit,
	}); err != nil {
		return nil, mapEcommerceRuntimeError(err)
	}
	s.recordActivity(userID, "task", 1)
	_ = s.log(userID, task.ID, "info", "电商槽位付费重试已进入后端编排", "")
	view, err := s.ProjectEcommerceRun(userID, projectID, runID)
	if err != nil {
		return nil, err
	}
	return &EcommerceRetryResponse{Attempt: *attempt, Run: view}, nil
}

func (s *Service) ReviewProjectEcommerceSlot(userID string, projectID string, runID string, slotID string, req ReviewEcommerceSlotRequest) (*EcommerceRunView, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return nil, err
	}
	run, err := s.repo.EcommerceProductionRunForUser(userID, projectID, runID)
	if err != nil {
		return nil, err
	}
	normalized, err := normalizeEcommerceQAReview(*run, req)
	if err != nil {
		return nil, err
	}
	if existing, existingErr := s.repo.EcommerceArtifactForProjectByID(projectID, normalized.ReviewID); existingErr == nil {
		review, decodeErr := ecommerceQAReviewFromArtifact(*existing)
		if decodeErr != nil || review.RunID != runID || review.SlotID != slotID || !ecommerceQAReplayMatches(review, normalized) {
			return nil, Conflict("QA reviewId 已用于另一条审核记录")
		}
		return s.ProjectEcommerceRun(userID, projectID, runID)
	} else if !errors.Is(existingErr, gorm.ErrRecordNotFound) {
		return nil, existingErr
	}

	attempt, err := s.repo.EcommerceProductionAttemptForUser(userID, runID, slotID, normalized.AttemptID)
	if err != nil {
		return nil, err
	}
	if attempt.Status != "succeeded" {
		return nil, BadAuthRequest("只能审核已经生成成功的 Attempt")
	}
	if strings.TrimSpace(attempt.ResultID) == "" || strings.TrimSpace(attempt.GeneratedAssetArtifactID) == "" {
		return nil, BadAuthRequest("Attempt 缺少后端记录的 Result 或 generated_asset，不能审核")
	}
	if attempt.ResultID != normalized.ResultID {
		return nil, Conflict("QA 绑定的 Result 已变化，请刷新工作台后重新审核")
	}
	var task *model.Task
	if attempt.TaskID != "" {
		task, err = s.repo.TaskForUser(userID, attempt.TaskID)
		if err != nil {
			return nil, err
		}
	}
	if normalized.Action == "accept" && strings.TrimSpace(run.PixelSize) != "" {
		actualSize, verified := ecommerceResultPixelSize(attempt.ResultPayloadJSON)
		if !verified {
			return nil, BadAuthRequest("结果像素尺寸尚未核验，不能作为 4K 成片接受")
		}
		if !ecommercePixelSizeMeetsTarget(actualSize, run.PixelSize) {
			return nil, BadAuthRequest(fmt.Sprintf("结果实际为 %s，未达到本次至少 %s 且保持同画幅的输出要求", actualSize, run.PixelSize))
		}
	}
	now := time.Now()
	payload := ecommerceQAArtifactPayload{
		SchemaVersion: ecommerceQAReportSchemaVersion, ReviewID: normalized.ReviewID,
		RunID: runID, SlotID: slotID, AttemptID: attempt.ID, ResultID: attempt.ResultID,
		Decision: normalized.Decision, Action: normalized.Action, IssueCodes: normalized.IssueCodes,
		Note: normalized.Note, Dimensions: normalized.Dimensions,
		RuntimeEvidence: ecommerceQARuntimeEvidence(*attempt, task),
		ReviewerUserID:  userID, Source: "human", ReviewedAt: now,
	}
	artifact, err := plannedEcommerceArtifact(*run, EcommerceArtifactTypeQAReport, "slot:"+slotID+":qa", payload, []string{attempt.GeneratedAssetArtifactID}, EcommerceAgentOrchestrator, "")
	if err != nil {
		return nil, err
	}
	artifact.ID = normalized.ReviewID
	artifact.SchemaVersion = ecommerceQAReportSchemaVersion
	artifact.Evidence = "recorded"
	artifact.CreatedAt, artifact.UpdatedAt = now, now
	issuesJSON := jsonStringArray(normalized.IssueCodes)
	if err := s.repo.RecordEcommerceSlotQA(repository.EcommerceSlotQAInput{
		UserID: userID, ProjectID: projectID, RunID: runID, SlotID: slotID, AttemptID: attempt.ID,
		Decision: normalized.Decision, Action: normalized.Action, IssuesJSON: issuesJSON, Note: normalized.Note, Artifact: &artifact, Now: now,
	}); err != nil {
		return nil, mapEcommerceRuntimeError(err)
	}
	return s.ProjectEcommerceRun(userID, projectID, runID)
}

func (s *Service) CreateProjectEcommerceVideoSequence(userID string, projectID string, runID string, req CreateEcommerceVideoSequenceRequest) (*EcommerceRunView, error) {
	if _, err := s.requireEcommerceProject(userID, projectID); err != nil {
		return nil, err
	}
	run, err := s.repo.EcommerceProductionRunForUser(userID, projectID, runID)
	if err != nil {
		return nil, err
	}
	slots, err := s.repo.EcommerceProductionSlots(runID)
	if err != nil {
		return nil, err
	}
	requested := uniqueNonEmpty(req.SlotIDs)
	requestedSet := make(map[string]struct{}, len(requested))
	for _, id := range requested {
		requestedSet[id] = struct{}{}
	}
	accepted := make([]model.EcommerceProductionSlot, 0, len(slots))
	for _, slot := range slots {
		if len(requestedSet) > 0 {
			if _, exists := requestedSet[slot.ID]; !exists {
				continue
			}
		}
		if !slot.Accepted || slot.AcceptedAttemptID == "" || slot.ResultURL == "" {
			return nil, BadAuthRequest("只有已接受的图片才能进入视频阶段")
		}
		accepted = append(accepted, slot)
	}
	if len(accepted) == 0 {
		return nil, BadAuthRequest("至少选择一张已接受图片创建视频编排")
	}
	duration := req.DurationSeconds
	if duration == 0 {
		duration = 15
	}
	if duration < len(accepted) || duration > 60 {
		return nil, BadAuthRequest("视频总时长必须足够覆盖每个镜头，且不能超过 60 秒")
	}
	aspect := normalizeEcommerceAspectRatio(firstNonEmpty(req.AspectRatio, "9:16"))
	if aspect == "" {
		return nil, BadAuthRequest("视频比例无效")
	}
	segments := make([]map[string]any, 0, len(accepted))
	baseMS := duration * 1000 / len(accepted)
	for index, slot := range accepted {
		segmentMS := baseMS
		if index == len(accepted)-1 {
			segmentMS = duration*1000 - baseMS*(len(accepted)-1)
		}
		segments = append(segments, map[string]any{
			"position": index + 1, "slotId": slot.ID, "attemptId": slot.AcceptedAttemptID,
			"sourceUrl": slot.ResultURL, "durationMs": segmentMS,
			"motion": ecommerceMotionForRole(slot.Role), "transition": map[bool]string{true: "cut", false: "match_cut"}[index == 0],
		})
	}
	now := time.Now()
	motionPayload := map[string]any{
		"schemaVersion": 1, "agentId": EcommerceAgentMotionDirector, "runId": runID,
		"aspectRatio": aspect, "durationMs": duration * 1000, "segments": segments,
		"rules": []string{"Preserve product geometry and model identity from the accepted still.", "Use restrained camera and subject motion suitable for ecommerce.", "Do not introduce new text, logos, objects or product states."},
	}
	sequencePayload := map[string]any{
		"schemaVersion": 1, "runId": runID, "title": firstNonEmpty(strings.TrimSpace(req.Title), "15 秒电商短片"),
		"aspectRatio": aspect, "durationMs": duration * 1000, "musicResourceId": strings.TrimSpace(req.MusicResourceID),
		"segments": segments, "status": "planned", "exportEngine": "existing_ffmpeg_timeline",
	}
	refs := make([]string, 0, len(accepted))
	for _, slot := range accepted {
		refs = append(refs, slot.GeneratedAssetID)
	}
	motionArtifact, err := plannedEcommerceArtifact(*run, EcommerceArtifactTypeMotionPlan, "motion-plan", motionPayload, refs, EcommerceAgentMotionDirector, "ecommerce.skill.motion-director")
	if err != nil {
		return nil, err
	}
	sequenceArtifact, err := plannedEcommerceArtifact(*run, EcommerceArtifactTypeVideoSequence, "video-sequence", sequencePayload, refs, EcommerceAgentMotionDirector, "ecommerce.skill.video-sequence")
	if err != nil {
		return nil, err
	}
	motionArtifact.CreatedAt, motionArtifact.UpdatedAt = now, now
	sequenceArtifact.CreatedAt, sequenceArtifact.UpdatedAt = now, now
	run.UpdatedAt = now
	if err := s.repo.SaveEcommerceVideoPlan(run, []model.EcommerceArtifact{motionArtifact, sequenceArtifact}); err != nil {
		return nil, err
	}
	return s.ProjectEcommerceRun(userID, projectID, runID)
}

func (s *Service) EcommerceProviderRoutes() ([]EcommerceProviderRoute, error) {
	channels, err := s.repo.SystemChannels(false)
	if err != nil {
		return nil, err
	}
	routes := []EcommerceProviderRoute{}
	for channelIndex := range channels {
		channel := &channels[channelIndex]
		models, modelErr := s.repo.ChannelModels(channel.ID, false)
		if modelErr != nil {
			return nil, modelErr
		}
		for modelIndex := range models {
			item := &models[modelIndex]
			if item.Capability != "image" || !stringInSlice(item.ModelKey, channelModelNames(*channel)) {
				continue
			}
			profile := DefaultImageCapabilityConfig(string(item.Protocol), item.ModelKey)
			if capability, decodeErr := DecodeModelCapabilityConfig(item.CapabilityConfigJSON); decodeErr == nil && capability != nil && capability.Image != nil {
				profile = capability.Image
			}
			route := EcommerceProviderRoute{
				ChannelID: channel.ID, ChannelName: channel.Name, ChannelModelID: item.ID,
				Model: item.ModelKey, ModelDisplayName: firstNonEmpty(item.DisplayName, item.ModelKey), Protocol: string(item.Protocol),
				BillingMode: item.BillingMode, PriceVersion: item.PriceVersion, CapabilityVersion: item.CapabilityVersion,
				MaxReferenceImages: profile.References.MaxImages, SupportsImageEdit: profile.References.MaxImages > 0,
				BillingReady: item.PriceConfigured, Blockers: []string{},
			}
			if item.PriceConfigured {
				route.UnitPriceMicrocredits = item.UnitPriceMicrocredits
			} else {
				route.Blockers = append(route.Blockers, "模型尚未配置用户积分价格")
			}
			if profile.References.MaxImages < 3 {
				route.Blockers = append(route.Blockers, "电商系列模型至少需要支持 3 张参考图")
			}
			if strings.TrimSpace(channel.BaseURL) == "" || strings.TrimSpace(channel.APIKey) == "" || (requiresFilmProviderSecret(item.Protocol) && strings.TrimSpace(channel.SecretKey) == "") {
				route.Blockers = append(route.Blockers, "服务端渠道配置尚不完整")
			}
			if err := validateGenerationInterface("image", string(item.Protocol)); err != nil {
				route.Blockers = append(route.Blockers, "当前协议不能执行图片生成")
			}
			route.ProviderReady = strings.TrimSpace(channel.BaseURL) != "" && strings.TrimSpace(channel.APIKey) != "" && profile.References.MaxImages >= 3
			route.RouteReady = route.ProviderReady && route.BillingReady && len(route.Blockers) == 0
			routes = append(routes, route)
		}
	}
	sort.SliceStable(routes, func(i, j int) bool {
		if routes[i].RouteReady != routes[j].RouteReady {
			return routes[i].RouteReady
		}
		return routes[i].ModelDisplayName < routes[j].ModelDisplayName
	})
	return routes, nil
}

func requiresFilmProviderSecret(protocol model.ChannelInterfaceType) bool {
	return protocol == model.ChannelInterfaceVolcengineJiMengImage || protocol == model.ChannelInterfaceVolcengineJiMengVideo
}

func filmGenerationImageSize(profile ImageSizeConfig, aspectRatio string) (string, error) {
	if profile.Parameter == "none" {
		return "", nil
	}
	aspectRatio = strings.TrimSpace(aspectRatio)
	if containsCapabilityString(profile.Values, aspectRatio) {
		return aspectRatio, nil
	}
	for _, candidate := range profile.Values {
		if videoRatioAllowed([]string{aspectRatio}, candidate) {
			return candidate, nil
		}
	}
	if profile.AllowCustom {
		return aspectRatio, nil
	}
	return "", BadAuthRequest("电商 Run 画幅不在所选图片模型支持范围内")
}

func (s *Service) createEcommerceRetryQuote(userID string, run model.EcommerceProductionRun, slot model.EcommerceProductionSlot, req RetryEcommerceSlotRequest) (*EcommerceRetryResponse, error) {
	routes, err := s.EcommerceProviderRoutes()
	if err != nil {
		return nil, err
	}
	channelID := firstNonEmpty(strings.TrimSpace(req.ChannelID), run.QuoteChannelID)
	modelKey := firstNonEmpty(strings.TrimSpace(req.Model), run.QuoteModel)
	route := selectEcommerceRoute(routes, channelID, modelKey)
	if route == nil || !route.RouteReady {
		return nil, BadAuthRequest("当前没有可用于付费重试的图片模型")
	}
	now := time.Now()
	quote, err := s.newEcommerceQuote(userID, run, *route, now, "retry:"+slot.ID+":"+newID())
	if err != nil {
		return nil, err
	}
	quote.Count = 1
	quote.TotalMicrocredits = quote.UnitMicrocredits
	kind := strings.ToLower(strings.TrimSpace(req.Kind))
	if kind != "retry" && kind != "repair" && kind != "variation" {
		kind = "retry"
	}
	prompt := slot.Prompt
	if patch := strings.TrimSpace(req.PromptPatch); patch != "" {
		prompt += "\nUser-directed revision: " + patch
	}
	attempt := model.EcommerceProductionAttempt{
		ID: newID(), UserID: userID, ProjectID: run.ProjectID, RunID: run.ID, SlotID: slot.ID,
		Kind: kind, Status: EcommerceRunStatusAwaitingCost, Prompt: prompt, NegativePrompt: slot.NegativePrompt,
		CreatedAt: now, UpdatedAt: now,
	}
	quote.Fingerprint = ecommerceQuoteFingerprint(run, *route, quote.ExpiresAt, 1, "attempt:"+attempt.ID)
	applyQuoteToAttempt(&attempt, *quote)
	if err := s.repo.CreateEcommerceRetryQuoteAttempt(&attempt); err != nil {
		return nil, mapEcommerceRuntimeError(err)
	}
	return &EcommerceRetryResponse{Attempt: attempt, Quote: quote}, nil
}

func (s *Service) ecommerceRunView(userID string, projectID string, runID string, routes []EcommerceProviderRoute) (*EcommerceRunView, error) {
	run, err := s.repo.EcommerceProductionRunForUser(userID, projectID, runID)
	if err != nil {
		return nil, err
	}
	slots, err := s.repo.EcommerceProductionSlots(runID)
	if err != nil {
		return nil, err
	}
	attempts, err := s.repo.EcommerceProductionAttempts(runID)
	if err != nil {
		return nil, err
	}
	qaArtifacts, err := s.repo.EcommerceRunQAArtifacts(projectID, runID)
	if err != nil {
		return nil, err
	}
	reviewsBySlot := make(map[string][]EcommerceQAReviewView, len(slots))
	for _, artifact := range qaArtifacts {
		review, reviewErr := ecommerceQAReviewFromArtifact(artifact)
		if reviewErr != nil {
			return nil, reviewErr
		}
		if review.RunID != runID {
			return nil, fmt.Errorf("ecommerce QA artifact %s belongs to another run", artifact.ID)
		}
		reviewsBySlot[review.SlotID] = append(reviewsBySlot[review.SlotID], review)
	}
	attemptsBySlot := make(map[string][]EcommerceAttemptView, len(slots))
	for _, attempt := range attempts {
		view := EcommerceAttemptView{Attempt: attempt}
		if attempt.TaskID != "" {
			task, taskErr := s.repo.TaskForUser(userID, attempt.TaskID)
			if taskErr != nil {
				return nil, taskErr
			}
			orders := map[string]model.BillingOrder{}
			if task.BillingOrderID != "" {
				if order, orderErr := s.repo.BillingOrder(task.BillingOrderID); orderErr == nil {
					orders[task.ID] = *order
				}
			}
			summary := taskSummariesForOutputWithBilling([]model.Task{*task}, orders)[0]
			view.Task = &summary
		}
		attemptsBySlot[attempt.SlotID] = append(attemptsBySlot[attempt.SlotID], view)
	}
	slotViews := make([]EcommerceSlotView, 0, len(slots))
	for _, slot := range slots {
		attemptViews := attemptsBySlot[slot.ID]
		if attemptViews == nil {
			attemptViews = []EcommerceAttemptView{}
		}
		reviews := reviewsBySlot[slot.ID]
		if reviews == nil {
			reviews = []EcommerceQAReviewView{}
		}
		slotViews = append(slotViews, EcommerceSlotView{Slot: slot, Attempts: attemptViews, Reviews: reviews})
	}
	return &EcommerceRunView{Run: *run, Quote: ecommerceQuoteFromRun(*run), Slots: slotViews, Routes: routes}, nil
}

func (s *Service) newEcommerceQuote(userID string, run model.EcommerceProductionRun, route EcommerceProviderRoute, now time.Time, scope string) (*EcommerceQuote, error) {
	resolution, err := s.resolveEcommerceProviderRoute(route.ChannelID, route.Model)
	if err != nil {
		return nil, BadAuthRequest("报价对应的图片模型已不可用")
	}
	if _, err := ecommerceImageProviderConfig(run, resolution); err != nil {
		return nil, err
	}
	order, err := s.newBillingOrder(userID, "", "quote:"+run.ID+":"+scope+":"+newID(), route.ChannelID, route.Model, "image", "ecommerce_quote", 1, tokenBillingEstimate{})
	if err != nil {
		return nil, err
	}
	expiresAt := now.Add(ecommerceQuoteTTL)
	quote := &EcommerceQuote{
		ExpiresAt: expiresAt, ChannelID: route.ChannelID, ChannelModelID: route.ChannelModelID,
		Model: route.Model, Resolution: run.Resolution, PixelSize: run.PixelSize,
		PriceVersion: route.PriceVersion, UnitMicrocredits: order.AmountMicrocredits,
		Count: run.OutputCount, TotalMicrocredits: order.AmountMicrocredits * int64(run.OutputCount),
	}
	quote.Fingerprint = ecommerceQuoteFingerprint(run, route, expiresAt, run.OutputCount, scope)
	return quote, nil
}

func ecommerceQuoteFingerprint(run model.EcommerceProductionRun, route EcommerceProviderRoute, expiresAt time.Time, count int, scope string) string {
	value := strings.Join([]string{
		run.UserID, run.ProjectID, run.ID, scope, run.PresetID, strconv.Itoa(run.PresetVersion),
		run.AspectRatio, run.Resolution, run.PixelSize, strconv.Itoa(count), route.ChannelID, route.ChannelModelID, route.Model,
		strconv.FormatInt(route.PriceVersion, 10), strconv.FormatInt(expiresAt.Unix(), 10),
	}, "\x00")
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func applyQuoteToRun(run *model.EcommerceProductionRun, quote EcommerceQuote) {
	run.QuoteFingerprint = quote.Fingerprint
	run.QuoteExpiresAt = &quote.ExpiresAt
	run.QuoteChannelID = quote.ChannelID
	run.QuoteChannelModelID = quote.ChannelModelID
	run.QuoteModel = quote.Model
	run.QuotePriceVersion = quote.PriceVersion
	run.QuoteUnitMicrocredits = quote.UnitMicrocredits
	run.QuoteTotalMicrocredits = quote.TotalMicrocredits
}

func applyQuoteToAttempt(attempt *model.EcommerceProductionAttempt, quote EcommerceQuote) {
	attempt.QuoteFingerprint = quote.Fingerprint
	attempt.QuoteExpiresAt = &quote.ExpiresAt
	attempt.QuoteChannelID = quote.ChannelID
	attempt.QuoteChannelModelID = quote.ChannelModelID
	attempt.QuoteModel = quote.Model
	attempt.QuotePriceVersion = quote.PriceVersion
	attempt.QuoteAmountMicrocredits = quote.UnitMicrocredits
}

func ecommerceQuoteFromRun(run model.EcommerceProductionRun) *EcommerceQuote {
	if run.QuoteFingerprint == "" || run.QuoteExpiresAt == nil {
		return nil
	}
	return &EcommerceQuote{
		Fingerprint: run.QuoteFingerprint, ExpiresAt: *run.QuoteExpiresAt, ChannelID: run.QuoteChannelID,
		ChannelModelID: run.QuoteChannelModelID, Model: run.QuoteModel,
		Resolution: run.Resolution, PixelSize: run.PixelSize, PriceVersion: run.QuotePriceVersion,
		UnitMicrocredits: run.QuoteUnitMicrocredits, Count: run.OutputCount, TotalMicrocredits: run.QuoteTotalMicrocredits,
	}
}

func selectEcommerceRoute(routes []EcommerceProviderRoute, channelID string, modelKey string) *EcommerceProviderRoute {
	channelID, modelKey = strings.TrimSpace(channelID), strings.TrimPrefix(strings.TrimSpace(modelKey), "models/")
	if channelID != "" || modelKey != "" {
		for index := range routes {
			if (channelID == "" || routes[index].ChannelID == channelID) && (modelKey == "" || routes[index].Model == modelKey) {
				return &routes[index]
			}
		}
		return nil
	}
	for index := range routes {
		if routes[index].RouteReady {
			return &routes[index]
		}
	}
	return nil
}

func (s *Service) resolveEcommerceProviderRoute(channelID string, modelKey string) (ecommerceProviderRouteResolution, error) {
	config, err := s.resolveProviderConfig(providerConfig{ChannelID: strings.TrimSpace(channelID), Model: strings.TrimSpace(modelKey)})
	if err != nil {
		return ecommerceProviderRouteResolution{}, err
	}
	channel, err := s.repo.SystemChannel(config.ChannelID)
	if err != nil {
		return ecommerceProviderRouteResolution{}, err
	}
	channelModel, err := s.repo.ChannelModelByKey(channel.ID, strings.TrimPrefix(config.Model, "models/"))
	if err != nil {
		return ecommerceProviderRouteResolution{}, err
	}
	if channelModel.Capability != "image" || !channelModel.PriceConfigured {
		return ecommerceProviderRouteResolution{}, BadAuthRequest("所选模型不是已计价的图片模型")
	}
	profile := DefaultImageCapabilityConfig(string(channelModel.Protocol), channelModel.ModelKey)
	capability, err := DecodeModelCapabilityConfig(channelModel.CapabilityConfigJSON)
	if err != nil {
		return ecommerceProviderRouteResolution{}, err
	}
	if capability != nil && capability.Image != nil {
		profile = capability.Image
	}
	if err := validateImageCapabilityConfig(profile); err != nil {
		return ecommerceProviderRouteResolution{}, err
	}
	if profile.References.MaxImages < 3 {
		return ecommerceProviderRouteResolution{}, BadAuthRequest("电商生产要求图片模型至少支持 3 张参考图")
	}
	if strings.TrimSpace(channel.BaseURL) == "" || strings.TrimSpace(channel.APIKey) == "" || (requiresFilmProviderSecret(channelModel.Protocol) && strings.TrimSpace(channel.SecretKey) == "") {
		return ecommerceProviderRouteResolution{}, BadAuthRequest("所选系统渠道尚未完成服务端配置")
	}
	return ecommerceProviderRouteResolution{config: config, channel: channel, channelModel: channelModel, profile: profile}, nil
}

func ecommerceImageProviderConfig(run model.EcommerceProductionRun, resolution ecommerceProviderRouteResolution) (providerConfig, error) {
	size, quality, err := ecommerceImageProviderOptions(run, resolution.profile)
	if err != nil {
		return providerConfig{}, err
	}
	config := providerConfig{ChannelID: resolution.channel.ID, Model: resolution.channelModel.ModelKey, Size: size, Count: "1"}
	if quality != "" {
		config.Quality = quality
	}
	if resolution.profile.TransparentBackground.Supported {
		config.TransparentBackground = strconv.FormatBool(resolution.profile.TransparentBackground.Default)
	}
	return config, nil
}

func ecommerceImageTask(run model.EcommerceProductionRun, slot model.EcommerceProductionSlot, attempt model.EcommerceProductionAttempt, config providerConfig, references []providerMedia, resolution ecommerceProviderRouteResolution, now time.Time) (*model.Task, map[string]any, error) {
	cameraSnapshot := EcommerceShotCameraSpec{}
	if strings.TrimSpace(slot.CameraJSON) != "" {
		if err := json.Unmarshal([]byte(slot.CameraJSON), &cameraSnapshot); err != nil {
			return nil, nil, BadAuthRequest("电商槽位机位快照损坏，请刷新方案后重试")
		}
	}
	prompt, camera := ensureEcommerceCameraContract(attempt.Prompt, run.Kernel, slot.Role, slot.Title, cameraSnapshot)
	if strings.TrimSpace(attempt.NegativePrompt) != "" {
		prompt += "\nAvoid these failure modes: " + attempt.NegativePrompt
	}
	prompt += "\nDo not reproduce another series slot's camera angle, pose, crop, subject placement, or background-anchor layout."
	identityReferencePolicy := "not_applicable"
	if run.Kernel == EcommerceKernelModelInteraction {
		identityReferencePolicy = "locked_text_brief_no_campaign-frame-seed"
		if len(parseJSONStringArray(run.ModelAssetIDsJSON)) > 0 {
			identityReferencePolicy = "authorized_model_assets_identity_only"
		}
	}
	metadata := map[string]any{
		"domain": "ecommerce", "domainProjectId": run.ProjectID, "ecommerceRunId": run.ID,
		"ecommerceSlotId": slot.ID, "ecommerceAttemptId": attempt.ID, "slotRole": slot.Role,
		"generationRequestArtifactId": run.GenerationRequestArtifactID,
		"outputResolution":            run.Resolution, "pixelSize": run.PixelSize, "aspectRatio": run.AspectRatio,
		"cameraContract": camera, "identityReferencePolicy": identityReferencePolicy,
		"providerRoute": map[string]any{
			"channelId": resolution.channel.ID, "channelModelId": resolution.channelModel.ID,
			"model": resolution.channelModel.ModelKey, "capability": "image", "protocol": string(resolution.channelModel.Protocol),
			"capabilityVersion": resolution.channelModel.CapabilityVersion, "priceVersion": resolution.channelModel.PriceVersion,
		},
	}
	runtime := canvasGenerationInput{Mode: "image", Prompt: prompt, Config: config, ReferenceImages: references, Metadata: metadata}
	encoded, err := json.Marshal(runtime)
	if err != nil {
		return nil, nil, BadAuthRequest("电商图片任务输入无法序列化")
	}
	var normalized map[string]any
	if err := json.Unmarshal(encoded, &normalized); err != nil {
		return nil, nil, err
	}
	if containsInlineMediaDataURL(normalized) {
		return nil, nil, BadAuthRequest("电商生成引用必须先保存为项目资源")
	}
	task := &model.Task{
		ID: newID(), UserID: run.UserID, ProjectID: run.ProjectID, DomainProjectID: run.ProjectID,
		Type: "canvas_image", Status: model.TaskStatusScheduled, Stage: "系列任务已锁价，等待调度", Progress: 0,
		Prompt: prompt, Operation: "ecommerce_image", Provider: model.TaskProviderEcommerce, Model: resolution.channelModel.ModelKey,
		InputJSON: string(encoded), CreatedAt: now, UpdatedAt: now,
	}
	return task, normalized, nil
}

func (s *Service) ecommerceRunProviderReferences(userID string, run model.EcommerceProductionRun, maxImages int) ([]providerMedia, error) {
	groups := [][]string{
		parseJSONStringArray(run.ProductAssetIDsJSON), parseJSONStringArray(run.ModelAssetIDsJSON),
		parseJSONStringArray(run.SceneAssetIDsJSON), parseJSONStringArray(run.BrandAssetIDsJSON), parseJSONStringArray(run.SupportingAssetIDsJSON),
	}
	result := []providerMedia{}
	seen := map[string]struct{}{}
	for groupIndex, ids := range groups {
		facts, err := s.ecommerceAssetFacts(userID, run.ProjectID, ids)
		if err != nil {
			return nil, err
		}
		for _, fact := range facts {
			if _, exists := seen[fact.ID]; exists {
				continue
			}
			if len(result) >= maxImages {
				if groupIndex == 0 {
					return nil, BadAuthRequest(fmt.Sprintf("主商品参考图超过所选模型最多 %d 张的限制", maxImages))
				}
				continue
			}
			media, mediaErr := ecommerceProviderMedia(fact)
			if mediaErr != nil {
				return nil, mediaErr
			}
			seen[fact.ID] = struct{}{}
			result = append(result, media)
		}
	}
	if len(result) == 0 {
		return nil, BadAuthRequest("商品资产没有可供 Provider 使用的已上传图片资源")
	}
	return result, nil
}

func (s *Service) ecommerceAssetFacts(userID string, projectID string, ids []string) ([]ecommerceAssetFact, error) {
	ids = uniqueNonEmpty(ids)
	result := make([]ecommerceAssetFact, 0, len(ids))
	for _, id := range ids {
		asset, err := s.repo.ProjectAssetForProject(projectID, id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, BadAuthRequest("所选电商资产未关联当前项目：" + id)
			}
			return nil, err
		}
		if asset.UserID != userID {
			return nil, BadAuthRequest("所选电商资产不属于当前用户")
		}
		payload := map[string]any{}
		if strings.TrimSpace(asset.PayloadJSON) != "" {
			if err := json.Unmarshal([]byte(asset.PayloadJSON), &payload); err != nil {
				return nil, BadAuthRequest("电商资产数据损坏：" + asset.Title)
			}
		}
		data, _ := payload["data"].(map[string]any)
		metadata, _ := payload["metadata"].(map[string]any)
		result = append(result, ecommerceAssetFact{ID: asset.ID, Title: asset.Title, Kind: asset.Kind, Category: string(asset.Category), Data: data, Metadata: metadata})
	}
	return result, nil
}

func ecommerceProviderMedia(asset ecommerceAssetFact) (providerMedia, error) {
	if asset.Kind != "image" {
		return providerMedia{}, BadAuthRequest("电商生成参考必须是图片资产：" + asset.Title)
	}
	storageKey := strings.TrimSpace(fmt.Sprint(asset.Data["storageKey"]))
	url := strings.TrimSpace(firstNonEmpty(fmt.Sprint(asset.Data["url"]), fmt.Sprint(asset.Data["dataUrl"])))
	if storageKey == "<nil>" {
		storageKey = ""
	}
	if url == "<nil>" {
		url = ""
	}
	if storageKey == "" && !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") && !strings.HasPrefix(url, "/api/resources/") {
		return providerMedia{}, BadAuthRequest("图片资产尚未上传到后端资源存储：" + asset.Title)
	}
	media := providerMedia{
		ID: asset.ID, Name: firstNonEmpty(asset.Title, asset.ID), Type: firstNonEmpty(strings.TrimSpace(fmt.Sprint(asset.Data["mimeType"])), "image/png"),
		URL: url, StorageKey: storageKey, MimeType: strings.TrimSpace(fmt.Sprint(asset.Data["mimeType"])),
		Bytes: anyInt64(asset.Data["bytes"]), Width: int(anyInt64(asset.Data["width"])), Height: int(anyInt64(asset.Data["height"])),
	}
	return media, nil
}

func ecommerceGeneratedImageReference(payloadJSON string) (map[string]any, bool) {
	var payload map[string]any
	if json.Unmarshal([]byte(payloadJSON), &payload) != nil {
		return nil, false
	}
	items, _ := payload["images"].([]any)
	if len(items) == 0 {
		return nil, false
	}
	image, _ := items[0].(map[string]any)
	if image == nil {
		return nil, false
	}
	result := map[string]any{
		"id":      "ecommerce-current-result",
		"name":    "current-result.png",
		"type":    "image/png",
		"dataUrl": "",
	}
	for _, key := range []string{"storageKey", "dataUrl", "url", "bytes", "width", "height", "mimeType"} {
		if value, exists := image[key]; exists {
			result[key] = value
		}
	}
	if mimeType, ok := image["mimeType"].(string); ok && mimeType != "" {
		result["type"] = mimeType
	}
	return result, true
}

func ecommerceResultPixelSize(payloadJSON string) (string, bool) {
	image, ok := ecommerceGeneratedImageReference(payloadJSON)
	if !ok {
		return "", false
	}
	width, height := anyInt64(image["width"]), anyInt64(image["height"])
	if width <= 0 || height <= 0 {
		return "", false
	}
	return fmt.Sprintf("%dx%d", width, height), true
}

func ecommercePixelSizeMeetsTarget(actual string, expected string) bool {
	parse := func(value string) (int, int, bool) {
		parts := strings.Split(strings.ToLower(strings.TrimSpace(strings.ReplaceAll(value, "×", "x"))), "x")
		if len(parts) != 2 {
			return 0, 0, false
		}
		width, widthErr := strconv.Atoi(parts[0])
		height, heightErr := strconv.Atoi(parts[1])
		return width, height, widthErr == nil && heightErr == nil && width > 0 && height > 0
	}
	actualWidth, actualHeight, actualOK := parse(actual)
	expectedWidth, expectedHeight, expectedOK := parse(expected)
	if !actualOK || !expectedOK || actualWidth < expectedWidth || actualHeight < expectedHeight {
		return false
	}
	difference := actualWidth*expectedHeight - expectedWidth*actualHeight
	if difference < 0 {
		difference = -difference
	}
	return difference*100 <= expectedWidth*actualHeight
}

func (s *Service) ecommerceProjectForRead(userID string, projectID string) (*model.Project, error) {
	project, err := s.repo.ProjectForUser(userID, strings.TrimSpace(projectID))
	if err != nil {
		return nil, err
	}
	if project.Type != model.ProjectTypeEcommerce {
		return nil, BadAuthRequest("电商工作台只能读取 ecommerce 项目")
	}
	return project, nil
}

func (s *Service) projectEcommercePresetsForRead(userID string, projectID string) (EcommercePresetCatalog, error) {
	rows, err := s.repo.ProjectEcommercePresetVersions(projectID)
	if err != nil {
		return EcommercePresetCatalog{}, err
	}
	custom := []EcommercePreset{}
	seen := map[string]struct{}{}
	for _, row := range rows {
		if row.UserID != userID {
			continue
		}
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

func normalizeEcommerceTargetChannel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "taobao_jd", "taobao", "jd", "淘宝/京东", "淘宝", "京东", "":
		return "taobao_jd"
	case "xiaohongshu", "小红书":
		return "xiaohongshu"
	case "douyin", "抖音":
		return "douyin"
	default:
		return ""
	}
}

func normalizeEcommerceAspectRatio(value string) string {
	value = strings.TrimSpace(value)
	for _, allowed := range []string{"1:1", "3:4", "4:3", "9:16", "16:9"} {
		if value == allowed {
			return value
		}
	}
	return ""
}

func normalizeEcommerceResolution(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "4k"
	}
	for _, allowed := range []string{"1k", "2k", "4k"} {
		if value == allowed {
			return value
		}
	}
	return ""
}

func ecommercePixelSize(aspectRatio string, resolution string) string {
	resolution = normalizeEcommerceResolution(resolution)
	sizes := map[string]map[string]string{
		"1k": {"1:1": "1024x1024", "3:4": "768x1024", "4:3": "1024x768", "9:16": "576x1024", "16:9": "1024x576"},
		"2k": {"1:1": "2048x2048", "3:4": "1536x2048", "4:3": "2048x1536", "9:16": "1152x2048", "16:9": "2048x1152"},
		"4k": {"1:1": "4096x4096", "3:4": "3072x4096", "4:3": "4096x3072", "9:16": "2160x3840", "16:9": "3840x2160"},
	}
	return sizes[resolution][normalizeEcommerceAspectRatio(aspectRatio)]
}

func ecommerceImageProviderOptions(run model.EcommerceProductionRun, profile *ImageCapabilityConfig) (string, string, error) {
	if profile == nil {
		return "", "", BadAuthRequest("所选图片模型缺少能力配置")
	}
	// Legacy Runs did not persist an explicit resolution. Preserve their old
	// request semantics instead of silently upgrading an already quoted task.
	if strings.TrimSpace(run.Resolution) == "" && strings.TrimSpace(run.PixelSize) == "" {
		size, err := filmGenerationImageSize(profile.Size, run.AspectRatio)
		quality := ""
		if profile.Quality.Supported {
			quality = profile.Quality.Default
		}
		return size, quality, err
	}
	resolution := normalizeEcommerceResolution(run.Resolution)
	expectedSize := ecommercePixelSize(run.AspectRatio, resolution)
	actualSize := strings.ToLower(strings.TrimSpace(strings.ReplaceAll(run.PixelSize, "×", "x")))
	if resolution == "" || expectedSize == "" || actualSize != expectedSize {
		return "", "", BadAuthRequest("电商 Run 的分辨率与像素尺寸不一致，请新建生产方案")
	}
	quality := ""
	resolutionQualityMapped := false
	if profile.Quality.Supported {
		quality = profile.Quality.Default
		preferred := map[string]string{"1k": "low", "2k": "medium", "4k": "high"}[resolution]
		if containsCapabilityString(profile.Quality.Values, resolution) {
			quality = resolution
			resolutionQualityMapped = true
		} else if containsCapabilityString(profile.Quality.Values, preferred) {
			quality = preferred
			resolutionQualityMapped = true
		}
	}
	switch profile.Size.Parameter {
	case "size":
		if !containsCapabilityString(profile.Size.Values, expectedSize) && !profile.Size.AllowCustom {
			return "", "", BadAuthRequest(fmt.Sprintf("所选图片模型不支持 %s %s 输出", strings.ToUpper(resolution), expectedSize))
		}
		return expectedSize, quality, nil
	case "aspect_ratio":
		size, err := filmGenerationImageSize(profile.Size, run.AspectRatio)
		if err != nil {
			return "", "", err
		}
		if !resolutionQualityMapped {
			return "", "", BadAuthRequest(fmt.Sprintf("所选图片模型不能保证 %s 输出", strings.ToUpper(resolution)))
		}
		return size, quality, nil
	default:
		return "", "", BadAuthRequest("所选图片模型不支持精确分辨率控制")
	}
}

func normalizeEcommerceCategory(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	aliases := map[string]string{"clothing": "apparel", "fashion": "apparel", "shoes": "shoes_bags", "bags": "shoes_bags", "cosmetics": "beauty", "digital": "electronics", "furniture": "home"}
	if alias := aliases[value]; alias != "" {
		value = alias
	}
	for _, allowed := range []string{"apparel", "shoes_bags", "jewelry", "beauty", "electronics", "home", "food", "general"} {
		if value == allowed {
			return value
		}
	}
	return "general"
}

func categoryFromAsset(asset ecommerceAssetFact) string {
	text := strings.ToLower(strings.Join([]string{asset.Title, asset.Category, fmt.Sprint(asset.Metadata["category"])}, " "))
	for category, words := range map[string][]string{
		"apparel":    {"衣", "衫", "裙", "裤", "apparel", "shirt", "dress", "wardrobe"},
		"shoes_bags": {"鞋", "包", "shoe", "bag"}, "jewelry": {"珠宝", "首饰", "戒指", "项链", "jewelry"},
		"beauty": {"美妆", "护肤", "口红", "beauty", "cosmetic"}, "electronics": {"数码", "耳机", "手机", "electronic", "digital"},
		"home": {"家居", "家具", "home", "furniture"}, "food": {"食品", "饮料", "food", "drink"},
	} {
		for _, word := range words {
			if strings.Contains(text, word) {
				return category
			}
		}
	}
	return "general"
}

func ecommerceMotionForRole(role string) string {
	switch {
	case strings.HasPrefix(role, "hero"):
		return "slow 4 percent push-in with subtle subject breathing or product parallax"
	case strings.HasPrefix(role, "environment"):
		return "gentle lateral reveal that preserves scene geometry"
	case strings.HasPrefix(role, "action"):
		return "one short natural use action, then settle on the product"
	case strings.HasPrefix(role, "detail"):
		return "restrained macro rack focus across the recorded product detail"
	default:
		return "small stabilized camera drift with no product deformation"
	}
}

func mapEcommerceRuntimeError(err error) error {
	switch {
	case errors.Is(err, repository.ErrEcommerceQuoteExpired):
		return Conflict("报价已过期，请重新获取并确认")
	case errors.Is(err, repository.ErrEcommerceQuoteChanged):
		return Conflict("报价或模型配置已变化，请重新确认")
	case errors.Is(err, repository.ErrEcommerceRunStateConflict):
		return Conflict("电商 Run 状态已被其他操作推进，请刷新工作台")
	case errors.Is(err, repository.ErrEcommerceAttemptConflict):
		return Conflict("电商槽位 Attempt 已被其他操作推进，请刷新工作台")
	case errors.Is(err, repository.ErrEcommerceQAReviewConflict):
		return Conflict("QA reviewId 已用于另一条审核记录")
	case errors.Is(err, repository.ErrInsufficientCredits):
		return BadAuthRequest("积分不足，请先充值后再提交整组任务")
	default:
		return err
	}
}

func jsonStringArray(values []string) string {
	encoded, _ := json.Marshal(uniqueNonEmpty(values))
	return string(encoded)
}

func jsonObject(value map[string]any) string {
	if value == nil {
		return "{}"
	}
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func parseJSONStringArray(raw string) []string {
	values := []string{}
	_ = json.Unmarshal([]byte(raw), &values)
	return uniqueNonEmpty(values)
}

func anyInt64(value any) int64 {
	if value == nil {
		return 0
	}
	switch number := value.(type) {
	case int:
		return int64(number)
	case int8:
		return int64(number)
	case int16:
		return int64(number)
	case int32:
		return int64(number)
	case int64:
		return number
	case uint:
		return int64(number)
	case uint8:
		return int64(number)
	case uint16:
		return int64(number)
	case uint32:
		return int64(number)
	case uint64:
		if number > math.MaxInt64 {
			return 0
		}
		return int64(number)
	case float32:
		if !math.IsNaN(float64(number)) && !math.IsInf(float64(number), 0) {
			return int64(number)
		}
	case float64:
		if !math.IsNaN(number) && !math.IsInf(number, 0) {
			return int64(number)
		}
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if number, err := strconv.ParseInt(strings.TrimSuffix(text, ".0"), 10, 64); err == nil {
		return number
	}
	if number, err := strconv.ParseFloat(text, 64); err == nil && !math.IsNaN(number) && !math.IsInf(number, 0) {
		return int64(number)
	}
	return 0
}
