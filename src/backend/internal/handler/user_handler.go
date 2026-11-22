package handler

import (
	"ascp-backend/internal/middleware"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type UserHandler struct {
	userSvc service.UserService
}

func NewUserHandler(userSvc service.UserService) *UserHandler {
	return &UserHandler{userSvc: userSvc}
}

func (h *UserHandler) SearchUsers(c echo.Context) error {
	userID := middleware.GetUserID(c)
	keyword := c.QueryParam("keyword")
	list, err := h.userSvc.SearchUsers(userID, keyword)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}

func (h *UserHandler) GetPublicProfile(c echo.Context) error {
	targetID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	profile, err := h.userSvc.GetPublicProfile(targetID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, profile)
}

func (h *UserHandler) GetUserFavorites(c echo.Context) error {
	targetID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	viewerID := middleware.GetUserID(c)
	isPublic, list, err := h.userSvc.GetUserFavorites(viewerID, targetID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"is_public": isPublic,
		"list":      list,
	})
}

func (h *UserHandler) GetMyFavorites(c echo.Context) error {
	userID := middleware.GetUserID(c)
	list, err := h.userSvc.GetMyFavorites(userID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}
