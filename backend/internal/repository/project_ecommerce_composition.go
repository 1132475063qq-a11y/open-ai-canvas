package repository

import (
	"encoding/json"
	"fmt"
	"math/bits"
	"strconv"
	"strings"
	"time"

	"infinite-canvas/backend/internal/model"

	"gorm.io/gorm"
)

const (
	ecommerceCompositionHashAlgorithm  = "dhash128-v1"
	ecommerceCompositionDuplicateLimit = 12
	ecommerceAutoDuplicateNotePrefix   = "自动构图检查："
	ecommerceDuplicateCompositionIssue = "duplicate_composition"
)

type ecommerceCompositionFingerprint struct {
	hash      string
	algorithm string
	valid     bool
}

func ecommerceResultCompositionFingerprint(payloadJSON string) (string, string, bool) {
	var payload map[string]any
	if json.Unmarshal([]byte(payloadJSON), &payload) != nil {
		return "", "", false
	}
	images, _ := payload["images"].([]any)
	if len(images) == 0 {
		return "", "", false
	}
	image, _ := images[0].(map[string]any)
	if image == nil {
		return "", "", false
	}
	hash, _ := image["compositionHash"].(string)
	algorithm, _ := image["compositionHashAlgorithm"].(string)
	hash, algorithm = strings.TrimSpace(hash), strings.TrimSpace(algorithm)
	if algorithm != ecommerceCompositionHashAlgorithm || len(hash) != 32 {
		return "", "", false
	}
	if _, _, ok := ecommerceCompositionHashParts(hash); !ok {
		return "", "", false
	}
	return hash, algorithm, true
}

func ecommerceCompositionHashDistance(left string, right string) (int, bool) {
	leftHigh, leftLow, leftOK := ecommerceCompositionHashParts(left)
	rightHigh, rightLow, rightOK := ecommerceCompositionHashParts(right)
	if !leftOK || !rightOK {
		return 0, false
	}
	return bits.OnesCount64(leftHigh^rightHigh) + bits.OnesCount64(leftLow^rightLow), true
}

func ecommerceCompositionHashParts(value string) (uint64, uint64, bool) {
	if len(value) != 32 {
		return 0, 0, false
	}
	high, highErr := strconv.ParseUint(value[:16], 16, 64)
	low, lowErr := strconv.ParseUint(value[16:], 16, 64)
	return high, low, highErr == nil && lowErr == nil
}

// reconcileEcommerceCompositionQA is order-independent: if a later slot
// finishes first, the next completion still marks the higher-position image
// as the duplicate of the earliest canonical composition.
func reconcileEcommerceCompositionQA(tx *gorm.DB, runID string, now time.Time) error {
	var slots []model.EcommerceProductionSlot
	if err := tx.Where("run_id = ?", runID).Order("position asc").Find(&slots).Error; err != nil {
		return err
	}
	fingerprints := make([]ecommerceCompositionFingerprint, len(slots))
	for index := range slots {
		hash, algorithm, ok := ecommerceResultCompositionFingerprint(slots[index].ResultPayloadJSON)
		fingerprints[index] = ecommerceCompositionFingerprint{hash: hash, algorithm: algorithm, valid: ok}
	}

	canonical := make([]int, 0, len(slots))
	for index := range slots {
		fingerprint := fingerprints[index]
		if !fingerprint.valid {
			continue
		}
		duplicateIndex, distance := -1, 0
		for _, candidateIndex := range canonical {
			candidate := fingerprints[candidateIndex]
			if !candidate.valid || candidate.algorithm != fingerprint.algorithm {
				continue
			}
			candidateDistance, ok := ecommerceCompositionHashDistance(fingerprint.hash, candidate.hash)
			if !ok || candidateDistance > ecommerceCompositionDuplicateLimit {
				continue
			}
			if duplicateIndex < 0 || candidateDistance < distance {
				duplicateIndex, distance = candidateIndex, candidateDistance
			}
		}
		if duplicateIndex < 0 {
			canonical = append(canonical, index)
		}
		if err := updateEcommerceCompositionQA(tx, slots[index], fingerprint, duplicateSlot(slots, duplicateIndex), distance, now); err != nil {
			return err
		}
	}
	return nil
}

func duplicateSlot(slots []model.EcommerceProductionSlot, index int) *model.EcommerceProductionSlot {
	if index < 0 || index >= len(slots) {
		return nil
	}
	return &slots[index]
}

func updateEcommerceCompositionQA(tx *gorm.DB, slot model.EcommerceProductionSlot, fingerprint ecommerceCompositionFingerprint, duplicate *model.EcommerceProductionSlot, distance int, now time.Time) error {
	updates := map[string]any{
		"composition_hash": fingerprint.hash, "composition_hash_alg": fingerprint.algorithm, "updated_at": now,
	}
	if slot.Accepted {
		return tx.Model(&model.EcommerceProductionSlot{}).Where("id = ?", slot.ID).Updates(updates).Error
	}
	issues := ecommerceQAIssues(slot.QAIssuesJSON)
	previouslyAutomatic := slot.DuplicateOfSlotID != ""
	issues = removeEcommerceQAIssue(issues, ecommerceDuplicateCompositionIssue)
	note := removeEcommerceAutoDuplicateNote(slot.QANote)
	updates["duplicate_of_slot_id"] = ""
	updates["composition_distance"] = 0
	if duplicate != nil {
		issues = append(issues, ecommerceDuplicateCompositionIssue)
		updates["duplicate_of_slot_id"] = duplicate.ID
		updates["composition_distance"] = distance
		updates["qa_status"] = "FAIL"
		autoNote := fmt.Sprintf("%s与槽位 %d「%s」的整体机位和构图过于接近（指纹距离 %d/%d），请人工确认后按新机位单张重做。", ecommerceAutoDuplicateNotePrefix, duplicate.Position, duplicate.Title, distance, ecommerceCompositionDuplicateLimit)
		if note == "" {
			note = autoNote
		} else {
			note += "\n" + autoNote
		}
	} else if previouslyAutomatic {
		updates["qa_status"] = ecommerceAutomaticQAStatus(issues)
	}
	encoded, err := json.Marshal(uniqueEcommerceQAIssues(issues))
	if err != nil {
		return err
	}
	updates["qa_issues_json"] = string(encoded)
	updates["qa_note"] = note
	return tx.Model(&model.EcommerceProductionSlot{}).Where("id = ?", slot.ID).Updates(updates).Error
}

func ecommerceQAIssues(raw string) []string {
	var values []string
	if json.Unmarshal([]byte(raw), &values) != nil {
		return []string{}
	}
	return uniqueEcommerceQAIssues(values)
}

func uniqueEcommerceQAIssues(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func removeEcommerceQAIssue(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func removeEcommerceAutoDuplicateNote(note string) string {
	lines := strings.Split(strings.TrimSpace(note), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, ecommerceAutoDuplicateNotePrefix) {
			result = append(result, line)
		}
	}
	return strings.Join(result, "\n")
}

func ecommerceAutomaticQAStatus(issues []string) string {
	for _, issue := range issues {
		if issue != "visual_review_required" && issue != "resolution_unverified" {
			return "FAIL"
		}
	}
	return "UNCERTAIN"
}
