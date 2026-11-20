package handler

import (
	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type OrgHandler struct {
	orgSvc service.OrgService
}

func NewOrgHandler(orgSvc service.OrgService) *OrgHandler {
	return &OrgHandler{orgSvc: orgSvc}
}

func (h *OrgHandler) ListOrgs(c echo.Context) error {
	userID := middleware.GetUserID(c)
	list, err := h.orgSvc.ListOrgs(userID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"list":  list,
		"total": len(list),
	})
}

func (h *OrgHandler) CreateOrg(c echo.Context) error {
	var req model.CreateOrgRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	org, err := h.orgSvc.CreateOrg(userID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, org)
}

func (h *OrgHandler) GetOrgDetail(c echo.Context) error {
	orgID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	detail, err := h.orgSvc.GetOrgDetail(userID, orgID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}

func (h *OrgHandler) ListMembers(c echo.Context) error {
	orgID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	list, err := h.orgSvc.ListMembers(orgID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"list": list,
	})
}

func (h *OrgHandler) AddMember(c echo.Context) error {
	orgID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.AddOrgMemberRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	if err := h.orgSvc.AddMember(userID, orgID, req); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}

func (h *OrgHandler) RemoveMember(c echo.Context) error {
	orgID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	targetUserID, ok := parseUintParam(c, "userId")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	if err := h.orgSvc.RemoveMember(userID, orgID, targetUserID); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}
