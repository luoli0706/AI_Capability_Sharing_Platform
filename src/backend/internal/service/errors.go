package service

import (
	"errors"
	"net/http"

	"ascp-backend/internal/pkg/response"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

// AppError represents a domain/business error with an HTTP status and business code.
type AppError struct {
	HTTPStatus int
	Code       int
	Message    string
}

func (e *AppError) Error() string {
	return e.Message
}

func NewBadRequest(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusBadRequest, Code: response.CodeBadRequest, Message: msg}
}

func NewUnauthorized(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusUnauthorized, Code: response.CodeUnauthorized, Message: msg}
}

func NewForbidden(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusForbidden, Code: response.CodeForbidden, Message: msg}
}

func NewNotFound(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusNotFound, Code: response.CodeNotFound, Message: msg}
}

func NewConflict(msg string) *AppError {
	return &AppError{HTTPStatus: http.StatusConflict, Code: response.CodeConflict, Message: msg}
}

// HandleError maps service errors to unified JSON responses.
func HandleError(c echo.Context, err error) error {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return response.Error(c, appErr.HTTPStatus, appErr.Code, appErr.Message)
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return response.NotFound(c, "resource not found")
	}
	return response.InternalError(c, "internal server error")
}
