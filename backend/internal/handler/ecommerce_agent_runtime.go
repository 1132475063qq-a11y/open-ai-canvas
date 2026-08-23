package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"infinite-canvas/backend/internal/service"

	"github.com/gin-gonic/gin"
)

const ecommerceAgentRunRequestLimit = 256 << 10

// RegisterProjectEcommerceAgentRuntimeRoutes exposes only the provider-neutral
// control-plane surface. Worker claiming stays server-side; clients create or
// inspect a durable Run and never receive a way to bypass quote/billing gates.
func RegisterProjectEcommerceAgentRuntimeRoutes(r *gin.RouterGroup, svc *service.Service) {
	ecommerce := r.Group("/projects/:id/ecommerce")
	ecommerce.GET("/agent-runtime/catalog", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		catalog, err := svc.EcommerceAgentRuntimeCatalog(user.ID, c.Param("id"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, catalog)
	})
	ecommerce.GET("/agent-runs", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		limit := 50
		if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
			limit, err = strconv.Atoi(raw)
			if err != nil || limit < 1 || limit > 100 {
				fail(c, http.StatusBadRequest, errors.New("limit 必须是 1-100 的整数"))
				return
			}
		}
		runs, err := svc.ListEcommerceAgentRuns(user.ID, c.Param("id"), limit)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"runs": runs})
	})
	ecommerce.POST("/agent-runs", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, ecommerceAgentRunRequestLimit)
		var request service.CreateEcommerceAgentRunRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failEcommerceAgentBind(c, err)
			return
		}
		result, err := svc.CreateEcommerceAgentRun(user.ID, c.Param("id"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	ecommerce.GET("/agent-runs/:runId", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		detail, err := svc.EcommerceAgentRunDetail(user.ID, c.Param("id"), c.Param("runId"))
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, detail)
	})
}

func failEcommerceAgentBind(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		fail(c, http.StatusRequestEntityTooLarge, errors.New("Ecommerce Agent 请求体过大"))
		return
	}
	fail(c, http.StatusBadRequest, err)
}
