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
	govSvc service.GovernanceService
}

func NewOrgHandler(orgSvc service.OrgService, govSvc service.GovernanceService) *OrgHandler {
	return &OrgHandler{orgSvc: orgSvc, govSvc: govSvc}
}

func (h *OrgHandler) ListOrgs(c echo.Context) error {
	page, pageSize := parsePageParamsWithDefault(c, 12)
	q := model.OrgQuery{
		Tab:          c.QueryParam("tab"),
		Keyword:      c.QueryParam("keyword"),
		IsSuperAdmin: middleware.HasRole(c, model.RoleSuperAdmin),
		Page:         page,
		PageSize:     pageSize,
	}

	userID := middleware.GetUserID(c)
	list, total, err := h.orgSvc.ListOrgs(userID, q)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
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

func (h *OrgHandler) UpdateOrg(c echo.Context) error {
	orgID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.UpdateOrgRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	org, err := h.orgSvc.UpdateOrg(userID, orgID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, org)
}

func (h *OrgHandler) DeleteOrg(c echo.Context) error {
	orgID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	if err := h.orgSvc.DeleteOrg(userID, orgID); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}

func (h *OrgHandler) GetOrgDetail(c echo.Context) error {
	orgID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.orgSvc.GetOrgDetail(userID, isSuperAdmin, orgID)
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
	page, pageSize := parsePageParams(c)

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	list, err := h.orgSvc.ListMembers(userID, isSuperAdmin, orgID)
	if err != nil {
		return service.HandleError(c, err)
	}
	paged, total := paginateSlice(list, page, pageSize)
	return response.Success(c, map[string]interface{}{
		"list":      paged,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
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

func (h *OrgHandler) JoinOrg(c echo.Context) error {
	orgID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.JoinOrgRequest
	if c.Request().ContentLength != 0 {
		if !bindAndValidate(c, &req) {
			return nil
		}
	}

	userID := middleware.GetUserID(c)
	approval, err := h.govSvc.RequestJoinOrg(userID, orgID, req.Reason)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, approval)
}
