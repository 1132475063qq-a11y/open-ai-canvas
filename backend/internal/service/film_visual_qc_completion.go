package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
	"unicode/utf8"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

type filmVisualQCProviderResult struct {
	Mode string `json:"mode"`
	Text string `json:"text"`
}

type filmVisualQCOutput struct {
	SchemaVersion   int                            `json:"schemaVersion"`
	OverallDecision model.FilmProductionQCDecision `json:"overallDecision"`
	Dimensions      []filmVisualQCDimension        `json:"dimensions"`
	Summary         string                         `json:"summary"`
}

type filmVisualQCDimension struct {
	Dimension    string                         `json:"dimension"`
	Decision     model.FilmProductionQCDecision `json:"decision"`
	IssueCodes   []string                       `json:"issueCodes"`
	Observations []string                       `json:"observations"`
	Rationale    string                         `json:"rationale"`
}

func (s *Service) saveFilmVisualQCTaskCompletionWithinStorageQuota(task *model.Task, resultJSON []byte) error {
	if task == nil || task.Type != model.FilmVisualQCTaskTypeImage {
		return errors.New("Film visual QC completion requires a visual QC task")
	}
	policy, err := s.RuntimePolicy()
	if err != nil {
		return err
	}
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	usage, err := s.repo.UserStorageUsage(task.UserID)
	if err != nil {
		return err
	}
	attempt, err := s.repo.FilmVisualQCAttemptByTaskID(task.ID)
	if err != nil {
		return err
	}
	output, err := parseFilmVisualQCResult(resultJSON)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	command, err := buildFilmVisualQCCompletion(*task, *attempt, output, now)
	if err != nil {
		return err
	}
	publicInputJSON := publicTaskInputJSON(task.InputJSON)
	taskDelta := int64(len(resultJSON) + len(publicInputJSON) - len(task.ResultJSON) - len(task.InputJSON))
	if err := validateTaskDataGrowthQuotaWithPolicy(usage, taskDelta, policy.Resource); err != nil {
		return err
	}
	expectedStatus := task.Status
	completed := *task
	completed.Status = model.TaskStatusSucceeded
	completed.Stage = "视觉 QC 等待人工裁决"
	completed.Progress = 100
	completed.ResultJSON = string(resultJSON)
	completed.InputJSON = publicInputJSON
	completed.Error = ""
	completed.LeaseOwner = ""
	completed.LeaseExpiresAt = nil
	completed.CompletedAt = &now
	if err := s.repo.SaveFilmVisualQCTaskCompletion(&completed, expectedStatus, command); err != nil {
		return err
	}
	*task = completed
	return nil
}

func parseFilmVisualQCResult(raw []byte) (filmVisualQCOutput, error) {
	var provider filmVisualQCProviderResult
	if len(raw) == 0 || json.Unmarshal(raw, &provider) != nil || provider.Mode != "text" || strings.TrimSpace(provider.Text) == "" {
		return filmVisualQCOutput{}, errors.New("视觉 QC 模型没有返回文本结果")
	}
	text := strings.TrimSpace(provider.Text)
	if len(text) > maxFilmProductionQCBytes || !utf8.ValidString(text) || strings.HasPrefix(text, "```") {
		return filmVisualQCOutput{}, errors.New("视觉 QC 模型输出无效、包含 Markdown fence 或超过 64KB")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(text))
	decoder.DisallowUnknownFields()
	var output filmVisualQCOutput
	if err := decoder.Decode(&output); err != nil {
		return filmVisualQCOutput{}, fmt.Errorf("解析视觉 QC 结构化输出失败：%w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return filmVisualQCOutput{}, errors.New("视觉 QC 输出包含多余 JSON 内容")
	}
	if err := validateFilmVisualQCOutput(output); err != nil {
		return filmVisualQCOutput{}, err
	}
	return output, nil
}

func validateFilmVisualQCOutput(output filmVisualQCOutput) error {
	if output.SchemaVersion != 1 {
		return errors.New("视觉 QC schemaVersion 必须为 1")
	}
	if len(output.Dimensions) != len(filmVisualQCDimensions) {
		return errors.New("视觉 QC 必须完整返回八个语义维度")
	}
	worst := model.FilmProductionQCDecisionPass
	for index, dimension := range output.Dimensions {
		if dimension.Dimension != filmVisualQCDimensions[index] {
			return fmt.Errorf("视觉 QC 维度 %d 必须为 %s", index+1, filmVisualQCDimensions[index])
		}
		if dimension.Decision != model.FilmProductionQCDecisionPass && dimension.Decision != model.FilmProductionQCDecisionUncertain && dimension.Decision != model.FilmProductionQCDecisionFail {
			return fmt.Errorf("视觉 QC 维度 %s 的 decision 无效", dimension.Dimension)
		}
		if len(dimension.Observations) == 0 || len(dimension.Observations) > 8 {
			return fmt.Errorf("视觉 QC 维度 %s 必须包含 1-8 条可见事实", dimension.Dimension)
		}
		for _, observation := range dimension.Observations {
			if strings.TrimSpace(observation) == "" || utf8.RuneCountInString(observation) > 500 {
				return fmt.Errorf("视觉 QC 维度 %s 的 observation 为空或过长", dimension.Dimension)
			}
		}
		if strings.TrimSpace(dimension.Rationale) == "" || utf8.RuneCountInString(dimension.Rationale) > 1000 {
			return fmt.Errorf("视觉 QC 维度 %s 的 rationale 为空或过长", dimension.Dimension)
		}
		seen := map[string]bool{}
		for _, rawCode := range dimension.IssueCodes {
			code := strings.ToUpper(strings.TrimSpace(rawCode))
			if code != rawCode || !filmProductionIssueCodePattern.MatchString(code) || seen[code] {
				return fmt.Errorf("视觉 QC 维度 %s 的 issueCode 无效或重复", dimension.Dimension)
			}
			seen[code] = true
		}
		if dimension.Decision != model.FilmProductionQCDecisionPass && len(dimension.IssueCodes) == 0 {
			return fmt.Errorf("视觉 QC 非 PASS 维度 %s 必须给出 issueCode", dimension.Dimension)
		}
		if dimension.Decision == model.FilmProductionQCDecisionFail {
			worst = model.FilmProductionQCDecisionFail
		} else if dimension.Decision == model.FilmProductionQCDecisionUncertain && worst != model.FilmProductionQCDecisionFail {
			worst = model.FilmProductionQCDecisionUncertain
		}
	}
	if output.OverallDecision != worst {
		return fmt.Errorf("视觉 QC overallDecision=%s 与逐维度结论 %s 不一致", output.OverallDecision, worst)
	}
	if strings.TrimSpace(output.Summary) == "" || utf8.RuneCountInString(output.Summary) > 2000 {
		return errors.New("视觉 QC summary 为空或超过 2000 字")
	}
	return nil
}

func buildFilmVisualQCCompletion(task model.Task, attempt model.FilmVisualQCAttempt, output filmVisualQCOutput, at time.Time) (repository.FilmVisualQCCompletionCommand, error) {
	if attempt.TaskID != task.ID || attempt.UserID != task.UserID || attempt.ProjectID != task.ProjectID {
		return repository.FilmVisualQCCompletionCommand{}, repository.ErrFilmProductionStateConflict
	}
	issueCodes := make([]string, 0)
	seen := map[string]bool{}
	for _, dimension := range output.Dimensions {
		for _, code := range dimension.IssueCodes {
			if !seen[code] {
				seen[code] = true
				issueCodes = append(issueCodes, code)
			}
		}
	}
	reportID, artifactID, revisionID := newID(), newID(), newID()
	evidence := map[string]any{
		"schemaVersion": 1, "assessmentKind": "visual_semantic_qc", "mediaState": model.FilmContinuityMediaStateAvailable,
		"humanDecisionRequired": true, "modelAttemptId": attempt.ID, "sourceAttemptId": attempt.SourceAttemptID, "sourceResultId": attempt.SourceResultID,
		"sourceResultRevisionId": attempt.SourceResultRevisionID, "dimensions": output.Dimensions,
		"model": map[string]any{"logicalModelId": attempt.LogicalModelID, "logicalModelRevisionId": attempt.LogicalModelRevisionID, "code": attempt.Model},
	}
	report := &model.FilmProductionQCReport{
		ID: reportID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, RootRunID: attempt.RootRunID, ShotID: attempt.ShotID,
		AttemptID: attempt.SourceAttemptID, ResultID: attempt.SourceResultID, Decision: output.OverallDecision, Action: model.FilmProductionQCActionHold,
		IssueCodesJSON: mustFilmJSON(issueCodes), EvidenceJSON: mustFilmJSON(evidence), Note: strings.TrimSpace(output.Summary), Source: "model",
		AssessmentKind: "visual_semantic_qc", ModelAttemptID: attempt.ID, IdempotencyKey: "film-visual-qc:" + attempt.ID,
		ArtifactID: artifactID, RevisionID: revisionID, CreatedAt: at,
	}
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 3, "artifactType": "generation-qc-report", "mediaType": "image", "reportId": report.ID,
		"attemptId": attempt.SourceAttemptID, "modelAttemptId": attempt.ID, "resultId": attempt.SourceResultID,
		"decision": report.Decision, "action": report.Action, "issueCodes": issueCodes, "evidence": evidence,
		"note": report.Note, "source": report.Source, "assessmentKind": report.AssessmentKind, "mediaState": model.FilmContinuityMediaStateAvailable,
	})
	if err != nil {
		return repository.FilmVisualQCCompletionCommand{}, err
	}
	artifact := &model.ProductionArtifact{
		ID: artifactID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, Domain: "film",
		ArtifactType: "generation-qc-report", LogicalKey: "film-production:model-visual-qc:" + attempt.ID,
	}
	authorities := []map[string]any{
		{"kind": "registry", "id": attempt.RegistryID, "version": attempt.RegistryVersion, "digest": attempt.RegistryDigest},
		{"kind": "agent", "id": "quality_control_editor"},
		{"kind": "logical_model", "id": attempt.LogicalModelID, "revisionId": attempt.LogicalModelRevisionID},
		{"kind": "model_route", "id": attempt.RouteID, "channelModelId": attempt.ChannelModelID},
	}
	for _, skillID := range filmVisualQCSkillIDs {
		authorities = append(authorities, map[string]any{"kind": "skill", "id": skillID})
	}
	revision := &model.ProductionArtifactRevision{
		ID: revisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(content), ContentDigest: digestBytesHex(content),
		SourceRunID: attempt.RootRunID, SourceAttemptID: attempt.ID,
		SourceArtifactRefsJSON: mustFilmJSON([]map[string]any{
			{"artifactId": attempt.SourceResultArtifactID, "revisionId": attempt.SourceResultRevisionID, "type": "generation-result", "digest": attempt.SourceResultDigest},
		}),
		AuthorityRefsJSON: mustFilmJSON(authorities), CreatedByType: "agent", CreatedByID: "quality_control_editor", CreatedAt: at,
	}
	return repository.FilmVisualQCCompletionCommand{
		AttemptID: attempt.ID, Report: report, Artifact: artifact, Revision: revision, At: at,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "film.production.image.visual_qc.completed", ActorType: "agent", ActorID: "quality_control_editor",
			PayloadJSON: mustFilmJSON(map[string]any{"modelAttemptId": attempt.ID, "sourceAttemptId": attempt.SourceAttemptID, "reportId": report.ID, "decision": report.Decision, "humanDecisionRequired": true}),
		},
	}, nil
}
