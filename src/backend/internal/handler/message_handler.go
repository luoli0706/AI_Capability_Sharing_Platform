package handler

import (
	"strconv"

	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type MessageHandler struct {
	msgSvc service.MessageService
}

func NewMessageHandler(msgSvc service.MessageService) *MessageHandler {
	return &MessageHandler{msgSvc: msgSvc}
}

func (h *MessageHandler) GetUnreadCount(c echo.Context) error {
	userID := middleware.GetUserID(c)
	count, err := h.msgSvc.GetUnreadCount(userID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"unread_count": count})
}

func (h *MessageHandler) ListConversations(c echo.Context) error {
	userID := middleware.GetUserID(c)
	list, err := h.msgSvc.ListConversations(userID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}

func (h *MessageHandler) CreateConversation(c echo.Context) error {
	var req model.CreateConversationRequest
	if !bindAndValidate(c, &req) {
		return nil
	}
	userID := middleware.GetUserID(c)
	item, err := h.msgSvc.CreateOrGetConversation(userID, req.TargetUserID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, item)
}

func (h *MessageHandler) ListMessages(c echo.Context) error {
	convID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	beforeID, _ := strconv.ParseUint(c.QueryParam("before_id"), 10, 64)
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 50
	}

	userID := middleware.GetUserID(c)
	list, hasMore, err := h.msgSvc.ListMessages(userID, convID, beforeID, limit)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"list":     list,
		"has_more": hasMore,
	})
}

func (h *MessageHandler) SendMessage(c echo.Context) error {
	convID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	var req model.SendMessageRequest
	if !bindAndValidate(c, &req) {
		return nil
	}
	userID := middleware.GetUserID(c)
	msg, err := h.msgSvc.SendMessage(userID, convID, req.Content)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, msg)
}

func (h *MessageHandler) MarkRead(c echo.Context) error {
	convID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	userID := middleware.GetUserID(c)
	if err := h.msgSvc.MarkRead(userID, convID); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"ok": true})
}
