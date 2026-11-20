package handler

import (
	"strconv"

	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type AuditHandler struct {
	auditSvc service.AuditService
}

func NewAuditHandler(auditSvc service.AuditService) *AuditHandler {
	return &AuditHandler{auditSvc: auditSvc}
}

func (h *AuditHandler) ListLogs(c echo.Context) error {
	page, pageSize := parsePageParams(c)
	domainID, _ := strconv.ParseUint(c.QueryParam("domain_id"), 10, 64)

	q := model.AuditLogQuery{
		AssetType: c.QueryParam("asset_type"),
		Action:    c.QueryParam("action"),
		DomainID:  domainID,
		Keyword:   c.QueryParam("keyword"),
		Page:      page,
		PageSize:  pageSize,
	}

	userID := middleware.GetUserID(c)
	list, total, err := h.auditSvc.ListManagedLogs(c.Request().Context(), userID, q)
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
