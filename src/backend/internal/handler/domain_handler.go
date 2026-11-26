package handler

import (
	"strconv"

	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type DomainHandler struct {
	domainSvc service.DomainService
	govSvc    service.GovernanceService
}

func NewDomainHandler(domainSvc service.DomainService) *DomainHandler {
	return &DomainHandler{domainSvc: domainSvc}
}

func (h *DomainHandler) SetGovernanceService(govSvc service.GovernanceService) {
	h.govSvc = govSvc
}

func (h *DomainHandler) ListDomains(c echo.Context) error {
	orgID, _ := strconv.ParseUint(c.QueryParam("org_id"), 10, 64)
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if pageSize <= 0 {
		pageSize = 12
	}
	if pageSize > 100 {
		pageSize = 100
	}

	q := model.DomainQuery{
		Tab:       c.QueryParam("tab"),
		Keyword:   c.QueryParam("keyword"),
		DomainUID: c.QueryParam("domain_uid"),
		OrgID:     orgID,
		Page:      page,
		PageSize:  pageSize,
	}

	userID := middleware.GetUserID(c)
	list, total, err := h.domainSvc.ListDomains(userID, q)
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

func (h *DomainHandler) CreateDomain(c echo.Context) error {
	var req model.CreateDomainRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	domain, err := h.domainSvc.CreateDomain(userID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, domain)
}

func (h *DomainHandler) UpdateDomain(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.UpdateDomainRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	domain, err := h.domainSvc.UpdateDomain(userID, domainID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, domain)
}

func (h *DomainHandler) DeleteDomain(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	if err := h.domainSvc.DeleteDomain(userID, domainID); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}

func (h *DomainHandler) ListMembers(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	page, pageSize := parsePageParams(c)

	list, err := h.domainSvc.ListMembers(domainID)
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

func (h *DomainHandler) AddMember(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.AddDomainMemberRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	if err := h.domainSvc.AddMember(userID, domainID, req); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}

func (h *DomainHandler) UpdateMemberRole(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	targetUserID, ok := parseUintParam(c, "userId")
	if !ok {
		return nil
	}

	var req model.UpdateDomainMemberRoleRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	if err := h.domainSvc.UpdateMemberRole(userID, domainID, targetUserID, req); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}

func (h *DomainHandler) RemoveMember(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	targetUserID, ok := parseUintParam(c, "userId")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	if err := h.domainSvc.RemoveMember(userID, domainID, targetUserID); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}

func (h *DomainHandler) UpdateJoinSettings(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.UpdateDomainJoinSettingsRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	domain, err := h.domainSvc.UpdateJoinSettings(userID, domainID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, domain)
}

func (h *DomainHandler) ApplyJoinDomain(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	if h.govSvc == nil {
		return response.InternalError(c, "governance service not configured")
	}

	var req model.JoinDomainRequest
	if c.Request().ContentLength != 0 {
		if !bindAndValidate(c, &req) {
			return nil
		}
	}

	userID := middleware.GetUserID(c)
	approval, err := h.govSvc.RequestJoinDomain(userID, domainID, req.Reason)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, approval)
}

func (h *DomainHandler) JoinDomainByQuiz(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	if h.govSvc == nil {
		return response.InternalError(c, "governance service not configured")
	}

	var req model.JoinDomainByQuizRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	if err := h.govSvc.JoinDomainByQuiz(userID, domainID, req.Answer); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}
