package handler

import (
	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type DomainHandler struct {
	domainSvc service.DomainService
}

func NewDomainHandler(domainSvc service.DomainService) *DomainHandler {
	return &DomainHandler{domainSvc: domainSvc}
}

func (h *DomainHandler) ListDomains(c echo.Context) error {
	userID := middleware.GetUserID(c)
	list, err := h.domainSvc.ListDomains(userID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"list":  list,
		"total": len(list),
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

func (h *DomainHandler) ListMembers(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	list, err := h.domainSvc.ListMembers(domainID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"list": list,
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
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	if err := h.domainSvc.AddMember(userID, isSuperAdmin, domainID, req); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}
