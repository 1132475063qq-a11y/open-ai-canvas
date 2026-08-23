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
	filmProductionQuoteRequestLimit = 256 << 10
	filmProductionQCRequestLimit    = 64 << 10
)

func RegisterFilmProductionRoutes(r *gin.RouterGroup, svc *service.Service) {
	production := r.Group("/projects/:id/film/production")
	production.POST("/visual-qc-quotes", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQuoteRequestLimit)
		var request service.CreateFilmVisualQCQuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		quote, err := svc.CreateFilmVisualQCQuote(user.ID, c.Param("id"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, quote)
	})
	production.POST("/visual-qc-quotes/:quoteId/submit", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQCRequestLimit)
		var request service.SubmitFilmVisualQCQuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		result, err := svc.SubmitFilmVisualQCQuote(
			user.ID, c.Param("id"), c.Param("quoteId"), c.GetHeader("X-Idempotency-Key"), request,
		)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	production.POST("/video-visual-qc-quotes", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQuoteRequestLimit)
		var request service.CreateFilmVideoVisualQCQuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		quote, err := svc.CreateFilmVideoVisualQCQuote(user.ID, c.Param("id"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, quote)
	})
	production.POST("/video-visual-qc-quotes/:quoteId/submit", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQCRequestLimit)
		var request service.SubmitFilmVideoVisualQCQuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		result, err := svc.SubmitFilmVideoVisualQCQuote(user.ID, c.Param("id"), c.Param("quoteId"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	production.POST("/video-sequence-visual-qc-quotes", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQuoteRequestLimit)
		var request service.CreateFilmVideoSequenceVisualQCQuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		quote, err := svc.CreateFilmVideoSequenceVisualQCQuote(user.ID, c.Param("id"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, quote)
	})
	production.POST("/video-sequence-visual-qc-quotes/:quoteId/submit", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQCRequestLimit)
		var request service.SubmitFilmVideoSequenceVisualQCQuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		result, err := svc.SubmitFilmVideoSequenceVisualQCQuote(user.ID, c.Param("id"), c.Param("quoteId"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	production.POST("/video-sequences", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQuoteRequestLimit)
		var request service.CreateFilmVideoSequenceRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		sequence, err := svc.CreateFilmVideoSequence(user.ID, c.Param("id"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, sequence)
	})
	production.GET("/video-sequences", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		limit, err := filmVideoSequenceListLimit(c.Query("limit"))
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		sequences, err := svc.ListFilmVideoSequences(user.ID, c.Param("id"), c.Query("rootRunId"), limit)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"sequences": sequences})
	})
	production.PATCH("/video-sequences/:sequenceId", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQuoteRequestLimit)
		var request service.UpdateFilmVideoSequenceRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		sequence, err := svc.UpdateFilmVideoSequence(user.ID, c.Param("id"), c.Param("sequenceId"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, sequence)
	})
	production.POST("/video-sequences/:sequenceId/review", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQCRequestLimit)
		var request service.CreateFilmVideoSequenceReviewRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		result, err := svc.CreateFilmVideoSequenceReview(user.ID, c.Param("id"), c.Param("sequenceId"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	production.POST("/video-quotes", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQuoteRequestLimit)
		var request service.CreateFilmVideoQuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		quote, err := svc.CreateFilmVideoQuote(user.ID, c.Param("id"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, quote)
	})
	production.POST("/video-quotes/:quoteId/submit", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQCRequestLimit)
		var request service.SubmitFilmVideoQuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		result, err := svc.SubmitFilmVideoQuote(user.ID, c.Param("id"), c.Param("quoteId"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	production.POST("/video-attempts/:attemptId/qc", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQCRequestLimit)
		var request service.CreateFilmProductionQCRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		result, err := svc.CreateFilmVideoHumanQC(
			user.ID, c.Param("id"), c.Param("attemptId"), c.GetHeader("X-Idempotency-Key"), request,
		)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	production.POST("/image-quotes", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQuoteRequestLimit)
		var request service.CreateFilmProductionImageQuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		quote, err := svc.CreateFilmProductionImageQuote(user.ID, c.Param("id"), c.GetHeader("X-Idempotency-Key"), request)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, quote)
	})
	production.POST("/image-quotes/:quoteId/submit", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQCRequestLimit)
		var request service.SubmitFilmProductionImageQuoteRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		result, err := svc.SubmitFilmProductionImageQuote(
			user.ID, c.Param("id"), c.Param("quoteId"), c.GetHeader("X-Idempotency-Key"), request,
		)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
	production.GET("/attempts", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		limit, err := filmProductionAttemptListLimit(c.Query("limit"))
		if err != nil {
			fail(c, http.StatusBadRequest, err)
			return
		}
		attempts, err := svc.ListFilmProductionAttempts(
			user.ID, c.Param("id"), c.Query("rootRunId"), c.Query("shotId"), limit,
		)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, gin.H{"attempts": attempts})
	})
	production.POST("/attempts/:attemptId/qc", func(c *gin.Context) {
		user, err := currentUser(c, svc)
		if err != nil {
			failService(c, err)
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, filmProductionQCRequestLimit)
		var request service.CreateFilmProductionQCRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			failFilmProductionBind(c, err)
			return
		}
		result, err := svc.CreateFilmProductionHumanQC(
			user.ID, c.Param("id"), c.Param("attemptId"), c.GetHeader("X-Idempotency-Key"), request,
		)
		if err != nil {
			failService(c, err)
			return
		}
		ok(c, result)
	})
}

func filmProductionAttemptListLimit(raw string) (int, error) {
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

func filmVideoSequenceListLimit(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 20, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 || limit > 100 {
		return 0, errors.New("limit 必须是 1-100 的整数")
	}
	return limit, nil
}

func failFilmProductionBind(c *gin.Context, err error) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		fail(c, http.StatusRequestEntityTooLarge, errors.New("Film 制作请求体过大"))
		return
	}
	fail(c, http.StatusBadRequest, err)
}
