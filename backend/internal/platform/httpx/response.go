package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

const requestIDKey = "request_id"

type envelope struct {
	Data any `json:"data"`
	Meta any `json:"meta,omitempty"`
}

type errorEnvelope struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code      string            `json:"code"`
	Message   string            `json:"message"`
	Details   map[string]string `json:"details,omitempty"`
	RequestID string            `json:"request_id"`
}

func OK(c *gin.Context, data any) {
	c.JSON(http.StatusOK, envelope{Data: data, Meta: requestMeta(c, nil)})
}

func Created(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, envelope{Data: data, Meta: requestMeta(c, nil)})
}

func Accepted(c *gin.Context, data any) {
	c.JSON(http.StatusAccepted, envelope{Data: data, Meta: requestMeta(c, nil)})
}

func NoContent(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

// List renders a collection with pagination metadata merged into meta.
func List(c *gin.Context, items any, meta map[string]any) {
	c.JSON(http.StatusOK, envelope{Data: items, Meta: requestMeta(c, meta)})
}

func requestMeta(c *gin.Context, extra map[string]any) map[string]any {
	meta := make(map[string]any, len(extra)+1)
	for k, v := range extra {
		meta[k] = v
	}
	meta["request_id"] = RequestID(c)
	return meta
}

// Fail maps any error to the error envelope. Unknown errors become 500 and are
// logged with their cause; the client only sees a generic message.
func Fail(c *gin.Context, err error) {
	var appErr *Error
	if !errors.As(err, &appErr) {
		appErr = Internal(err)
	}

	if appErr.Status >= http.StatusInternalServerError {
		slog.ErrorContext(c.Request.Context(), "request failed",
			"code", appErr.Code,
			"error", appErr.Error(),
			"request_id", RequestID(c),
			"method", c.Request.Method,
			"path", c.FullPath(),
		)
	}

	c.AbortWithStatusJSON(appErr.Status, errorEnvelope{Error: errorBody{
		Code:      appErr.Code,
		Message:   appErr.Message,
		Details:   appErr.Details,
		RequestID: RequestID(c),
	}})
}

// RequestID returns the per-request correlation ID set by the middleware.
func RequestID(c *gin.Context) string {
	if v, ok := c.Get(requestIDKey); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// BindJSON decodes the request body or returns a 422 error.
func BindJSON(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		return Validation(map[string]string{"body": "request body is missing or is not valid JSON"})
	}
	return nil
}

// ClientMeta is the request context copied into audit records.
type ClientMeta struct {
	IP        string
	UserAgent string
	RequestID string
}

func ClientMetaFrom(c *gin.Context) ClientMeta {
	return ClientMeta{
		IP:        c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
		RequestID: RequestID(c),
	}
}
