package service

import (
	"encoding/json"
	"fmt"
	"time"

	"infinite-canvas/backend/internal/model"
	"infinite-canvas/backend/internal/repository"
)

const filmContinuityCompilerID = "film-continuity-compiler-v1"

type filmContinuityArtifactContent struct {
	SchemaVersion int                              `json:"schemaVersion"`
	ArtifactType  string                           `json:"artifactType"`
	LedgerID      string                           `json:"ledgerId"`
	SequenceID    string                           `json:"sequenceId"`
	MediaState    model.FilmContinuityMediaState   `json:"mediaState"`
	Status        model.FilmContinuityLedgerStatus `json:"status"`
	Dimensions    []string                         `json:"dimensions"`
	Shots         []filmContinuityArtifactShot     `json:"shots"`
	Issues        []filmContinuityArtifactIssue    `json:"issues"`
}

type filmContinuityArtifactShot struct {
	StateID       string                               `json:"stateId"`
	ShotID        string                               `json:"shotId"`
	Position      int                                  `json:"position"`
	Status        model.FilmContinuityShotStatus       `json:"status"`
	ReadIn        map[string]any                       `json:"readIn"`
	WriteOut      map[string]any                       `json:"writeOut"`
	Dimensions    map[string]string                    `json:"dimensions"`
	ReferenceLock map[string]any                       `json:"referenceLock"`
}

type filmContinuityArtifactIssue struct {
	IssueID      string `json:"issueId"`
	ShotID       string `json:"shotId"`
	Dimension    string `json:"dimension"`
	Severity     string `json:"severity"`
	Code         string `json:"code"`
	Message      string `json:"message"`
	Authority    string `json:"authority"`
	Owner        string `json:"owner"`
	RepairStatus string `json:"repairStatus"`
}

var filmContinuityDimensions = []string{
	"character_state",
	"costume_state",
	"prop_state",
	"scene_state",
	"knowledge_state",
	"audio_state",
	"screen_direction",
	"action_axis",
	"ui_state",
}

func buildFilmContinuityLedger(
	sequence model.FilmVideoSequence,
	slots []model.FilmVideoSlot,
	shots []model.Shot,
	at time.Time,
) (*model.FilmContinuityLedger, []model.FilmContinuityShotState, []model.FilmContinuityIssue, *model.ProductionArtifact, *model.ProductionArtifactRevision, error) {
	shotByID := make(map[string]model.Shot, len(shots))
	for _, shot := range shots {
		shotByID[shot.ID] = shot
	}
	ledgerID := newID()
	artifactID := newID()
	revisionID := newID()
	states := make([]model.FilmContinuityShotState, 0, len(slots))
	issues := make([]model.FilmContinuityIssue, 0, len(slots))
	stateContent := make([]map[string]any, 0, len(slots))
	issueContent := make([]map[string]any, 0, len(slots))
	sourceRefs := []map[string]any{{
		"artifactId": sequence.PromptArtifactID, "revisionId": sequence.PromptArtifactRevisionID,
		"type": "ai-video-prompts", "digest": sequence.PromptArtifactDigest,
	}}

	for index, slot := range slots {
		shot, ok := shotByID[slot.ShotID]
		if !ok {
			return nil, nil, nil, nil, nil, fmt.Errorf("continuity Ledger 缺少镜头 %s", slot.ShotID)
		}
		previousShotID := ""
		inheritance := "root_state"
		if index > 0 {
			previousShotID = slots[index-1].ShotID
			inheritance = "requires_semantic_review"
		}
		readIn := map[string]any{
			"fromShotId":  filmProductionEmptyStringAsNil(previousShotID),
			"inheritance": inheritance,
		}
		writeOut := map[string]any{
			"shotTitle":       shot.Title,
			"shotDescription": shot.Description,
			"videoPrompt":     slot.Prompt,
			"durationMs":      slot.DurationMs,
		}
		dimensions := make(map[string]string, len(filmContinuityDimensions))
		for _, dimension := range filmContinuityDimensions {
			dimensions[dimension] = "UNVERIFIED"
		}
		referenceLock := map[string]any{
			"sourceImageAttemptId":  slot.SourceImageAttemptID,
			"sourceImageResultId":   slot.SourceImageResultID,
			"sourceImageResourceId": slot.SourceImageResourceID,
			"sourceImageArtifactId": slot.SourceImageArtifactID,
			"sourceImageRevisionId": slot.SourceImageRevisionID,
		}
		readInJSON, writeOutJSON, dimensionJSON, referenceLockJSON, err := marshalFilmContinuityState(readIn, writeOut, dimensions, referenceLock)
		if err != nil {
			return nil, nil, nil, nil, nil, err
		}
		state := model.FilmContinuityShotState{
			ID: newID(), UserID: sequence.UserID, ProjectID: sequence.ProjectID, RootRunID: sequence.RootRunID,
			SequenceID: sequence.ID, LedgerID: ledgerID, ShotID: slot.ShotID, Position: slot.Position,
			ReadInJSON: readInJSON, WriteOutJSON: writeOutJSON, DimensionStateJSON: dimensionJSON,
			ReferenceLockJSON: referenceLockJSON, Status: model.FilmContinuityShotStatusNeedsReview, CreatedAt: at,
		}
		states = append(states, state)
		issue := model.FilmContinuityIssue{
			ID: newID(), UserID: sequence.UserID, ProjectID: sequence.ProjectID, RootRunID: sequence.RootRunID,
			SequenceID: sequence.ID, LedgerID: ledgerID, ShotID: slot.ShotID, Dimension: "cross_media",
			Severity: "warning", Code: "SEMANTIC_CONTINUITY_UNVERIFIED",
			Message:   "结构化链路与 Reference Lock 已建立；人物、服装、道具、场景、轴线、声音和 UI 连续性仍需基于真实媒体复核。",
			Authority: "continuity-check@3.0", Owner: "quality_control_editor", RepairStatus: "open",
			SourceRefsJSON: mustFilmJSON([]map[string]any{
				{"artifactId": sequence.PromptArtifactID, "revisionId": sequence.PromptArtifactRevisionID, "type": "ai-video-prompts"},
				{"artifactId": slot.SourceImageArtifactID, "revisionId": slot.SourceImageRevisionID, "type": "generation-result"},
			}), CreatedAt: at,
		}
		issues = append(issues, issue)
		stateContent = append(stateContent, map[string]any{
			"stateId": state.ID, "shotId": state.ShotID, "position": state.Position, "status": state.Status,
			"readIn": readIn, "writeOut": writeOut, "dimensions": dimensions, "referenceLock": referenceLock,
		})
		issueContent = append(issueContent, map[string]any{
			"issueId": issue.ID, "shotId": issue.ShotID, "dimension": issue.Dimension, "severity": issue.Severity,
			"code": issue.Code, "message": issue.Message, "authority": issue.Authority, "owner": issue.Owner,
			"repairStatus": issue.RepairStatus,
		})
		sourceRefs = append(sourceRefs, map[string]any{
			"artifactId": slot.SourceImageArtifactID, "revisionId": slot.SourceImageRevisionID, "type": "generation-result",
		})
	}

	ledger := &model.FilmContinuityLedger{
		ID: ledgerID, UserID: sequence.UserID, ProjectID: sequence.ProjectID, RootRunID: sequence.RootRunID,
		SequenceID: sequence.ID, MediaState: model.FilmContinuityMediaStateStructuredOnly,
		Status: model.FilmContinuityLedgerStatusNeedsYou, IssueCount: len(issues), SourceArtifactRefs: mustFilmJSON(sourceRefs),
		ArtifactID: artifactID, ArtifactRevisionID: revisionID, CreatedAt: at,
	}
	content, err := json.Marshal(map[string]any{
		"schemaVersion": 1, "artifactType": "continuity-ledger", "ledgerId": ledger.ID,
		"sequenceId": sequence.ID, "mediaState": ledger.MediaState, "status": ledger.Status,
		"dimensions": filmContinuityDimensions, "shots": stateContent, "issues": issueContent,
	})
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	artifact := &model.ProductionArtifact{
		ID: artifactID, UserID: sequence.UserID, ProjectID: sequence.ProjectID, Domain: "film",
		ArtifactType: "continuity-ledger", LogicalKey: "film-video:continuity:" + sequence.ID,
	}
	revision := &model.ProductionArtifactRevision{
		ID: revisionID, Status: model.ProductionArtifactStatusLocked, ContentJSON: string(content), ContentDigest: digestBytesHex(content),
		SourceRunID: sequence.RootRunID, SourceArtifactRefsJSON: ledger.SourceArtifactRefs,
		AuthorityRefsJSON: mustFilmJSON([]map[string]any{
			{"kind": "registry", "id": sequence.RegistryID, "version": sequence.RegistryVersion, "digest": sequence.RegistryDigest},
			{"kind": "skill", "id": "continuity-check", "version": "3.0.0"},
		}),
		CreatedByType: "runtime", CreatedByID: filmContinuityCompilerID, CreatedAt: at,
	}
	return ledger, states, issues, artifact, revision, nil
}

func rebuildFilmContinuityLedgerForSequenceUpdate(
	sequence model.FilmVideoSequence,
	slots []model.FilmVideoSlot,
	shots []model.Shot,
	existing repository.FilmContinuityLedgerDetail,
	at time.Time,
) (*model.FilmContinuityLedger, []model.FilmContinuityShotState, []model.FilmContinuityIssue, *model.ProductionArtifact, *model.ProductionArtifactRevision, error) {
	ledger, states, issues, artifact, revision, err := buildFilmContinuityLedger(sequence, slots, shots, at)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	var content filmContinuityArtifactContent
	if err := json.Unmarshal([]byte(revision.ContentJSON), &content); err != nil {
		return nil, nil, nil, nil, nil, conflictError("Continuity Ledger revision 内容已损坏，不能直接编辑当前视频序列")
	}
	content.LedgerID = existing.Ledger.ID
	stateIDsByShot := make(map[string]string, len(existing.Shots))
	for _, state := range existing.Shots {
		stateIDsByShot[state.ShotID] = state.ID
	}
	issueIDsByShot := make(map[string][]string, len(existing.Issues))
	for _, issue := range existing.Issues {
		issueIDsByShot[issue.ShotID] = append(issueIDsByShot[issue.ShotID], issue.ID)
	}
	if len(stateIDsByShot) != len(existing.Shots) || len(content.Shots) != len(states) || len(content.Issues) != len(issues) {
		return nil, nil, nil, nil, nil, conflictError("Continuity Ledger revision 镜头集合已变化，不能直接编辑当前视频序列")
	}
	for index := range states {
		if content.Shots[index].ShotID != states[index].ShotID {
			return nil, nil, nil, nil, nil, conflictError("Continuity Ledger revision 镜头顺序已变化，不能直接编辑当前视频序列")
		}
		oldID, ok := stateIDsByShot[states[index].ShotID]
		if !ok {
			return nil, nil, nil, nil, nil, conflictError("Continuity Ledger 缺少待编辑镜头状态")
		}
		content.Shots[index].StateID = oldID
		states[index].ID = oldID
		states[index].LedgerID = existing.Ledger.ID
	}
	for index := range issues {
		if content.Issues[index].ShotID != issues[index].ShotID {
			return nil, nil, nil, nil, nil, conflictError("Continuity Ledger revision 问题顺序已变化，不能直接编辑当前视频序列")
		}
		ids := issueIDsByShot[issues[index].ShotID]
		if len(ids) == 0 {
			return nil, nil, nil, nil, nil, conflictError("Continuity Ledger 缺少待编辑镜头问题")
		}
		oldID := ids[0]
		issueIDsByShot[issues[index].ShotID] = ids[1:]
		content.Issues[index].IssueID = oldID
		issues[index].ID = oldID
		issues[index].LedgerID = existing.Ledger.ID
	}
	for _, ids := range issueIDsByShot {
		if len(ids) > 0 {
			return nil, nil, nil, nil, nil, conflictError("Continuity Ledger 缺少待编辑镜头问题")
		}
	}
	encoded, err := json.Marshal(content)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	revision.ContentJSON = string(encoded)
	revision.ContentDigest = digestBytesHex(encoded)
	ledger.ID = existing.Ledger.ID
	ledger.ArtifactID = existing.Ledger.ArtifactID
	ledger.ArtifactRevisionID = revision.ID
	ledger.CreatedAt = existing.Ledger.CreatedAt
	artifact.ID = existing.Ledger.ArtifactID
	return ledger, states, issues, artifact, revision, nil
}

func marshalFilmContinuityState(readIn map[string]any, writeOut map[string]any, dimensions map[string]string, referenceLock map[string]any) (string, string, string, string, error) {
	values := []any{readIn, writeOut, dimensions, referenceLock}
	encoded := make([]string, 0, len(values))
	for _, value := range values {
		item, err := json.Marshal(value)
		if err != nil {
			return "", "", "", "", err
		}
		encoded = append(encoded, string(item))
	}
	return encoded[0], encoded[1], encoded[2], encoded[3], nil
}

func filmContinuityLedgerView(detail repository.FilmContinuityLedgerDetail) (FilmContinuityLedgerView, error) {
	shots := make([]FilmContinuityShotStateView, 0, len(detail.Shots))
	for _, state := range detail.Shots {
		readIn := map[string]any{}
		writeOut := map[string]any{}
		dimensions := map[string]string{}
		referenceLock := map[string]any{}
		if json.Unmarshal([]byte(state.ReadInJSON), &readIn) != nil || json.Unmarshal([]byte(state.WriteOutJSON), &writeOut) != nil ||
			json.Unmarshal([]byte(state.DimensionStateJSON), &dimensions) != nil || json.Unmarshal([]byte(state.ReferenceLockJSON), &referenceLock) != nil {
			return FilmContinuityLedgerView{}, fmt.Errorf("continuity Ledger 镜头状态已损坏")
		}
		shots = append(shots, FilmContinuityShotStateView{
			ID: state.ID, ShotID: state.ShotID, Position: state.Position, ReadIn: readIn, WriteOut: writeOut,
			Dimensions: dimensions, ReferenceLock: referenceLock, Status: state.Status,
		})
	}
	issues := make([]FilmContinuityIssueView, 0, len(detail.Issues))
	for _, issue := range detail.Issues {
		sourceRefs := []map[string]any{}
		if json.Unmarshal([]byte(firstNonEmpty(issue.SourceRefsJSON, "[]")), &sourceRefs) != nil {
			return FilmContinuityLedgerView{}, fmt.Errorf("continuity Ledger 问题来源已损坏")
		}
		issues = append(issues, FilmContinuityIssueView{
			ID: issue.ID, ShotID: issue.ShotID, Dimension: issue.Dimension, Severity: issue.Severity,
			Code: issue.Code, Message: issue.Message, Authority: issue.Authority, Owner: issue.Owner,
			RepairStatus: issue.RepairStatus, SourceRefs: sourceRefs, CreatedAt: issue.CreatedAt,
		})
	}
	return FilmContinuityLedgerView{Ledger: detail.Ledger, Shots: shots, Issues: issues}, nil
}

func filmReworkEventView(event model.FilmReworkEvent) (FilmReworkEventView, error) {
	repairScope := map[string]any{}
	recheckGate := map[string]any{}
	if json.Unmarshal([]byte(firstNonEmpty(event.RepairScopeJSON, "{}")), &repairScope) != nil ||
		json.Unmarshal([]byte(firstNonEmpty(event.RecheckGateJSON, "{}")), &recheckGate) != nil {
		return FilmReworkEventView{}, fmt.Errorf("Film Rework Event 已损坏")
	}
	return FilmReworkEventView{FilmReworkEvent: event, RepairScope: repairScope, RecheckGate: recheckGate}, nil
}
