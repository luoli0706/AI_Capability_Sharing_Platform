package handler

import (
	"strconv"

	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type APIKeyHandler struct {
	keySvc service.APIKeyService
}

func NewAPIKeyHandler(keySvc service.APIKeyService) *APIKeyHandler {
	return &APIKeyHandler{keySvc: keySvc}
}

func (h *APIKeyHandler) ListKeys(c echo.Context) error {
	page, pageSize := parsePageParams(c)
	userID := middleware.GetUserID(c)
	list, err := h.keySvc.ListKeys(userID)
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

func (h *APIKeyHandler) CreateKey(c echo.Context) error {
	var req model.CreateAPIKeyRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	plainKey, info, err := h.keySvc.CreateKey(userID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"api_key":  plainKey,
		"key_info": info,
	})
}

func (h *APIKeyHandler) RevokeKey(c echo.Context) error {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	if err := h.keySvc.RevokeKey(userID, id); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}

type InvokeHandler struct {
	invokeSvc service.InvokeService
}

func NewInvokeHandler(invokeSvc service.InvokeService) *InvokeHandler {
	return &InvokeHandler{invokeSvc: invokeSvc}
}

func (h *InvokeHandler) invokeByType(c echo.Context, expectedType string) error {
	assetCode := c.Param("assetCode")
	if assetCode == "" {
		return response.BadRequest(c, "missing assetCode")
	}

	var payload map[string]interface{}
	if c.Request().ContentLength > 0 {
		if err := c.Bind(&payload); err != nil {
			return response.BadRequest(c, "invalid JSON payload")
		}
	}
	if payload == nil {
		payload = map[string]interface{}{}
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	authMode := middleware.GetAuthMode(c)
	traceID := c.Response().Header().Get(echo.HeaderXRequestID)

	res, err := h.invokeSvc.InvokeAsset(
		userID,
		isSuperAdmin,
		assetCode,
		expectedType,
		authMode,
		c.RealIP(),
		traceID,
		payload,
	)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, res)
}

func (h *InvokeHandler) InvokeMCP(c echo.Context) error {
	return h.invokeByType(c, model.AssetTypeMCP)
}

func (h *InvokeHandler) InvokeSkill(c echo.Context) error {
	return h.invokeByType(c, model.AssetTypeSkill)
}

func (h *InvokeHandler) InvokeAgent(c echo.Context) error {
	return h.invokeByType(c, model.AssetTypeAgent)
}

func (h *InvokeHandler) InvokeKB(c echo.Context) error {
	return h.invokeByType(c, model.AssetTypeKB)
}

func (h *InvokeHandler) ListInvokeLogs(c echo.Context) error {
	page, pageSize := parsePageParams(c)
	assetType := c.QueryParam("asset_type")
	var isSuccess *int8
	if raw := c.QueryParam("is_success"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil {
			val := int8(v)
			isSuccess = &val
		}
	}

	userID := middleware.GetUserID(c)
	list, total, err := h.invokeSvc.ListInvokeLogs(userID, assetType, isSuccess, page, pageSize)
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
