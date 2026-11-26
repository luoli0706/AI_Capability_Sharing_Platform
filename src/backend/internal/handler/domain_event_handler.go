package handler

import (
	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/response"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type DomainEventHandler struct {
	eventSvc service.DomainEventService
}

func NewDomainEventHandler(eventSvc service.DomainEventService) *DomainEventHandler {
	return &DomainEventHandler{eventSvc: eventSvc}
}

func (h *DomainEventHandler) ListEvents(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	statusFilter := c.QueryParam("status")
	eventTypeFilter := c.QueryParam("event_type")

	list, err := h.eventSvc.ListEvents(userID, isSuperAdmin, domainID, statusFilter, eventTypeFilter)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, map[string]interface{}{
		"list": list,
	})
}

func (h *DomainEventHandler) CreateEvent(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}

	var req model.CreateDomainEventRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	event, err := h.eventSvc.CreateEvent(userID, isSuperAdmin, domainID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, event)
}

func (h *DomainEventHandler) GetEventDetail(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	eventID, ok := parseUintParam(c, "eventId")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.eventSvc.GetEventDetail(userID, isSuperAdmin, domainID, eventID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}

func (h *DomainEventHandler) AddCandidate(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	eventID, ok := parseUintParam(c, "eventId")
	if !ok {
		return nil
	}

	var req model.AddDomainEventCandidateRequest
	if !bindAndValidate(c, &req) {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.eventSvc.AddCandidate(userID, isSuperAdmin, domainID, eventID, req)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}

func (h *DomainEventHandler) VoteCandidate(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	eventID, ok := parseUintParam(c, "eventId")
	if !ok {
		return nil
	}
	candidateID, ok := parseUintParam(c, "candidateId")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.eventSvc.VoteCandidate(userID, isSuperAdmin, domainID, eventID, candidateID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}

func (h *DomainEventHandler) UnvoteCandidate(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	eventID, ok := parseUintParam(c, "eventId")
	if !ok {
		return nil
	}
	candidateID, ok := parseUintParam(c, "candidateId")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	detail, err := h.eventSvc.UnvoteCandidate(userID, isSuperAdmin, domainID, eventID, candidateID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, detail)
}

func (h *DomainEventHandler) CloseEvent(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	eventID, ok := parseUintParam(c, "eventId")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	summary, err := h.eventSvc.CloseEvent(userID, isSuperAdmin, domainID, eventID)
	if err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, summary)
}

func (h *DomainEventHandler) DeleteEvent(c echo.Context) error {
	domainID, ok := parseUintParam(c, "id")
	if !ok {
		return nil
	}
	eventID, ok := parseUintParam(c, "eventId")
	if !ok {
		return nil
	}

	userID := middleware.GetUserID(c)
	isSuperAdmin := middleware.HasRole(c, model.RoleSuperAdmin)
	if err := h.eventSvc.DeleteEvent(userID, isSuperAdmin, domainID, eventID); err != nil {
		return service.HandleError(c, err)
	}
	return response.Success(c, nil)
}
