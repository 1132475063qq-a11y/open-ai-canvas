package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

const (
	filmAgentRunRequestLimit      = 256 << 10
	filmAgentDecisionRequestLimit = 64 << 10
)

func RegisterFilmAgentRuntimeRoutes(r *gin.RouterGroup, svc *service.Service) {
	film := r.Group("/projects/:id/film")
	film.GET("/agent-runtime/catalog", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		catalog, err := svc.FilmAgentRuntimeCatalog(user.ID, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, catalog)
	})
	film.GET("/agent-runs", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		limit, err := filmAgentRunListLimit(c.Query("limit"))
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		runs, err := svc.ListFilmAgentRuns(user.ID, c.Param("id"), limit)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"runs": runs})
	})
	film.POST("/agent-runs", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmAgentRunRequestLimit)
		var request service.CreateFilmAgentRunRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmAgentBind(c, err)
			return
		}
		result, err := svc.CreateFilmAgentRun(user.ID, c.Param("id"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	film.GET("/agent-runs/:runId", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		detail, err := svc.FilmAgentRunDetail(user.ID, c.Param("id"), c.Param("runId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, detail)
	})
	film.POST("/agent-runs/:runId/decisions/:decisionId/resolve", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmAgentDecisionRequestLimit)
		var request service.ResolveFilmAgentDecisionRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmAgentBind(c, err)
			return
		}
		detail, err := svc.ResolveFilmAgentDecision(user.ID, c.Param("id"), c.Param("runId"), c.Param("decisionId"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, detail)
	})
	film.POST("/agent-runs/:runId/steps/:stepId/retry", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmAgentDecisionRequestLimit)
		var request service.RetryFilmAgentStepRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmAgentBind(c, err)
			return
		}
		detail, err := svc.RetryFilmAgentStep(user.ID, c.Param("id"), c.Param("runId"), c.Param("stepId"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, detail)
	})
	film.POST("/agent-runs/:runId/artifacts/:artifactId/lock", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmAgentDecisionRequestLimit)
		var request service.LockFilmAgentArtifactRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmAgentBind(c, err)
			return
		}
		result, err := svc.LockFilmAgentArtifact(user.ID, c.Param("id"), c.Param("runId"), c.Param("artifactId"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	film.POST("/agent-runs/:runId/artifacts/:artifactId/rollback", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmAgentDecisionRequestLimit)
		var request service.RollbackFilmAgentArtifactRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmAgentBind(c, err)
			return
		}
		result, err := svc.RollbackFilmAgentArtifact(user.ID, c.Param("id"), c.Param("runId"), c.Param("artifactId"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	film.GET("/agent-runs/:runId/closeout", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		preview, err := svc.PreviewFilmAgentCloseout(user.ID, c.Param("id"), c.Param("runId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, preview)
	})
	film.POST("/agent-runs/:runId/closeout", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmAgentDecisionRequestLimit)
		var request service.ConfirmFilmAgentCloseoutRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmAgentBind(c, err)
			return
		}
		result, err := svc.ConfirmFilmAgentCloseout(user.ID, c.Param("id"), c.Param("runId"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
}

func filmAgentRunListLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 50, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > 100 {
		return 0, errors.New("limit 必须是 1-100 的整数")
	}
	return limit, nil
}

func failFilmAgentBind(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		fail(c, http.StatusRequestEntityTooLarge, errors.New("Film Agent 请求体过大"))
		return
	}
	fail(c, http.StatusBadRequest, err)
}
