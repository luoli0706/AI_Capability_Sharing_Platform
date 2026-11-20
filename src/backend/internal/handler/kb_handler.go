package handler

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"

	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type KBHandler struct {
	kbSvc service.KBService
}

func NewKBHandler(kbSvc service.KBService) *KBHandler {
	return &KBHandler{kbSvc: kbSvc}
}

func (h *KBHandler) CreateFolder(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.CreateKBFolderRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	folder, err := h.kbSvc.CreateFolder(userID, isSuperAdmin, kbID, req.FolderPath)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, folder)
}

func (h *KBHandler) UploadDocument(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var files []*multipart.FileHeader
	if form, err := c.MultipartForm(); err == nil && form != nil {
		if multi := form.File["files"]; len(multi) > 0 {
			files = append(files, multi...)
		}
		if single := form.File["file"]; len(single) > 0 {
			files = append(files, single...)
		}
	}
	if len(files) == 0 {
		if fh, err := c.FormFile("file"); err == nil && fh != nil {
			files = append(files, fh)
		}
	}
	if len(files) == 0 {
		return response.BadRequest(c, "file is required")
	}

	folderPath := c.FormValue("folder_path")
	relativePath := c.FormValue("relative_path")

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	docs, err := h.kbSvc.UploadDocuments(userID, isSuperAdmin, kbID, folderPath, relativePath, files)
	if err != nil {
		return service.HandleError(c, err)
	}
	if len(docs) == 1 {
		return response.Success(c, docs[0])
	}
	return response.Success(c, map[string]interface{}{"list": docs})
}

func (h *KBHandler) ListDocuments(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	list, err := h.kbSvc.ListDocuments(userID, isSuperAdmin, kbID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}

func (h *KBHandler) GetDocumentContent(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	res, err := h.kbSvc.GetDocumentContent(userID, isSuperAdmin, kbID, docID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, res)
}

func (h *KBHandler) ListChunks(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}
	page, pageSize := parsePageParams(c)

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	list, total, err := h.kbSvc.ListChunks(userID, isSuperAdmin, kbID, docID, page, pageSize)
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

func (h *KBHandler) DeleteDocument(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	if err := h.kbSvc.DeleteDocument(userID, isSuperAdmin, kbID, docID); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}

func (h *KBHandler) Search(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.KBSearchRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	hits, err := h.kbSvc.Search(userID, isSuperAdmin, kbID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": hits})
}

func (h *KBHandler) DownloadPreview(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	scope := c.QueryParam("scope")
	docID, _ := strconv.ParseUint(c.QueryParam("doc_id"), 10, 64)
	folderPath := c.QueryParam("folder_path")

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	preview, err := h.kbSvc.GetDownloadPreview(userID, isSuperAdmin, kbID, scope, docID, folderPath)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, preview)
}

func (h *KBHandler) Download(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	scope := c.QueryParam("scope")
	docID, _ := strconv.ParseUint(c.QueryParam("doc_id"), 10, 64)
	folderPath := c.QueryParam("folder_path")

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	filename, contentType, data, err := h.kbSvc.DownloadKB(userID, isSuperAdmin, kbID, scope, docID, folderPath)
	if err != nil {
		return service.HandleError(c, err)
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Blob(http.StatusOK, contentType, data)
}
