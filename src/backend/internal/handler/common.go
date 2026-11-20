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
