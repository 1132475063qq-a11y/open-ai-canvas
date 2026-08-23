package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"

	"gorm.io/gorm"
)

type CreateFilmVideoSequenceReviewRequest struct {
	Decision   model.FilmVideoSequenceReviewDecision `json:"decision"`
	Action     model.FilmVideoSequenceReviewAction   `json:"action"`
	IssueCodes []string                              `json:"issueCodes"`
	Evidence   map[string]any                        `json:"evidence"`
	Note       string                                `json:"note"`
}

type FilmVideoSequenceReviewView struct {
	ID               string                                `json:"id"`
	SequenceID       string                                `json:"sequenceId"`
	LedgerID         string                                `json:"ledgerId"`
	Decision         model.FilmVideoSequenceReviewDecision `json:"decision"`
	Action           model.FilmVideoSequenceReviewAction   `json:"action"`
	IssueCodes       []string                              `json:"issueCodes"`
	Evidence         map[string]any                        `json:"evidence"`
	Note             string                                `json:"note"`
	Source           string                                `json:"source"`
	AssessmentKind   string                                `json:"assessmentKind,omitempty"`
	ModelAttemptID   string                                `json:"modelAttemptId,omitempty"`
	ReviewerUserID   string                                `json:"reviewerUserId,omitempty"`
	ScopeFingerprint string                                `json:"scopeFingerprint"`
	ArtifactID       string                                `json:"artifactId"`
	RevisionID       string                                `json:"revisionId"`
	CreatedAt        time.Time                             `json:"createdAt"`
	Valid            bool                                  `json:"valid"`
}

type CreateFilmVideoSequenceReviewResult struct {
	Review     FilmVideoSequenceReviewView `json:"review"`
	Sequence   FilmVideoSequenceView       `json:"sequence"`
	Idempotent bool                        `json:"idempotent"`
}

func (s *Service) CreateFilmVideoSequenceReview(userID string, projectID string, sequenceID string, idempotencyKey string, request CreateFilmVideoSequenceReviewRequest) (CreateFilmVideoSequenceReviewResult, error) {
	if _, err := s.requireMutableFilmProject(userID, projectID); err != nil {
		return CreateFilmVideoSequenceReviewResult{}, err
	}
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if !filmAgentIdempotencyKeyPattern.MatchString(idempotencyKey) {
		return CreateFilmVideoSequenceReviewResult{}, BadAuthRequest("X-Idempotency-Key 必须为 8-128 位字母、数字或 ._:-")
	}
	detail, err := s.repo.FilmVideoSequenceForUser(userID, projectID, strings.TrimSpace(sequenceID))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CreateFilmVideoSequenceReviewResult{}, NotFound("视频序列不存在")
	}
	if err != nil {
		return CreateFilmVideoSequenceReviewResult{}, err
	}
	if !filmVideoSequenceHasAcceptedSlots(*detail) {
		return CreateFilmVideoSequenceReviewResult{}, conflictError("所有视频槽位通过人工验收后，才能进行整组连续性裁决")
	}
	if detail.Continuity == nil {
		return CreateFilmVideoSequenceReviewResult{}, conflictError("视频序列缺少 Continuity Ledger，不能进行整组裁决")
	}
	issueCodes, evidence, note, err := normalizeFilmVideoSequenceReviewInput(request, detail.Slots)
	if err != nil {
		return CreateFilmVideoSequenceReviewResult{}, err
	}
	scopeFingerprint, err := filmVideoSequenceScopeFingerprint(*detail)
	if err != nil {
		return CreateFilmVideoSequenceReviewResult{}, err
	}
	now := time.Now().UTC()
	review := &model.FilmVideoSequenceReview{
		ID: newID(), UserID: userID, IdempotencyKey: idempotencyKey, ProjectID: projectID,
		RootRunID: detail.Sequence.RootRunID, SequenceID: detail.Sequence.ID, LedgerID: detail.Continuity.Ledger.ID,
		Decision: request.Decision, Action: request.Action, IssueCodesJSON: mustFilmJSON(issueCodes),
		EvidenceJSON: mustFilmJSON(evidence), Note: note, Source: "human", ReviewerUserID: userID,
		ScopeFingerprint: scopeFingerprint, ArtifactID: newID(), RevisionID: newID(), CreatedAt: now,
	}
	artifact, revision, err := buildFilmVideoSequenceReviewArtifact(*review, detail.Sequence, detail.Continuity.Ledger, now)
	if err != nil {
		return CreateFilmVideoSequenceReviewResult{}, err
	}
	eventType := "film.production.video.sequence.review.held"
	if request.Action == model.FilmVideoSequenceReviewActionAccept {
		eventType = "film.production.video.sequence.review.accepted"
	} else if request.Action == model.FilmVideoSequenceReviewActionRetry {
		eventType = "film.production.video.sequence.review.retry_requested"
	}
	stored, idempotent, err := s.repo.CreateFilmVideoSequenceReview(repository.FilmVideoSequenceReviewCreateCommand{
		UserID: userID, ProjectID: projectID, Review: review, Artifact: artifact, Revision: revision, At: now,
		Event: repository.AgentRuntimeEventInput{
			ID: newID(), EventType: eventType, ActorType: "human", ActorID: userID,
			PayloadJSON: mustFilmJSON(map[string]any{"sequenceId": detail.Sequence.ID, "reviewId": review.ID, "decision": review.Decision, "action": review.Action}),
		},
	})
	if err != nil {
		return CreateFilmVideoSequenceReviewResult{}, mapFilmVideoError(err)
	}
	updated, err := s.repo.FilmVideoSequenceForUser(userID, projectID, detail.Sequence.ID)
	if err != nil {
		return CreateFilmVideoSequenceReviewResult{}, err
	}
	sequenceView, err := s.filmVideoSequenceView(*updated)
	if err != nil {
		return CreateFilmVideoSequenceReviewResult{}, err
	}
	reviewView, err := filmVideoSequenceReviewView(*stored, *updated)
	if err != nil {
		return CreateFilmVideoSequenceReviewResult{}, err
	}
	return CreateFilmVideoSequenceReviewResult{Review: reviewView, Sequence: sequenceView, Idempotent: idempotent}, nil
}

func filmVideoSequenceHasAcceptedSlots(detail repository.FilmVideoSequenceDetail) bool {
	if len(detail.Slots) == 0 {
		return false
	}
	for _, slot := range detail.Slots {
		if slot.Slot.Status != model.FilmVideoSlotStatusAccepted || slot.Slot.CurrentAttemptID == "" || slot.Slot.ResultID == "" {
			return false
		}
	}
	return true
}

func filmVideoSequenceReviewAllowsRetry(detail repository.FilmVideoSequenceDetail, slot repository.FilmVideoSlotDetail, retryOf string) bool {
	if detail.SequenceReview == nil || detail.SequenceReview.Decision != model.FilmVideoSequenceReviewDecisionFail ||
		detail.SequenceReview.Action != model.FilmVideoSequenceReviewActionRetry || slot.Slot.Status != model.FilmVideoSlotStatusAccepted ||
		len(slot.Attempts) == 0 || slot.Attempts[0].Attempt.ID != retryOf {
		return false
	}
	currentScope, err := filmVideoSequenceScopeFingerprint(detail)
	return err == nil && detail.SequenceReview.ScopeFingerprint == currentScope
}

func normalizeFilmVideoSequenceReviewInput(request CreateFilmVideoSequenceReviewRequest, slots []repository.FilmVideoSlotDetail) ([]string, map[string]any, string, error) {
	qcRequest := CreateFilmProductionQCRequest{Decision: model.FilmProductionQCDecision(request.Decision), Action: model.FilmProductionQCAction(request.Action), IssueCodes: request.IssueCodes, Evidence: request.Evidence, Note: request.Note}
	issueCodes, evidence, note, err := normalizeFilmProductionQCInput(qcRequest)
	if err != nil {
		return nil, nil, "", err
	}
	dimensions, ok := evidence["dimensions"].(map[string]any)
	if request.Decision == model.FilmVideoSequenceReviewDecisionPass {
		if !ok {
			return nil, nil, "", BadAuthRequest("整组 PASS 必须提交跨镜头连续性维度裁决")
		}
		for _, dimension := range filmContinuityDimensions {
			if value, _ := dimensions[dimension].(string); !strings.EqualFold(strings.TrimSpace(value), "PASS") {
				return nil, nil, "", BadAuthRequest("整组 PASS 必须通过人物、服装、道具、场景、轴线等全部连续性维度")
			}
		}
		shotVerdicts, ok := evidence["shots"].(map[string]any)
		if !ok {
			return nil, nil, "", BadAuthRequest("整组 PASS 必须提交逐镜连续性裁决")
		}
		for _, slot := range slots {
			if value, _ := shotVerdicts[slot.Slot.ShotID].(string); !strings.EqualFold(strings.TrimSpace(value), "PASS") {
				return nil, nil, "", BadAuthRequest("整组 PASS 必须通过全部镜头的连续性裁决")
			}
		}
	}
	return issueCodes, evidence, note, nil
}

func filmVideoSequenceScopeFingerprint(detail repository.FilmVideoSequenceDetail) (string, error) {
	type slotScope struct {
		ID               string                    `json:"id"`
		Position         int                       `json:"position"`
		CurrentAttemptID string                    `json:"currentAttemptId"`
		ResultID         string                    `json:"resultId"`
		Status           model.FilmVideoSlotStatus `json:"status"`
		Revision         int64                     `json:"revision"`
	}
	scope := struct {
		SequenceID       string      `json:"sequenceId"`
		SequenceRevision int64       `json:"sequenceRevision"`
		Slots            []slotScope `json:"slots"`
		LedgerID         string      `json:"ledgerId"`
		LedgerRevisionID string      `json:"ledgerRevisionId"`
	}{SequenceID: detail.Sequence.ID, SequenceRevision: detail.Sequence.Revision}
	if detail.Continuity != nil {
		scope.LedgerID = detail.Continuity.Ledger.ID
		scope.LedgerRevisionID = detail.Continuity.Ledger.ArtifactRevisionID
	}
	for _, slot := range detail.Slots {
		scope.Slots = append(scope.Slots, slotScope{ID: slot.Slot.ID, Position: slot.Slot.Position, CurrentAttemptID: slot.Slot.CurrentAttemptID, ResultID: slot.Slot.ResultID, Status: slot.Slot.Status, Revision: slot.Slot.Revision})
	}
	encoded, err := json.Marshal(scope)
	if err != nil {
		return "", err
	}
	return digestBytesHex(encoded), nil
}

func filmVideoSequenceReviewView(review model.FilmVideoSequenceReview, detail repository.FilmVideoSequenceDetail) (FilmVideoSequenceReviewView, error) {
	issueCodes := []string{}
	evidence := map[string]any{}
	if json.Unmarshal([]byte(firstNonEmpty(review.IssueCodesJSON, "[]")), &issueCodes) != nil || json.Unmarshal([]byte(firstNonEmpty(review.EvidenceJSON, "{}")), &evidence) != nil {
		return FilmVideoSequenceReviewView{}, errors.New("Film 整组审查记录已损坏")
	}
	currentScope, err := filmVideoSequenceScopeFingerprint(detail)
	if err != nil {
		return FilmVideoSequenceReviewView{}, err
	}
	return FilmVideoSequenceReviewView{
		ID: review.ID, SequenceID: review.SequenceID, LedgerID: review.LedgerID, Decision: review.Decision, Action: review.Action,
		IssueCodes: issueCodes, Evidence: evidence, Note: review.Note, Source: review.Source, AssessmentKind: review.AssessmentKind,
		ModelAttemptID: review.ModelAttemptID, ReviewerUserID: review.ReviewerUserID,
		ScopeFingerprint: review.ScopeFingerprint, ArtifactID: review.ArtifactID, RevisionID: review.RevisionID, CreatedAt: review.CreatedAt,
		Valid: review.ScopeFingerprint == currentScope,
	}, nil
}

func buildFilmVideoSequenceReviewArtifact(review model.FilmVideoSequenceReview, sequence model.FilmVideoSequence, ledger model.FilmContinuityLedger, at time.Time) (*model.ProductionArtifact, *model.ProductionArtifactRevision, error) {
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "artifactType": "sequence-review", "reviewId": review.ID, "sequenceId": sequence.ID, "ledgerId": ledger.ID,
		"decision": review.Decision, "action": review.Action, "issueCodes": json.RawMessage(firstNonEmpty(review.IssueCodesJSON, "[]")),
		"evidence": json.RawMessage(firstNonEmpty(review.EvidenceJSON, "{}")), "note": review.Note, "source": review.Source,
		"reviewerUserId": review.ReviewerUserID, "scopeFingerprint": review.ScopeFingerprint,
	})
	if err != nil {
		return nil, nil, err
	}
	artifact := &model.ProductionArtifact{ID: review.ArtifactID, UserID: review.UserID, ProjectID: review.ProjectID, Domain: "film", ArtifactType: "sequence-review", LogicalKey: "film-video:sequence-review:" + review.ID}
	revision := &model.ProductionArtifactRevision{
		ID: review.RevisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(content), ContentDigest: digestBytesHex(content),
		SourceRunID: sequence.RootRunID, SourceArtifactRefsJSON: mustFilmJSON([]map[string]any{{"artifactId": sequence.ArtifactID, "revisionId": sequence.ArtifactRevisionID, "type": "video-sequence"}, {"artifactId": ledger.ArtifactID, "revisionId": ledger.ArtifactRevisionID, "type": "continuity-ledger"}}),
		AuthorityRefsJSON: mustFilmJSON([]map[string]any{{"kind": "human", "id": review.ReviewerUserID}, {"kind": "skill", "id": "continuity-check", "version": "3.0.0"}}),
		CreatedByType:     "human", CreatedByID: review.ReviewerUserID, CreatedAt: at,
	}
	return artifact, revision, nil
}
