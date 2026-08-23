package handler

import (
	"net/http"
	"strconv"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

const ecommerceRequestBodyLimit = 2 << 20
const ecommerceEvaluationRequestBodyLimit = 1 << 20

func RegisterProjectEcommerceRoutes(r *gin.RouterGroup, svc *service.Service) {
	r.GET("/projects/:id/ecommerce/workspace", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		workspace, err := svc.ProjectEcommerceWorkspace(user.ID, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"workspace": workspace})
	})

	r.GET("/projects/:id/ecommerce/presets", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		presets, err := svc.ProjectEcommercePresets(user.ID, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"presets": presets})
	})

	r.POST("/projects/:id/ecommerce/presets", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ecommerceRequestBodyLimit)
		var req service.SaveEcommercePresetRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		preset, err := svc.SaveProjectEcommercePreset(user.ID, c.Param("id"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"preset": preset})
	})

	r.POST("/projects/:id/ecommerce/runs", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ecommerceRequestBodyLimit)
		var req service.CreateEcommerceRunRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		run, err := svc.CreateProjectEcommerceRun(user.ID, c.Param("id"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"run": run})
	})

	r.GET("/projects/:id/ecommerce/runs/:runId", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		run, err := svc.ProjectEcommerceRun(user.ID, c.Param("id"), c.Param("runId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"run": run})
	})

	r.POST("/projects/:id/ecommerce/runs/:runId/quote", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		var req service.RefreshEcommerceQuoteRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		run, err := svc.RefreshProjectEcommerceRunQuote(user.ID, c.Param("id"), c.Param("runId"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"run": run})
	})

	r.POST("/projects/:id/ecommerce/runs/:runId/approve", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		run, err := svc.ApproveProjectEcommerceRun(user.ID, c.Param("id"), c.Param("runId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"run": run})
	})

	r.POST("/projects/:id/ecommerce/runs/:runId/submit", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		var req service.SubmitEcommerceRunRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		run, err := svc.SubmitProjectEcommerceRun(user.ID, c.Param("id"), c.Param("runId"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"run": run})
	})

	r.POST("/projects/:id/ecommerce/runs/:runId/cancel", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		run, err := svc.CancelProjectEcommerceRun(c.Request.Context(), user.ID, c.Param("id"), c.Param("runId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"run": run})
	})

	r.POST("/projects/:id/ecommerce/runs/:runId/slots/:slotId/retry", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ecommerceRequestBodyLimit)
		var req service.RetryEcommerceSlotRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		retry, err := svc.RetryProjectEcommerceSlot(user.ID, c.Param("id"), c.Param("runId"), c.Param("slotId"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"retry": retry})
	})

	r.POST("/projects/:id/ecommerce/runs/:runId/slots/:slotId/qa", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10)
		var req service.ReviewEcommerceSlotRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		run, err := svc.ReviewProjectEcommerceSlot(user.ID, c.Param("id"), c.Param("runId"), c.Param("slotId"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"run": run})
	})

	r.POST("/projects/:id/ecommerce/runs/:runId/video-sequences", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 256<<10)
		var req service.CreateEcommerceVideoSequenceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		run, err := svc.CreateProjectEcommerceVideoSequence(user.ID, c.Param("id"), c.Param("runId"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"run": run})
	})

	r.GET("/projects/:id/ecommerce/provider-evaluations", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
		plans, err := svc.ProjectEcommerceProviderEvaluationPlans(user.ID, c.Param("id"), limit)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"plans": plans})
	})

	r.POST("/projects/:id/ecommerce/provider-evaluations", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ecommerceEvaluationRequestBodyLimit)
		var req service.CreateEcommerceProviderEvaluationPlanRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		if req.IdempotencyKey == "" {
			req.IdempotencyKey = c.GetHeader("X-Idempotency-Key")
		}
		result, err := svc.CreateProjectEcommerceProviderEvaluationPlan(user.ID, c.Param("id"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})

	r.GET("/projects/:id/ecommerce/provider-evaluations/:planId", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		plan, err := svc.ProjectEcommerceProviderEvaluationPlan(user.ID, c.Param("id"), c.Param("planId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, plan)
	})

	r.POST("/projects/:id/ecommerce/provider-evaluations/:planId/attempts", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ecommerceEvaluationRequestBodyLimit)
		var req service.RecordEcommerceProviderEvaluationAttemptRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		if req.IdempotencyKey == "" {
			req.IdempotencyKey = c.GetHeader("X-Idempotency-Key")
		}
		plan, idempotent, err := svc.RecordProjectEcommerceProviderEvaluationAttempt(user.ID, c.Param("id"), c.Param("planId"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"plan": plan, "idempotent": idempotent})
	})

	r.POST("/projects/:id/ecommerce/provider-evaluations/:planId/scores", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ecommerceEvaluationRequestBodyLimit)
		var req service.RecordEcommerceProviderEvaluationScoreRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		if req.IdempotencyKey == "" {
			req.IdempotencyKey = c.GetHeader("X-Idempotency-Key")
		}
		plan, idempotent, err := svc.RecordProjectEcommerceProviderEvaluationScore(user.ID, c.Param("id"), c.Param("planId"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"plan": plan, "idempotent": idempotent})
	})

	r.POST("/projects/:id/ecommerce/provider-evaluations/:planId/blind-preferences", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ecommerceEvaluationRequestBodyLimit)
		var req service.RecordEcommerceProviderEvaluationPreferenceRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		if req.IdempotencyKey == "" {
			req.IdempotencyKey = c.GetHeader("X-Idempotency-Key")
		}
		plan, idempotent, err := svc.RecordProjectEcommerceProviderEvaluationPreference(user.ID, c.Param("id"), c.Param("planId"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"plan": plan, "idempotent": idempotent})
	})

	r.POST("/projects/:id/ecommerce/provider-evaluations/:planId/decision", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 64<<10)
		var req service.RecordEcommerceProviderEvaluationDecisionRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		plan, err := svc.RecordProjectEcommerceProviderEvaluationDecision(user.ID, c.Param("id"), c.Param("planId"), req)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, plan)
	})
}
