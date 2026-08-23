package service

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
)

const ecommerceQAReportSchemaVersion = 2

const (
	EcommerceQAVerdictPass          = "PASS"
	EcommerceQAVerdictUncertain     = "UNCERTAIN"
	EcommerceQAVerdictFail          = "FAIL"
	EcommerceQAVerdictNotApplicable = "NOT_APPLICABLE"
)

type EcommerceQADimensionAssessment struct {
	Key     string `json:"key"`
	Score   *int   `json:"score"`
	Verdict string `json:"verdict"`
	Note    string `json:"note,omitempty"`
}

type EcommerceQARuntimeEvidence struct {
	Source                  string `json:"source"`
	AttemptID               string `json:"attemptId"`
	AttemptStatus           string `json:"attemptStatus"`
	TaskID                  string `json:"taskId,omitempty"`
	TaskStatus              string `json:"taskStatus,omitempty"`
	ResultID                string `json:"resultId"`
	ProviderRequestRecorded bool   `json:"providerRequestRecorded"`
	BillingRecorded         bool   `json:"billingRecorded"`
	LatencyMS               *int64 `json:"latencyMs,omitempty"`
	APIStabilityScore       int    `json:"apiStabilityScore"`
	APIStabilityVerdict     string `json:"apiStabilityVerdict"`
}

type EcommerceQAReviewView struct {
	ArtifactID      string                           `json:"artifactId"`
	Revision        int                              `json:"revision"`
	SchemaVersion   int                              `json:"schemaVersion"`
	ReviewID        string                           `json:"reviewId"`
	RunID           string                           `json:"runId"`
	SlotID          string                           `json:"slotId"`
	AttemptID       string                           `json:"attemptId"`
	ResultID        string                           `json:"resultId"`
	Decision        string                           `json:"decision"`
	Action          string                           `json:"action"`
	IssueCodes      []string                         `json:"issueCodes"`
	Note            string                           `json:"note,omitempty"`
	Dimensions      []EcommerceQADimensionAssessment `json:"dimensions"`
	RuntimeEvidence EcommerceQARuntimeEvidence       `json:"runtimeEvidence"`
	ReviewerUserID  string                           `json:"reviewerUserId"`
	Source          string                           `json:"source"`
	ReviewedAt      time.Time                        `json:"reviewedAt"`
}

type ecommerceQAArtifactPayload struct {
	SchemaVersion   int                              `json:"schemaVersion"`
	ReviewID        string                           `json:"reviewId"`
	RunID           string                           `json:"runId"`
	SlotID          string                           `json:"slotId"`
	AttemptID       string                           `json:"attemptId"`
	ResultID        string                           `json:"resultId"`
	Decision        string                           `json:"decision"`
	Action          string                           `json:"action"`
	IssueCodes      []string                         `json:"issueCodes"`
	Note            string                           `json:"note,omitempty"`
	Dimensions      []EcommerceQADimensionAssessment `json:"dimensions"`
	RuntimeEvidence EcommerceQARuntimeEvidence       `json:"runtimeEvidence"`
	ReviewerUserID  string                           `json:"reviewerUserId"`
	Source          string                           `json:"source"`
	ReviewedAt      time.Time                        `json:"reviewedAt"`
}

type ecommerceQADimensionDefinition struct {
	Key   string
	Label string
}

var ecommerceQADimensionDefinitions = []ecommerceQADimensionDefinition{
	{Key: "product_fidelity", Label: "商品结构与保真"},
	{Key: "color_material", Label: "颜色与材质"},
	{Key: "logo_text", Label: "Logo 与文字"},
	{Key: "model_identity", Label: "模特身份"},
	{Key: "anatomy_contact", Label: "人体与接触关系"},
	{Key: "scene_consistency", Label: "场景一致性"},
	{Key: "series_consistency", Label: "系列一致性与差异度"},
	{Key: "commercial_quality", Label: "商业成片质量"},
}

type ecommerceNormalizedQAReview struct {
	ReviewID   string
	AttemptID  string
	ResultID   string
	Decision   string
	Action     string
	IssueCodes []string
	Note       string
	Dimensions []EcommerceQADimensionAssessment
}

func normalizeEcommerceQAReview(run model.EcommerceProductionRun, req ReviewEcommerceSlotRequest) (ecommerceNormalizedQAReview, error) {
	reviewID := strings.TrimSpace(req.ReviewID)
	if !validEcommerceQAReviewID(reviewID) {
		return ecommerceNormalizedQAReview{}, BadAuthRequest("QA reviewId 必须是 8-80 位稳定标识")
	}
	resultID := strings.TrimSpace(req.ResultID)
	if resultID == "" {
		return ecommerceNormalizedQAReview{}, BadAuthRequest("QA 必须绑定当前 Attempt 的 resultId")
	}
	attemptID := strings.TrimSpace(req.AttemptID)
	if attemptID == "" {
		return ecommerceNormalizedQAReview{}, BadAuthRequest("QA 必须绑定一个已成功的 Attempt")
	}
	decision := strings.ToUpper(strings.TrimSpace(req.Decision))
	if decision != EcommerceQAStatusPass && decision != EcommerceQAStatusUncertain && decision != EcommerceQAStatusFail {
		return ecommerceNormalizedQAReview{}, BadAuthRequest("QA 结论必须是 PASS、UNCERTAIN 或 FAIL")
	}
	action := strings.ToLower(strings.TrimSpace(req.Action))
	if action == "" {
		action = "hold"
	}
	switch decision {
	case EcommerceQAStatusPass:
		if action != "accept" && action != "hold" {
			return ecommerceNormalizedQAReview{}, BadAuthRequest("PASS 结果只能接受或暂存")
		}
	case EcommerceQAStatusUncertain:
		if action != "hold" {
			return ecommerceNormalizedQAReview{}, BadAuthRequest("UNCERTAIN 结果必须保留人工处理，不能接受或拒绝")
		}
	case EcommerceQAStatusFail:
		if action != "reject" {
			return ecommerceNormalizedQAReview{}, BadAuthRequest("FAIL 结果必须明确拒绝")
		}
	}

	note := strings.TrimSpace(req.Note)
	if len([]rune(note)) > 2000 {
		return ecommerceNormalizedQAReview{}, BadAuthRequest("QA 审核备注不能超过 2000 字")
	}
	issues := uniqueNonEmpty(req.IssueCodes)
	if len(issues) > 24 {
		return ecommerceNormalizedQAReview{}, BadAuthRequest("QA 问题类型不能超过 24 项")
	}
	for _, issue := range issues {
		if len(issue) > 80 {
			return ecommerceNormalizedQAReview{}, BadAuthRequest("QA 问题标识不能超过 80 字符")
		}
	}
	sort.Strings(issues)

	byKey := make(map[string]EcommerceQADimensionAssessment, len(req.Dimensions))
	for _, raw := range req.Dimensions {
		key := strings.ToLower(strings.TrimSpace(raw.Key))
		if key == "" {
			return ecommerceNormalizedQAReview{}, BadAuthRequest("QA 维度 key 不能为空")
		}
		if _, exists := byKey[key]; exists {
			return ecommerceNormalizedQAReview{}, BadAuthRequest("QA 维度不能重复")
		}
		raw.Key = key
		byKey[key] = raw
	}
	if len(byKey) != len(ecommerceQADimensionDefinitions) {
		return ecommerceNormalizedQAReview{}, BadAuthRequest("QA 必须完整评估全部 8 个商业质量维度")
	}

	dimensions := make([]EcommerceQADimensionAssessment, 0, len(ecommerceQADimensionDefinitions))
	derivedDecision := EcommerceQAStatusPass
	for _, definition := range ecommerceQADimensionDefinitions {
		dimension, exists := byKey[definition.Key]
		if !exists {
			return ecommerceNormalizedQAReview{}, BadAuthRequest("QA 缺少维度：" + definition.Label)
		}
		dimension.Note = strings.TrimSpace(dimension.Note)
		if len([]rune(dimension.Note)) > 500 {
			return ecommerceNormalizedQAReview{}, BadAuthRequest(definition.Label + "备注不能超过 500 字")
		}
		dimension.Verdict = strings.ToUpper(strings.TrimSpace(dimension.Verdict))
		required := ecommerceQADimensionRequired(run, definition.Key)
		if dimension.Verdict == EcommerceQAVerdictNotApplicable {
			if required {
				return ecommerceNormalizedQAReview{}, BadAuthRequest(definition.Label + "是本次生产的必检维度，不能标记为不适用")
			}
			if dimension.Score != nil {
				return ecommerceNormalizedQAReview{}, BadAuthRequest(definition.Label + "标记不适用时不能同时评分")
			}
			dimensions = append(dimensions, dimension)
			continue
		}
		if dimension.Score == nil || *dimension.Score < 1 || *dimension.Score > 5 {
			return ecommerceNormalizedQAReview{}, BadAuthRequest(definition.Label + "必须给出 1-5 分或明确标记不适用")
		}
		expectedVerdict := ecommerceQAVerdictForScore(*dimension.Score)
		if dimension.Verdict != expectedVerdict {
			return ecommerceNormalizedQAReview{}, BadAuthRequest(fmt.Sprintf("%s 的分数与结论不一致", definition.Label))
		}
		if *dimension.Score <= 3 && dimension.Note == "" && note == "" && len(issues) == 0 {
			return ecommerceNormalizedQAReview{}, BadAuthRequest(definition.Label + "低于通过线时必须记录问题或备注")
		}
		if ecommerceQADecisionSeverity(expectedVerdict) > ecommerceQADecisionSeverity(derivedDecision) {
			derivedDecision = expectedVerdict
		}
		dimensions = append(dimensions, dimension)
	}

	if ecommerceQADecisionSeverity(decision) < ecommerceQADecisionSeverity(derivedDecision) {
		return ecommerceNormalizedQAReview{}, BadAuthRequest(fmt.Sprintf("QA 总结论不能比维度评分更宽松，维度评分至少要求 %s", derivedDecision))
	}
	if decision != derivedDecision && note == "" && len(issues) == 0 {
		return ecommerceNormalizedQAReview{}, BadAuthRequest("QA 总结论比维度评分更严格时必须记录原因")
	}
	if action == "accept" && derivedDecision != EcommerceQAStatusPass {
		return ecommerceNormalizedQAReview{}, BadAuthRequest("所有适用维度达到 4 分后才能接受并进入视频阶段")
	}

	return ecommerceNormalizedQAReview{
		ReviewID: reviewID, AttemptID: attemptID, ResultID: resultID, Decision: decision, Action: action,
		IssueCodes: issues, Note: note, Dimensions: dimensions,
	}, nil
}

func ecommerceQADimensionRequired(run model.EcommerceProductionRun, key string) bool {
	switch key {
	case "product_fidelity", "color_material", "scene_consistency", "commercial_quality":
		return true
	case "model_identity", "anatomy_contact":
		return run.Kernel == EcommerceKernelModelInteraction
	case "series_consistency":
		return run.OutputCount > 1
	default:
		return false
	}
}

func ecommerceQAVerdictForScore(score int) string {
	if score <= 2 {
		return EcommerceQAVerdictFail
	}
	if score == 3 {
		return EcommerceQAVerdictUncertain
	}
	return EcommerceQAVerdictPass
}

func ecommerceQADecisionSeverity(value string) int {
	switch value {
	case EcommerceQAVerdictFail:
		return 2
	case EcommerceQAVerdictUncertain:
		return 1
	default:
		return 0
	}
}

func validEcommerceQAReviewID(value string) bool {
	if len(value) < 8 || len(value) > 80 {
		return false
	}
	for _, char := range value {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '-' || char == '_' {
			continue
		}
		return false
	}
	return true
}

func ecommerceQARuntimeEvidence(attempt model.EcommerceProductionAttempt, task *model.Task) EcommerceQARuntimeEvidence {
	evidence := EcommerceQARuntimeEvidence{
		Source: "attempt_task", AttemptID: attempt.ID, AttemptStatus: attempt.Status,
		TaskID: attempt.TaskID, ResultID: attempt.ResultID,
		ProviderRequestRecorded: strings.TrimSpace(attempt.ProviderRequestID) != "",
		BillingRecorded:         strings.TrimSpace(attempt.BillingOrderID) != "",
		APIStabilityScore:       3, APIStabilityVerdict: EcommerceQAVerdictUncertain,
	}
	if task != nil {
		evidence.TaskStatus = string(task.Status)
		evidence.ProviderRequestRecorded = evidence.ProviderRequestRecorded || strings.TrimSpace(task.ProviderRequestID) != ""
		evidence.BillingRecorded = evidence.BillingRecorded || strings.TrimSpace(task.BillingOrderID) != ""
		if task.StartedAt != nil && task.CompletedAt != nil && !task.CompletedAt.Before(*task.StartedAt) {
			latency := task.CompletedAt.Sub(*task.StartedAt).Milliseconds()
			evidence.LatencyMS = &latency
		}
	}
	if attempt.Status != "succeeded" || (task != nil && task.Status != model.TaskStatusSucceeded) {
		evidence.APIStabilityScore = 1
		evidence.APIStabilityVerdict = EcommerceQAVerdictFail
		return evidence
	}
	if evidence.LatencyMS != nil {
		evidence.APIStabilityScore = 4
		evidence.APIStabilityVerdict = EcommerceQAVerdictPass
	}
	if evidence.LatencyMS != nil && evidence.ProviderRequestRecorded {
		evidence.APIStabilityScore = 5
	}
	return evidence
}

func ecommerceQAReviewFromArtifact(artifact model.EcommerceArtifact) (EcommerceQAReviewView, error) {
	var payload ecommerceQAArtifactPayload
	if err := json.Unmarshal([]byte(artifact.PayloadJSON), &payload); err != nil {
		return EcommerceQAReviewView{}, fmt.Errorf("decode ecommerce QA artifact %s: %w", artifact.ID, err)
	}
	if payload.SchemaVersion < 1 || strings.TrimSpace(payload.RunID) == "" || strings.TrimSpace(payload.SlotID) == "" || strings.TrimSpace(payload.AttemptID) == "" {
		return EcommerceQAReviewView{}, fmt.Errorf("ecommerce QA artifact %s has incomplete lineage", artifact.ID)
	}
	if payload.ReviewID == "" {
		payload.ReviewID = artifact.ID
	}
	return EcommerceQAReviewView{
		ArtifactID: artifact.ID, Revision: artifact.Revision, SchemaVersion: payload.SchemaVersion,
		ReviewID: payload.ReviewID, RunID: payload.RunID, SlotID: payload.SlotID, AttemptID: payload.AttemptID,
		ResultID: payload.ResultID, Decision: payload.Decision, Action: payload.Action,
		IssueCodes: payload.IssueCodes, Note: payload.Note, Dimensions: payload.Dimensions,
		RuntimeEvidence: payload.RuntimeEvidence, ReviewerUserID: payload.ReviewerUserID,
		Source: payload.Source, ReviewedAt: payload.ReviewedAt,
	}, nil
}

func ecommerceQAReplayMatches(review EcommerceQAReviewView, normalized ecommerceNormalizedQAReview) bool {
	return review.ReviewID == normalized.ReviewID &&
		review.AttemptID == normalized.AttemptID &&
		review.ResultID == normalized.ResultID &&
		review.Decision == normalized.Decision &&
		review.Action == normalized.Action &&
		review.Note == normalized.Note &&
		reflect.DeepEqual(review.IssueCodes, normalized.IssueCodes) &&
		reflect.DeepEqual(review.Dimensions, normalized.Dimensions)
}
