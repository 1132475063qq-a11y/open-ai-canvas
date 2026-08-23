package service

import (
	"strings"

	"infinite-canvas/backend/internal/model"
)

// ListEcommerceAgentRuns returns only the caller's Ecommerce-domain durable
// Runs. The domain predicate is deliberate: Film and Ecommerce share storage
// tables but never share a user-facing run list.
func (s *Service) ListEcommerceAgentRuns(userID string, projectID string, limit int) ([]model.AgentRuntimeRun, error) {
	if _, err := s.ecommerceProjectForRead(userID, projectID); err != nil {
		return nil, err
	}
	return s.repo.ProjectAgentRuntimeRunsForDomain(userID, strings.TrimSpace(projectID), "ecommerce", limit)
}
