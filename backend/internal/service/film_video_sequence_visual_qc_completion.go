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

type filmVideoSequenceVisualQCOutput struct {
	SchemaVersion   int                             `json:"schemaVersion"`
	OverallDecision model.FilmProductionQCDecision  `json:"overallDecision"`
	Dimensions      []filmVideoVisualQCDimension    `json:"dimensions"`
	Transitions     []filmVideoSequenceQCTransition `json:"transitions"`
	Summary         string                          `json:"summary"`
}

type filmVideoSequenceQCTransition struct {
	FromShotID   string                         `json:"fromShotId"`
	ToShotID     string                         `json:"toShotId"`
	Decision     model.FilmProductionQCDecision `json:"decision"`
	IssueCodes   []string                       `json:"issueCodes"`
	Observations []string                       `json:"observations"`
	Rationale    string                         `json:"rationale"`
}

func (s *Service) saveFilmVideoSequenceVisualQCTaskCompletionWithinStorageQuota(task *model.Task, resultJSON []byte) error {
	if task == nil || task.Type != model.FilmVisualQCTaskTypeSequence {
		return errors.New("Film video sequence visual QC completion requires a sequence visual QC task")
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
	attempt, err := s.repo.FilmVideoSequenceVisualQCAttemptByTaskID(task.ID)
	if err != nil {
		return err
	}
	var evidence []model.FilmVideoSequenceVisualQCSlotSnapshot
	if json.Unmarshal([]byte(attempt.SlotEvidenceJSON), &evidence) != nil || len(evidence) == 0 {
		return errors.New("整组视频连续性 QC 采样证据已损坏")
	}
	output, err := parseFilmVideoSequenceVisualQCResult(resultJSON, evidence)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	command, err := buildFilmVideoSequenceVisualQCCompletion(*task, *attempt, evidence, output, now)
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
	completed.Stage = "整组模型连续性 QC 等待人工裁决"
	completed.Progress = 100
	completed.ResultJSON = string(resultJSON)
	completed.InputJSON = publicInputJSON
	completed.Error = ""
	completed.LeaseOwner = ""
	completed.LeaseExpiresAt = nil
	completed.CompletedAt = &now
	if err := s.repo.SaveFilmVideoSequenceVisualQCTaskCompletion(&completed, expectedStatus, command); err != nil {
		return err
	}
	*task = completed
	return nil
}

func parseFilmVideoSequenceVisualQCResult(raw []byte, evidence []model.FilmVideoSequenceVisualQCSlotSnapshot) (filmVideoSequenceVisualQCOutput, error) {
	var provider filmVideoVisualQCProviderResult
	if json.Unmarshal(raw, &provider) != nil || provider.Mode != "text" || strings.TrimSpace(provider.Text) == "" {
		return filmVideoSequenceVisualQCOutput{}, errors.New("整组视频连续性 QC 模型没有返回文本结果")
	}
	text := strings.TrimSpace(provider.Text)
	if len(text) > 64<<10 || strings.Contains(text, "```") || !strings.HasPrefix(text, "{") || !strings.HasSuffix(text, "}") {
		return filmVideoSequenceVisualQCOutput{}, errors.New("整组视频连续性 QC 模型输出无效、包含 Markdown fence 或超过 64KB")
	}
	decoder := json.NewDecoder(bytes.NewReader([]byte(text)))
	decoder.DisallowUnknownFields()
	var output filmVideoSequenceVisualQCOutput
	if err := decoder.Decode(&output); err != nil {
		return filmVideoSequenceVisualQCOutput{}, fmt.Errorf("解析整组视频连续性 QC 结构化输出失败：%w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return filmVideoSequenceVisualQCOutput{}, errors.New("整组视频连续性 QC 输出包含多余 JSON 内容")
	}
	if err := validateFilmVideoSequenceVisualQCOutput(output, evidence); err != nil {
		return filmVideoSequenceVisualQCOutput{}, err
	}
	return output, nil
}

func validateFilmVideoSequenceVisualQCOutput(output filmVideoSequenceVisualQCOutput, evidence []model.FilmVideoSequenceVisualQCSlotSnapshot) error {
	if output.SchemaVersion != 1 {
		return errors.New("整组视频连续性 QC schemaVersion 必须为 1")
	}
	if len(output.Dimensions) != len(filmVideoSequenceVisualQCDimensions) {
		return fmt.Errorf("整组视频连续性 QC 必须完整返回 %d 个连续性维度", len(filmVideoSequenceVisualQCDimensions))
	}
	worst := model.FilmProductionQCDecisionPass
	for index, dimension := range output.Dimensions {
		if dimension.Dimension != filmVideoSequenceVisualQCDimensions[index] {
			return fmt.Errorf("整组视频连续性 QC 维度 %d 必须为 %s", index+1, filmVideoSequenceVisualQCDimensions[index])
		}
		if err := validateFilmVideoSequenceVisualQCVerdict(dimension.Decision, dimension.IssueCodes, dimension.Observations, dimension.Rationale, "维度 "+dimension.Dimension); err != nil {
			return err
		}
		if dimension.Dimension == "lip_sync_audio_continuity" && dimension.Decision != model.FilmProductionQCDecisionUncertain {
			return errors.New("整组视频连续性 QC 未提供音频证据，lip_sync_audio_continuity 必须为 UNCERTAIN")
		}
		worst = worseFilmVideoSequenceQCDecision(worst, dimension.Decision)
	}
	wantTransitions := len(evidence) - 1
	if wantTransitions < 0 {
		wantTransitions = 0
	}
	if len(output.Transitions) != wantTransitions {
		return fmt.Errorf("整组视频连续性 QC 必须按顺序返回 %d 个相邻镜头 transition", wantTransitions)
	}
	for index, transition := range output.Transitions {
		if transition.FromShotID != evidence[index].ShotID || transition.ToShotID != evidence[index+1].ShotID {
			return fmt.Errorf("整组视频连续性 QC transition %d 的镜头顺序与锁定序列不一致", index+1)
		}
		if err := validateFilmVideoSequenceVisualQCVerdict(transition.Decision, transition.IssueCodes, transition.Observations, transition.Rationale, fmt.Sprintf("transition %d", index+1)); err != nil {
			return err
		}
		worst = worseFilmVideoSequenceQCDecision(worst, transition.Decision)
	}
	if output.OverallDecision != worst {
		return fmt.Errorf("整组视频连续性 QC overallDecision=%s 与逐维度/transition 结论 %s 不一致", output.OverallDecision, worst)
	}
	if strings.TrimSpace(output.Summary) == "" || utf8.RuneCountInString(output.Summary) > 2000 {
		return errors.New("整组视频连续性 QC summary 为空或超过 2000 字")
	}
	return nil
}

func validateFilmVideoSequenceVisualQCVerdict(decision model.FilmProductionQCDecision, issueCodes []string, observations []string, rationale string, label string) error {
	if decision != model.FilmProductionQCDecisionPass && decision != model.FilmProductionQCDecisionUncertain && decision != model.FilmProductionQCDecisionFail {
		return fmt.Errorf("整组视频连续性 QC %s 的 decision 无效", label)
	}
	if len(observations) == 0 || len(observations) > 8 {
		return fmt.Errorf("整组视频连续性 QC %s 必须包含 1-8 条可见事实", label)
	}
	for _, observation := range observations {
		if strings.TrimSpace(observation) == "" || utf8.RuneCountInString(observation) > 500 {
			return fmt.Errorf("整组视频连续性 QC %s 的 observation 为空或过长", label)
		}
	}
	if strings.TrimSpace(rationale) == "" || utf8.RuneCountInString(rationale) > 1000 {
		return fmt.Errorf("整组视频连续性 QC %s 的 rationale 为空或过长", label)
	}
	seen := map[string]bool{}
	for _, rawCode := range issueCodes {
		code := strings.ToUpper(strings.TrimSpace(rawCode))
		if code != rawCode || !filmProductionIssueCodePattern.MatchString(code) || seen[code] {
			return fmt.Errorf("整组视频连续性 QC %s 的 issueCode 无效或重复", label)
		}
		seen[code] = true
	}
	if decision != model.FilmProductionQCDecisionPass && len(issueCodes) == 0 {
		return fmt.Errorf("整组视频连续性 QC 非 PASS %s 必须给出 issueCode", label)
	}
	return nil
}

func worseFilmVideoSequenceQCDecision(current model.FilmProductionQCDecision, next model.FilmProductionQCDecision) model.FilmProductionQCDecision {
	if current == model.FilmProductionQCDecisionFail || next == model.FilmProductionQCDecisionFail {
		return model.FilmProductionQCDecisionFail
	}
	if current == model.FilmProductionQCDecisionUncertain || next == model.FilmProductionQCDecisionUncertain {
		return model.FilmProductionQCDecisionUncertain
	}
	return model.FilmProductionQCDecisionPass
}

func buildFilmVideoSequenceVisualQCCompletion(task model.Task, attempt model.FilmVideoSequenceVisualQCAttempt, evidence []model.FilmVideoSequenceVisualQCSlotSnapshot, output filmVideoSequenceVisualQCOutput, at time.Time) (repository.FilmVideoSequenceVisualQCCompletionCommand, error) {
	if attempt.TaskID != task.ID || attempt.UserID != task.UserID || attempt.ProjectID != task.ProjectID || len(evidence) == 0 {
		return repository.FilmVideoSequenceVisualQCCompletionCommand{}, repository.ErrFilmProductionStateConflict
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
	for _, transition := range output.Transitions {
		for _, code := range transition.IssueCodes {
			if !seen[code] {
				seen[code] = true
				issueCodes = append(issueCodes, code)
			}
		}
	}
	reportID, artifactID, revisionID := newID(), newID(), newID()
	evidencePayload := map[string]any{
		"schemaVersion": 1, "assessmentKind": "sequence_visual_continuity_qc", "mediaState": model.FilmContinuityMediaStateAvailable,
		"humanDecisionRequired": true, "modelAttemptId": attempt.ID, "scopeFingerprint": attempt.ScopeFingerprint,
		"sampling": map[string]any{
			"method": "client_browser_capture", "slots": evidence, "provenanceVerifiedByServerDecoder": false, "audioEvidenceAttached": false,
			"limitation": "Frame resources and timestamps are frozen, but the backend did not decode source videos to independently prove frame origin. No audio evidence is attached.",
		},
		"dimensions": output.Dimensions, "transitions": output.Transitions,
		"model": map[string]any{"logicalModelId": attempt.LogicalModelID, "logicalModelRevisionId": attempt.LogicalModelRevisionID, "code": attempt.Model},
	}
	report := &model.FilmVideoSequenceReview{
		ID: reportID, UserID: attempt.UserID, IdempotencyKey: "film-video-sequence-visual-qc:" + attempt.ID,
		ProjectID: attempt.ProjectID, RootRunID: attempt.RootRunID, SequenceID: attempt.SequenceID, LedgerID: attempt.LedgerID,
		Decision: model.FilmVideoSequenceReviewDecision(output.OverallDecision), Action: model.FilmVideoSequenceReviewActionHold,
		IssueCodesJSON: mustFilmJSON(issueCodes), EvidenceJSON: mustFilmJSON(evidencePayload), Note: strings.TrimSpace(output.Summary),
		Source: "model", AssessmentKind: "sequence_visual_continuity_qc", ModelAttemptID: attempt.ID,
		ScopeFingerprint: attempt.ScopeFingerprint, ArtifactID: artifactID, RevisionID: revisionID, CreatedAt: at,
	}
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 2, "artifactType": "sequence-review", "reviewId": report.ID, "sequenceId": attempt.SequenceID,
		"ledgerId": attempt.LedgerID, "modelAttemptId": attempt.ID, "decision": report.Decision, "action": report.Action,
		"issueCodes": issueCodes, "evidence": evidencePayload, "note": report.Note, "source": report.Source,
		"assessmentKind": report.AssessmentKind, "scopeFingerprint": report.ScopeFingerprint,
	})
	if err != nil {
		return repository.FilmVideoSequenceVisualQCCompletionCommand{}, err
	}
	artifact := &model.ProductionArtifact{
		ID: artifactID, UserID: attempt.UserID, ProjectID: attempt.ProjectID, Domain: "film",
		ArtifactType: "sequence-review", LogicalKey: "film-video:sequence-model-visual-qc:" + attempt.ID,
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
	refs := []map[string]any{
		{"artifactId": attempt.LedgerArtifactID, "revisionId": attempt.LedgerRevisionID, "type": "continuity-ledger", "digest": attempt.LedgerDigest},
		{"artifactId": attempt.PromptArtifactID, "revisionId": attempt.PromptRevisionID, "type": "ai-video-prompts", "digest": attempt.PromptDigest},
	}
	for _, slot := range evidence {
		refs = append(refs,
			map[string]any{"artifactId": slot.SourceResultArtifactID, "revisionId": slot.SourceResultRevisionID, "type": "generation-result", "digest": slot.SourceResultDigest},
			map[string]any{"artifactId": slot.SourceImageArtifactID, "revisionId": slot.SourceImageRevisionID, "type": "generation-result", "digest": slot.SourceImageRevisionDigest},
		)
	}
	revision := &model.ProductionArtifactRevision{
		ID: revisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(content), ContentDigest: digestBytesHex(content),
		SourceRunID: attempt.RootRunID, SourceAttemptID: attempt.ID, SourceArtifactRefsJSON: mustFilmJSON(refs),
		AuthorityRefsJSON: mustFilmJSON(authorities), CreatedByType: "agent", CreatedByID: "quality_control_editor", CreatedAt: at,
	}
	return repository.FilmVideoSequenceVisualQCCompletionCommand{
		AttemptID: attempt.ID, Report: report, Artifact: artifact, Revision: revision, At: at,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: "film.production.video.sequence.visual_qc.completed", ActorType: "agent", ActorID: "quality_control_editor",
			PayloadJSON: mustFilmJSON(map[string]any{"modelAttemptId": attempt.ID, "sequenceId": attempt.SequenceID, "reportId": report.ID, "decision": report.Decision}),
		},
	}, nil
}
