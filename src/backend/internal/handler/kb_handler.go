package handler

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

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

func (h *KBHandler) CreateMarkdownFile(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.CreateKBFileRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	doc, err := h.kbSvc.CreateMarkdownFile(userID, isSuperAdmin, kbID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, doc)
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

func isTruthyQueryParam(c echo.Context, key string) bool {
	val := strings.TrimSpace(strings.ToLower(c.QueryParam(key)))
	return val == "1" || val == "true"
}

func (h *KBHandler) ListDocuments(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	refDomainID := parseOptionalUintQuery(c, "ref_domain_id")
	if isTruthyQueryParam(c, "lazy") {
		folderPath := c.QueryParam("folder_path")
		offset, _ := strconv.Atoi(c.QueryParam("offset"))
		limit, _ := strconv.Atoi(c.QueryParam("limit"))
		extGroup := c.QueryParam("ext_group")
		fields := c.QueryParam("fields")

		etag, err := h.kbSvc.GetFolderETag(userID, isSuperAdmin, kbID, folderPath, offset, limit, extGroup, fields, refDomainID)
		if err != nil {
			return service.HandleError(c, err)
		}
		if inm := strings.TrimSpace(c.Request().Header.Get("If-None-Match")); inm != "" && inm == etag {
			c.Response().Header().Set("ETag", etag)
			return c.NoContent(http.StatusNotModified)
		}

		res, err := h.kbSvc.ListDocumentsLazyPaged(userID, isSuperAdmin, kbID, folderPath, offset, limit, extGroup, fields, refDomainID)
		if err != nil {
			return service.HandleError(c, err)
		}
		c.Response().Header().Set("ETag", etag)
		return response.Success(c, res)
	}

	list, err := h.kbSvc.ListDocuments(userID, isSuperAdmin, kbID, refDomainID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{"list": list})
}

func (h *KBHandler) GetDocumentMeta(c echo.Context) error {
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
	refDomainID := parseOptionalUintQuery(c, "ref_domain_id")
	meta, err := h.kbSvc.GetDocumentMeta(userID, isSuperAdmin, kbID, docID, refDomainID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, meta)
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
	obf := isTruthyQueryParam(c, "obf")
	refDomainID := parseOptionalUintQuery(c, "ref_domain_id")
	res, err := h.kbSvc.GetDocumentContent(userID, isSuperAdmin, kbID, docID, obf, refDomainID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, res)
}

func (h *KBHandler) GetDocumentRaw(c echo.Context) error {
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
	obf := isTruthyQueryParam(c, "obf")
	refDomainID := parseOptionalUintQuery(c, "ref_domain_id")
	fileName, mimeType, isSVG, data, err := h.kbSvc.GetDocumentRaw(userID, isSuperAdmin, kbID, docID, obf, refDomainID)
	if err != nil {
		return service.HandleError(c, err)
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`inline; filename="%s"`, fileName))
	if isSVG {
		c.Response().Header().Set("Content-Security-Policy", "sandbox")
	}
	if obf {
		c.Response().Header().Set("X-ASCP-Obf", "1")
	}
	return c.Blob(http.StatusOK, mimeType, data)
}

func (h *KBHandler) GetBilibiliPreview(c echo.Context) error {
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
	refDomainID := parseOptionalUintQuery(c, "ref_domain_id")
	preview, err := h.kbSvc.GetBilibiliPreview(userID, isSuperAdmin, kbID, docID, refDomainID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, preview)
}

func (h *KBHandler) UpdateDocumentContent(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}

	var req model.UpdateKBFileRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	doc, err := h.kbSvc.UpdateMarkdownContent(userID, isSuperAdmin, kbID, docID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, doc)
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
	refDomainID := parseOptionalUintQuery(c, "ref_domain_id")
	list, total, err := h.kbSvc.ListChunks(userID, isSuperAdmin, kbID, docID, page, pageSize, refDomainID)
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

func (h *KBHandler) RestoreDocument(c echo.Context) error {
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
	doc, err := h.kbSvc.RestoreDocument(userID, isSuperAdmin, kbID, docID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, doc)
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
	refDomainID := parseOptionalUintQuery(c, "ref_domain_id")
	hits, err := h.kbSvc.Search(userID, isSuperAdmin, kbID, req, refDomainID)
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
	refDomainID := parseOptionalUintQuery(c, "ref_domain_id")
	preview, err := h.kbSvc.GetDownloadPreview(userID, isSuperAdmin, kbID, scope, docID, folderPath, refDomainID)
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
	refDomainID := parseOptionalUintQuery(c, "ref_domain_id")
	filename, contentType, data, err := h.kbSvc.DownloadKB(userID, isSuperAdmin, kbID, scope, docID, folderPath, refDomainID)
	if err != nil {
		return service.HandleError(c, err)
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf(`attachment; filename="%s"`, filename))
	return c.Blob(http.StatusOK, contentType, data)
}

func (h *KBHandler) MoveDocument(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}

	var req model.MoveKBDocumentRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	doc, err := h.kbSvc.MoveDocument(userID, isSuperAdmin, kbID, docID, req.TargetFolderPath)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, doc)
}

func (h *KBHandler) CopyDocument(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}

	var req model.CopyKBDocumentRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	doc, err := h.kbSvc.CopyDocument(userID, isSuperAdmin, kbID, docID, req.TargetFolderPath, req.NewName)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, doc)
}

func (h *KBHandler) RenameDocument(c echo.Context) error {
	kbID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	docID, ok := parseUintParam(c, "docId")
	if !ok {
		return nil
	}

	var req model.RenameKBDocumentRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	doc, err := h.kbSvc.RenameDocument(userID, isSuperAdmin, kbID, docID, req.NewName)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, doc)
}

func (h *KBHandler) ResolveExternalVideoURL(c echo.Context) error {
	rawURL := strings.TrimSpace(c.QueryParam("url"))
	resolved, err := h.kbSvc.ResolveExternalVideoURL(rawURL)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]string{
		"resolved_url": resolved,
	})
}
