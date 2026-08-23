package service

import (
	"errors"
	"log"
	"time"

	"infinite-canvas/backend/internal/repository"
)

// startEcommerceAgentWorker runs the provider-free Ecommerce runtime queue.
// Claiming is still fenced in the repository, so a second server instance can
// safely compete for work and recover an expired lease without duplicating an
// Attempt or a ProductDNA revision.
func (s *Service) startEcommerceAgentWorker() {
	if s == nil || s.ecommerceAgentRegistry == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			processed, err := s.ProcessNextEcommerceAgentStep()
			if err != nil && !errors.Is(err, repository.ErrAgentRuntimeLeaseLost) && !errors.Is(err, repository.ErrAgentRuntimeStateConflict) {
				log.Printf("process Ecommerce Agent execution failed: %v", err)
			}
			if processed {
				continue
			}
			<-ticker.C
		}
	}()
}
