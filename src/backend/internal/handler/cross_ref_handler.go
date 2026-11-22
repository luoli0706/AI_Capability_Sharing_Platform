package handler

import (
	"strconv"

	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type CrossRefHandler struct {
	svc service.CrossRefService
}

func NewCrossRefHandler(svc service.CrossRefService) *CrossRefHandler {
	return &CrossRefHandler{svc: svc}
}

func (h *CrossRefHandler) CreateCrossRef(c echo.Context) error {
	targetDomainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.CreateCrossRefRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	ref, err := h.svc.CreateCrossRef(userID, isSuperAdmin, targetDomainID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, ref)
}

func (h *CrossRefHandler) ListCrossRefs(c echo.Context) error {
	targetDomainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	inc := c.QueryParam("include_pending")
	includePending := inc == "1" || inc == "true"

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	list, err := h.svc.ListCrossRefs(userID, isSuperAdmin, targetDomainID, includePending)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"list": list,
	})
}

func (h *CrossRefHandler) DeleteCrossRef(c echo.Context) error {
	targetDomainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	refID, ok := parseUintParam(c, "refId")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	if err := h.svc.DeleteCrossRef(userID, isSuperAdmin, targetDomainID, refID); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}

func (h *CrossRefHandler) ListSourceDomains(c echo.Context) error {
	var excludeDomainID uint64
	if raw := c.QueryParam("exclude_domain_id"); raw != "" {
		parsed, err := strconv.ParseUint(raw, 10, 64)
		if err != nil {
			return response.BadRequest(c, "invalid exclude_domain_id")
		}
		excludeDomainID = parsed
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	list, err := h.svc.ListSourceDomains(userID, isSuperAdmin, excludeDomainID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"list": list,
	})
}

func (h *CrossRefHandler) ListSourceItems(c echo.Context) error {
	raw := c.QueryParam("source_domain_id")
	if raw == "" {
		return response.BadRequest(c, "source_domain_id is required")
	}
	sourceDomainID, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || sourceDomainID == 0 {
		return response.BadRequest(c, "invalid source_domain_id")
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	assets, kbFiles, err := h.svc.ListSourceItems(userID, isSuperAdmin, sourceDomainID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"assets":   assets,
		"kb_files": kbFiles,
	})
}
