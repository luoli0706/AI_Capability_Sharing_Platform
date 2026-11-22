package handler

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

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

func (h *AssetHandler) CheckAssetAccess(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, _ := strconv.ParseUint(c.QueryParam("doc_id"), 10, 64)
	refDomainID, _ := strconv.ParseUint(c.QueryParam("ref_domain_id"), 10, 64)

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	result, err := h.assetSvc.CheckAssetAccess(userID, isSuperAdmin, id, docID, refDomainID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, result)
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
	detail, err := h.assetSvc.PublishAsset(userID, isSuperAdmin, id, req.Version, req.Changelog)
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

func (h *AssetHandler) ListVersions(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	list, err := h.assetSvc.ListVersions(userID, isSuperAdmin, id)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}

func (h *AssetHandler) RollbackAsset(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.RollbackAssetRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.assetSvc.RollbackAsset(userID, isSuperAdmin, id, req.VersionID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}

func (h *AssetHandler) CheckMCPHealth(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	cfg, err := h.assetSvc.CheckMCPHealth(userID, isSuperAdmin, id)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, cfg)
}

func (h *AssetHandler) SyncMCPTools(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	cfg, err := h.assetSvc.SyncMCPTools(userID, isSuperAdmin, id)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, cfg)
}

func readMultipartFile(c echo.Context) (string, []byte, bool) {
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		_ = response.BadRequest(c, "file is required")
		return "", nil, false
	}
	src, err := fh.Open()
	if err != nil {
		_ = response.BadRequest(c, "failed to open uploaded file")
		return "", nil, false
	}
	defer func() { _ = src.Close() }()

	data, err := io.ReadAll(io.LimitReader(src, 20<<20))
	if err != nil {
		_ = response.BadRequest(c, "failed to read uploaded file")
		return "", nil, false
	}
	return fh.Filename, data, true
}

func (h *AssetHandler) ParseSkillFile(c echo.Context) error {
	filename, data, ok := readMultipartFile(c)
	if !ok {
		return nil
	}

	res, err := h.assetSvc.ParseSkillFile(filename, data)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, res)
}

func (h *AssetHandler) ImportSkillURL(c echo.Context) error {
	var req model.ImportSkillURLRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	res, err := h.assetSvc.ImportSkillFromURL(req.URL)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, res)
}

func (h *AssetHandler) UploadSkillPackage(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	filename, data, ok := readMultipartFile(c)
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.assetSvc.UploadSkillPackage(userID, isSuperAdmin, id, filename, data)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}

func (h *AssetHandler) DownloadSkillPackage(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	format := c.QueryParam("format")
	if format == "" {
		format = "zip"
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	filename, contentType, data, err := h.assetSvc.DownloadSkillPackage(userID, isSuperAdmin, id, format)
	if err != nil {
		return service.HandleError(c, err)
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Blob(http.StatusOK, contentType, data)
}

func (h *AssetHandler) SearchMCPRegistry(c echo.Context) error {
	query := c.QueryParam("q")
	limit, _ := strconv.Atoi(c.QueryParam("limit"))
	if limit <= 0 {
		limit = 10
	}
	if limit > 20 {
		limit = 20
	}

	list, err := h.assetSvc.SearchMCPRegistry(c.Request().Context(), query, limit)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}

func (h *AssetHandler) ParseMCPConfig(c echo.Context) error {
	var req model.ParseMCPConfigRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	list, err := h.assetSvc.ParseMCPConfigJSON(req.Content)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}

func (h *AssetHandler) ParseAgentFile(c echo.Context) error {
	ct := c.Request().Header.Get(echo.HeaderContentType)
	var raw, filename string
	if strings.Contains(ct, echo.MIMEApplicationJSON) {
		var req model.ParseAgentContentRequest
		if !bindAndValidate(c, &req) {
			return nil
		}
		raw = req.Content
		filename = req.Filename
	} else {
		fn, data, ok := readMultipartFile(c)
		if !ok {
			return nil
		}
		raw = string(data)
		filename = fn
	}

	res, err := h.assetSvc.ParseAgentFile(raw, filename)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, res)
}

func (h *AssetHandler) ImportAgentURL(c echo.Context) error {
	var req model.ImportAgentURLRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	res, err := h.assetSvc.ImportAgentFromURL(c.Request().Context(), req.URL)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, res)
}

func (h *AssetHandler) DownloadAgentPackage(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	format := c.QueryParam("format")
	if format == "" {
		format = "md"
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	filename, contentType, data, err := h.assetSvc.DownloadAgentPackage(userID, isSuperAdmin, id, format)
	if err != nil {
		return service.HandleError(c, err)
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Blob(http.StatusOK, contentType, data)
}
