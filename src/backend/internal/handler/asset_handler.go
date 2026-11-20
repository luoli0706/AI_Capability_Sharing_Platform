package handler

import (
	"strconv"

	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type AssetHandler struct {
	assetSvc service.AssetService
}

func NewAssetHandler(assetSvc service.AssetService) *AssetHandler {
	return &AssetHandler{assetSvc: assetSvc}
}

func (h *AssetHandler) ListAssets(c echo.Context) error {
	page, pageSize := parsePageParams(c)
	domainID, _ := strconv.ParseUint(c.QueryParam("domain_id"), 10, 64)

	q := model.AssetQuery{
		AssetType:  c.QueryParam("asset_type"),
		DomainID:   domainID,
		Status:     c.QueryParam("status"),
		Visibility: c.QueryParam("visibility"),
		Keyword:    c.QueryParam("keyword"),
		Scope:      c.QueryParam("scope"),
		Page:       page,
		PageSize:   pageSize,
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	list, total, err := h.assetSvc.ListAssets(userID, isSuperAdmin, q)
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

func (h *AssetHandler) ListUserAssets(c echo.Context) error {
	page, pageSize := parsePageParams(c)
	q := model.AssetQuery{
		AssetType: c.QueryParam("asset_type"),
		Status:    c.QueryParam("status"),
		Page:      page,
		PageSize:  pageSize,
	}

	userID := middleware.GetUserID(c)
	list, total, err := h.assetSvc.ListUserAssets(userID, q)
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

func (h *AssetHandler) GetAsset(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.assetSvc.GetAssetDetail(userID, isSuperAdmin, id)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}

func (h *AssetHandler) CreateAsset(c echo.Context) error {
	var req model.CreateAssetRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.assetSvc.CreateAsset(userID, isSuperAdmin, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}

func (h *AssetHandler) UpdateAsset(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.UpdateAssetRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.assetSvc.UpdateAsset(userID, isSuperAdmin, id, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}

func (h *AssetHandler) DeleteAsset(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	if err := h.assetSvc.DeleteAsset(userID, isSuperAdmin, id); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}

func (h *AssetHandler) PublishAsset(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.PublishAssetRequest
	if c.Request().ContentLength > 0 {
		if !bindAndValidate(c, &req) {
			return nil
		}
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.assetSvc.PublishAsset(userID, isSuperAdmin, id, req.Version)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}

func (h *AssetHandler) OfflineAsset(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.assetSvc.OfflineAsset(userID, isSuperAdmin, id)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}
