package handler

import (
	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type KBInteractionHandler struct {
	interactionSvc service.KBInteractionService
}

func NewKBInteractionHandler(interactionSvc service.KBInteractionService) *KBInteractionHandler {
	return &KBInteractionHandler{interactionSvc: interactionSvc}
}

func (h *KBInteractionHandler) GetInteractions(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}
	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	res, err := h.interactionSvc.GetInteractions(userID, isSuperAdmin, kbID, docID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, res)
}

func (h *KBInteractionHandler) ToggleLike(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}
	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	res, err := h.interactionSvc.ToggleLike(userID, isSuperAdmin, kbID, docID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, res)
}

func (h *KBInteractionHandler) ToggleFavorite(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}
	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	favorited, err := h.interactionSvc.ToggleFavorite(userID, isSuperAdmin, kbID, docID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"favorited_by_me": favorited})
}

func (h *KBInteractionHandler) ListComments(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}
	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	list, err := h.interactionSvc.ListComments(userID, isSuperAdmin, kbID, docID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}

func (h *KBInteractionHandler) CreateComment(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}
	var req model.CreateKBCommentRequest
	if !bindAndValidate(c, &req) {
		return nil
	}
	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	item, err := h.interactionSvc.CreateComment(userID, isSuperAdmin, kbID, docID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, item)
}

func (h *KBInteractionHandler) DeleteComment(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}
	commentID, ok := parseUintParam(c, "commentId")
	if !ok {
		return nil
	}
	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	if err := h.interactionSvc.DeleteComment(userID, isSuperAdmin, kbID, docID, commentID); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"ok": true})
}
