package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"ascp-backend/internal/middleware"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/validator"
	"ascp-backend/internal/service"

	"github.com/labstack/echo/v4"
)

type stubGovService struct {
	service.GovernanceService
	lastOrgID    uint64
	lastReason   string
	lastDomainID uint64
	lastAnswer   string
}

func (s *stubGovService) RequestJoinOrg(userID, orgID uint64, reason string) (*model.ApprovalRequest, error) {
	s.lastOrgID = orgID
	s.lastReason = reason
	return &model.ApprovalRequest{
		ID:          101,
		ApplicantID: userID,
		TargetType:  model.ApprovalTargetOrgJoin,
		AssetID:     orgID,
		Reason:      reason,
		Status:      model.ApprovalStatusPending,
	}, nil
}

func (s *stubGovService) RequestJoinDomain(userID, domainID uint64, reason string) (*model.ApprovalRequest, error) {
	s.lastDomainID = domainID
	s.lastReason = reason
	return &model.ApprovalRequest{
		ID:          102,
		ApplicantID: userID,
		TargetType:  model.ApprovalTargetDomainJoin,
		AssetID:     domainID,
		Reason:      reason,
		Status:      model.ApprovalStatusPending,
	}, nil
}

func (s *stubGovService) JoinDomainByQuiz(userID, domainID uint64, answer string) error {
	s.lastDomainID = domainID
	s.lastAnswer = answer
	return nil
}

func TestOrgJoinReasonAndDomainJoinHandlers(t *testing.T) {
	e := echo.New()
	e.Validator = validator.New()

	govStub := &stubGovService{}
	orgH := NewOrgHandler(nil, govStub)
	domH := NewDomainHandler(nil)
	domH.SetGovernanceService(govStub)

	// 1. Org join-requests passes reason cleanly (even when ContentLength is -1 / chunked)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/orgs/7/join-requests", bytes.NewBufferString(`{"reason":"申请加入研发组织"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	req.ContentLength = -1
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues("7")
	c.Set(middleware.ContextKeyUserID, uint64(42))

	if err := orgH.JoinOrg(c); err != nil {
		t.Fatalf("JoinOrg failed: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if govStub.lastOrgID != 7 || govStub.lastReason != "申请加入研发组织" {
		t.Fatalf("expected orgID=7 reason='申请加入研发组织', got orgID=%d reason=%q", govStub.lastOrgID, govStub.lastReason)
	}
	var respMap map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &respMap); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	data := respMap["data"].(map[string]interface{})
	if data["reason"] != "申请加入研发组织" {
		t.Fatalf("expected response data.reason='申请加入研发组织', got %v", data["reason"])
	}

	// 2. Domain join-requests passes reason cleanly
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/domains/9/join-requests", bytes.NewBufferString(`{"reason":"需要使用域内MCP"}`))
	req2.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec2 := httptest.NewRecorder()
	c2 := e.NewContext(req2, rec2)
	c2.SetParamNames("id")
	c2.SetParamValues("9")
	c2.Set(middleware.ContextKeyUserID, uint64(42))

	if err := domH.ApplyJoinDomain(c2); err != nil {
		t.Fatalf("ApplyJoinDomain failed: %v", err)
	}
	if rec2.Code != http.StatusOK || govStub.lastDomainID != 9 || govStub.lastReason != "需要使用域内MCP" {
		t.Fatalf("unexpected ApplyJoinDomain result: code=%d domainID=%d reason=%q", rec2.Code, govStub.lastDomainID, govStub.lastReason)
	}

	// 3. Domain join-by-quiz passes answer cleanly
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/domains/9/join-by-quiz", bytes.NewBufferString(`{"answer":"secret-code"}`))
	req3.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec3 := httptest.NewRecorder()
	c3 := e.NewContext(req3, rec3)
	c3.SetParamNames("id")
	c3.SetParamValues("9")
	c3.Set(middleware.ContextKeyUserID, uint64(42))

	if err := domH.JoinDomainByQuiz(c3); err != nil {
		t.Fatalf("JoinDomainByQuiz failed: %v", err)
	}
	if rec3.Code != http.StatusOK || govStub.lastAnswer != "secret-code" {
		t.Fatalf("unexpected JoinDomainByQuiz result: code=%d answer=%q", rec3.Code, govStub.lastAnswer)
	}
}
