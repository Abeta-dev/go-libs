// SPDX-License-Identifier: MIT

package ginmw

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Envelope defines the standardized envelope format for API responses.
type Envelope[T any] struct {
	Success bool      `json:"success"`
	Data    T         `json:"data,omitempty"`
	Error   *APIError `json:"error,omitempty"`
}

// Response is a convenience alias for untyped or dynamic envelopes.
type Response = Envelope[any]

// APIError represents structured error information in API response envelopes.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

// RespondSuccess sends an HTTP 200 with the success envelope.
func RespondSuccess(c *gin.Context, data any) {
	c.JSON(http.StatusOK, Response{
		Success: true,
		Data:    data,
	})
}

// RespondCreated sends an HTTP 201 with the success envelope.
func RespondCreated(c *gin.Context, data any) {
	c.JSON(http.StatusCreated, Response{
		Success: true,
		Data:    data,
	})
}

// RespondError sends the specified HTTP status code with a structured error envelope.
func RespondError(c *gin.Context, status int, code, message string) {
	c.AbortWithStatusJSON(status, Response{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
		},
	})
}

// RespondErrorWithDetail sends an error envelope with additional debugging detail.
func RespondErrorWithDetail(c *gin.Context, status int, code, message, detail string) {
	c.AbortWithStatusJSON(status, Response{
		Success: false,
		Error: &APIError{
			Code:    code,
			Message: message,
			Detail:  detail,
		},
	})
}
