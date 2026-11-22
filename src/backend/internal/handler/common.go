package handler

import (
	"strconv"

	"ascp-backend/internal/pkg/response"

	"github.com/labstack/echo/v4"
)

func bindAndValidate(c echo.Context, req interface{}) bool {
	if err := c.Bind(req); err != nil {
		_ = response.BadRequest(c, "invalid request payload")
		return false
	}
	if err := c.Validate(req); err != nil {
		_ = response.BadRequest(c, err.Error())
		return false
	}
	return true
}

func parseUintParam(c echo.Context, name string) (uint64, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil || id == 0 {
		_ = response.BadRequest(c, "invalid "+name)
		return 0, false
	}
	return id, true
}

func parseOptionalUintQuery(c echo.Context, name string) uint64 {
	raw := c.QueryParam(name)
	if raw == "" {
		return 0
	}
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func parsePageParams(c echo.Context) (int, int) {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func parsePageParamsWithDefault(c echo.Context, defaultSize int) (int, int) {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if pageSize <= 0 {
		pageSize = defaultSize
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}

func paginateSlice[T any](items []T, page, pageSize int) ([]T, int64) {
	total := int64(len(items))
	if total == 0 {
		return []T{}, 0
	}
	start := (page - 1) * pageSize
	if start >= len(items) {
		return []T{}, total
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return items[start:end], total
}
