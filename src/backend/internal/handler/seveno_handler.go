package handler

import (
	"errors"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

// SevenOHandler handles 7o release distribution and online encrypt/decrypt/inspect endpoints.
type SevenOHandler struct {
	svc *service.SevenOService
}

func NewSevenOHandler(svc *service.SevenOService) *SevenOHandler {
	return &SevenOHandler{svc: svc}
}

func (h *SevenOHandler) ListReleases(c echo.Context) error {
	data, err := h.svc.ListReleases()
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, data)
}

func (h *SevenOHandler) DownloadRelease(c echo.Context) error {
	filename := c.Param("filename")
	fullPath, err := h.svc.GetReleaseFilePath(filename)
	if err != nil {
		return service.HandleError(c, err)
	}
	return c.Attachment(fullPath, filepath.Base(fullPath))
}

func (h *SevenOHandler) limitRequestBody(c echo.Context) {
	if maxBytes := h.svc.MaxRequestBytes(); maxBytes > 0 {
		c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, maxBytes+(1<<20))
	}
}

func (h *SevenOHandler) payloadTooLargeResponse(c echo.Context) error {
	mb := h.svc.MaxRequestBytes() / (1024 * 1024)
	if mb <= 0 {
		mb = 50
	}
	return response.Error(c, http.StatusRequestEntityTooLarge, 41301, fmt.Sprintf("在线处理文件大小超过 %dMB 限制，请下载桌面客户端处理", mb))
}

func isMaxBytesError(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func setAttachmentDisposition(c echo.Context, outName string) {
	disp := mime.FormatMediaType("attachment", map[string]string{"filename": outName})
	if disp == "" {
		disp = fmt.Sprintf("attachment; filename=%q", outName)
	}
	c.Response().Header().Set(echo.HeaderContentDisposition, disp)
}

func (h *SevenOHandler) Encrypt(c echo.Context) error {
	h.limitRequestBody(c)

	var files []*multipart.FileHeader
	var relPaths []string

	form, err := c.MultipartForm()
	if err != nil && isMaxBytesError(err) {
		return h.payloadTooLargeResponse(c)
	}
	if err == nil && form != nil {
		if multi := form.File["files"]; len(multi) > 0 {
			files = append(files, multi...)
		}
		if single := form.File["file"]; len(single) > 0 {
			files = append(files, single...)
		}
		if paths := form.Value["rel_paths"]; len(paths) > 0 {
			relPaths = paths
		} else if paths := form.Value["relative_paths"]; len(paths) > 0 {
			relPaths = paths
		}
	}
	if len(files) == 0 {
		fh, err := c.FormFile("file")
		if err != nil && isMaxBytesError(err) {
			return h.payloadTooLargeResponse(c)
		}
		if err == nil && fh != nil {
			files = append(files, fh)
		}
	}
	if len(files) == 0 {
		return response.BadRequest(c, "file is required")
	}
	if len(relPaths) == 0 {
		if rp := strings.TrimSpace(c.FormValue("relative_path")); rp != "" {
			relPaths = []string{rp}
		}
	}

	password := c.FormValue("password")
	level := 3
	if levelStr := strings.TrimSpace(c.FormValue("level")); levelStr != "" {
		parsed, err := strconv.Atoi(levelStr)
		if err != nil {
			return response.BadRequest(c, "invalid compression level")
		}
		level = parsed
	}

	zipBytes, outName, err := h.svc.EncryptFiles(c.Request().Context(), files, relPaths, password, level)
	if err != nil {
		return service.HandleError(c, err)
	}

	setAttachmentDisposition(c, outName)
	return c.Blob(http.StatusOK, "application/zip", zipBytes)
}

func (h *SevenOHandler) Decrypt(c echo.Context) error {
	h.limitRequestBody(c)

	fh, err := c.FormFile("file")
	if err != nil {
		if isMaxBytesError(err) {
			return h.payloadTooLargeResponse(c)
		}
		return response.BadRequest(c, "file is required")
	}
	if fh == nil {
		return response.BadRequest(c, "file is required")
	}
	if maxBytes := h.svc.MaxRequestBytes(); maxBytes > 0 && fh.Size > maxBytes {
		return h.payloadTooLargeResponse(c)
	}
	src, err := fh.Open()
	if err != nil {
		return response.BadRequest(c, "failed to read uploaded file")
	}
	defer func() { _ = src.Close() }()

	password := c.FormValue("password")
	outZipBytes, err := h.svc.DecryptZip(c.Request().Context(), src, password)
	if err != nil {
		return service.HandleError(c, err)
	}

	outName := "decrypted.zip"
	if base := strings.TrimSuffix(filepath.Base(fh.Filename), filepath.Ext(fh.Filename)); base != "" && base != "." {
		outName = base + "-decrypted.zip"
	}
	setAttachmentDisposition(c, outName)
	return c.Blob(http.StatusOK, "application/zip", outZipBytes)
}

func (h *SevenOHandler) Inspect(c echo.Context) error {
	h.limitRequestBody(c)

	fh, err := c.FormFile("file")
	if err != nil {
		if isMaxBytesError(err) {
			return h.payloadTooLargeResponse(c)
		}
		return response.BadRequest(c, "file is required")
	}
	if fh == nil {
		return response.BadRequest(c, "file is required")
	}
	if maxBytes := h.svc.MaxRequestBytes(); maxBytes > 0 && fh.Size > maxBytes {
		return h.payloadTooLargeResponse(c)
	}
	src, err := fh.Open()
	if err != nil {
		return response.BadRequest(c, "failed to read uploaded file")
	}
	defer func() { _ = src.Close() }()

	info, err := h.svc.InspectZip(c.Request().Context(), src)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, info)
}
