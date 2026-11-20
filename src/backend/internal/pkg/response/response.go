package response

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

const (
	CodeSuccess      = 0
	CodeBadRequest   = 40001
	CodeUnauthorized = 40101
	CodeForbidden    = 40301
	CodeNotFound     = 40401
	CodeConflict     = 40901
	CodeInternal     = 50001
)

type Result struct {
	Code      int         `json:"code"`
	Message   string      `json:"message"`
	Data      interface{} `json:"data,omitempty"`
	RequestID string      `json:"request_id,omitempty"`
}

func Success(c echo.Context, data interface{}) error {
	reqID := c.Response().Header().Get(echo.HeaderXRequestID)
	return c.JSON(http.StatusOK, Result{
		Code:      CodeSuccess,
		Message:   "success",
		Data:      data,
		RequestID: reqID,
	})
}

func Error(c echo.Context, httpStatus int, code int, message string) error {
	reqID := c.Response().Header().Get(echo.HeaderXRequestID)
	return c.JSON(httpStatus, Result{
		Code:      code,
		Message:   message,
		RequestID: reqID,
	})
}

func BadRequest(c echo.Context, message string) error {
	return Error(c, http.StatusBadRequest, CodeBadRequest, message)
}

func Unauthorized(c echo.Context, message string) error {
	return Error(c, http.StatusUnauthorized, CodeUnauthorized, message)
}

func Forbidden(c echo.Context, message string) error {
	return Error(c, http.StatusForbidden, CodeForbidden, message)
}

func NotFound(c echo.Context, message string) error {
	return Error(c, http.StatusNotFound, CodeNotFound, message)
}

func Conflict(c echo.Context, message string) error {
	return Error(c, http.StatusConflict, CodeConflict, message)
}

func InternalError(c echo.Context, message string) error {
	return Error(c, http.StatusInternalServerError, CodeInternal, message)
}
