package handler

import (
	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type GovernanceHandler struct {
	govSvc service.GovernanceService
}

func NewGovernanceHandler(govSvc service.GovernanceService) *GovernanceHandler {
	return &GovernanceHandler{govSvc: govSvc}
}

func (h *GovernanceHandler) SubscribeAsset(c echo.Context) error {
	assetID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	perm, err := h.govSvc.SubscribeAsset(userID, assetID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, perm)
}

func (h *GovernanceHandler) CreateApproval(c echo.Context) error {
	var req model.CreateApprovalRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	approval, err := h.govSvc.CreateApproval(userID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, approval)
}

func (h *GovernanceHandler) ListMyRequests(c echo.Context) error {
	userID := middleware.GetUserID(c)
	list, err := h.govSvc.ListMyRequests(userID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}

func (h *GovernanceHandler) ListPendingRequests(c echo.Context) error {
	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	list, err := h.govSvc.ListPendingRequests(userID, isSuperAdmin)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}

func (h *GovernanceHandler) ListHandledRequests(c echo.Context) error {
	userID := middleware.GetUserID(c)
	list, err := h.govSvc.ListHandledRequests(userID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}

func (h *GovernanceHandler) HandleApproval(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.HandleApprovalRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	approval, err := h.govSvc.HandleApproval(userID, isSuperAdmin, id, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, approval)
}
