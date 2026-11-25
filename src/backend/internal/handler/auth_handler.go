package handler

import (
	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	authSvc  service.AuthService
	emailSvc service.EmailService
}

func NewAuthHandler(authSvc service.AuthService) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

func (h *AuthHandler) SetEmailService(emailSvc service.EmailService) {
	h.emailSvc = emailSvc
}

func (h *AuthHandler) Register(c echo.Context) error {
	var req model.RegisterRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	user, err := h.authSvc.Register(req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"user": user})
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req model.LoginRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	res, err := h.authSvc.LoginWithIP(req, c.RealIP())
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, res)
}

func (h *AuthHandler) SendRegisterEmailCode(c echo.Context) error {
	if h.emailSvc == nil {
		return response.Error(c, 503, 50301, "邮件验证服务未初始化")
	}
	var req model.SendEmailCodeRequest
	if !bindAndValidate(c, &req) {
		return nil
	}
	if err := h.emailSvc.SendPublicRegisterCode(c.Request().Context(), req.Email, c.RealIP()); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]string{"message": "验证码已发送"})
}

func (h *AuthHandler) SendBindEmailCode(c echo.Context) error {
	if h.emailSvc == nil {
		return response.Error(c, 503, 50301, "邮件验证服务未初始化")
	}
	var req model.SendEmailCodeRequest
	if !bindAndValidate(c, &req) {
		return nil
	}
	userID := middleware.GetUserID(c)
	if err := h.emailSvc.SendBindEmailCode(c.Request().Context(), userID, req.Email, c.RealIP()); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]string{"message": "验证码已发送"})
}

func (h *AuthHandler) BindEmail(c echo.Context) error {
	var req model.BindEmailRequest
	if !bindAndValidate(c, &req) {
		return nil
	}
	userID := middleware.GetUserID(c)
	user, err := h.authSvc.BindEmail(userID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, user)
}

func (h *AuthHandler) Logout(c echo.Context) error {
	userID := middleware.GetUserID(c)
	if err := h.authSvc.Logout(userID); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}

func (h *AuthHandler) GetProfile(c echo.Context) error {
	userID := middleware.GetUserID(c)
	user, err := h.authSvc.GetProfile(userID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, user)
}

func (h *AuthHandler) UpdateProfile(c echo.Context) error {
	var req model.UpdateProfileRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	user, err := h.authSvc.UpdateProfile(userID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, user)
}

func (h *AuthHandler) UpdatePassword(c echo.Context) error {
	var req model.UpdatePasswordRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	if err := h.authSvc.UpdatePassword(userID, req); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}
