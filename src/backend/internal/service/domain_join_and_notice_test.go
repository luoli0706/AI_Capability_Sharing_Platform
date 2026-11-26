package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"ascp-backend/internal/config"
	"ascp-backend/internal/model"
	"ascp-backend/internal/pkg/password"

	"gorm.io/gorm"
)

type testJoinMsgRepo struct {
	nextConvID uint64
	nextMsgID  uint64
	convs      map[uint64]*model.UserConversation
	msgs       []model.UserMessage
}

func newTestJoinMsgRepo() *testJoinMsgRepo {
	return &testJoinMsgRepo{
		nextConvID: 1,
		nextMsgID:  1,
		convs:      make(map[uint64]*model.UserConversation),
	}
}

func (m *testJoinMsgRepo) GetTotalUnreadCount(userID uint64) (int64, error) {
	var total int64
	for _, c := range m.convs {
		if c.User1ID == userID {
			total += int64(c.User1UnreadCount)
		} else if c.User2ID == userID {
			total += int64(c.User2UnreadCount)
		}
	}
	return total, nil
}
func (m *testJoinMsgRepo) ListConversations(userID uint64) ([]model.UserConversation, error) {
	var out []model.UserConversation
	for _, c := range m.convs {
		if c.User1ID == userID || c.User2ID == userID {
			out = append(out, *c)
		}
	}
	return out, nil
}
func (m *testJoinMsgRepo) FindOrCreateConversation(user1ID, user2ID uint64) (*model.UserConversation, error) {
	for _, c := range m.convs {
		if c.User1ID == user1ID && c.User2ID == user2ID {
			return c, nil
		}
	}
	c := &model.UserConversation{
		ID:        m.nextConvID,
		User1ID:   user1ID,
		User2ID:   user2ID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.nextConvID++
	m.convs[c.ID] = c
	return c, nil
}
func (m *testJoinMsgRepo) FindConversationByID(convID uint64) (*model.UserConversation, error) {
	if c, ok := m.convs[convID]; ok {
		return c, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *testJoinMsgRepo) ListMessages(convID uint64, beforeID uint64, limit int) ([]model.UserMessage, bool, error) {
	var out []model.UserMessage
	for _, msg := range m.msgs {
		if msg.ConversationID == convID {
			out = append(out, msg)
		}
	}
	return out, false, nil
}
func (m *testJoinMsgRepo) CreateMessage(conv *model.UserConversation, msg *model.UserMessage, preview string) error {
	msg.ID = m.nextMsgID
	m.nextMsgID++
	m.msgs = append(m.msgs, *msg)
	conv.LastMessageID = msg.ID
	conv.LastMessagePreview = preview
	now := msg.CreatedAt
	conv.LastMessageAt = &now
	if msg.ReceiverID == conv.User1ID {
		conv.User1UnreadCount++
	} else {
		conv.User2UnreadCount++
	}
	return nil
}
func (m *testJoinMsgRepo) MarkConversationRead(convID, readerID uint64, readAt time.Time) error {
	return nil
}
func (m *testJoinMsgRepo) ListContacts(userID uint64) ([]model.ContactUserItem, error) {
	return nil, nil
}

type testJoinUserRepo struct {
	nextID uint64
	users  map[uint64]*model.SysUser
}

func newTestJoinUserRepo() *testJoinUserRepo {
	return &testJoinUserRepo{
		nextID: 1,
		users:  make(map[uint64]*model.SysUser),
	}
}

func (m *testJoinUserRepo) CreateWithRole(user *model.SysUser, roleCode string) error {
	user.ID = m.nextID
	m.nextID++
	cp := *user
	m.users[user.ID] = &cp
	return nil
}
func (m *testJoinUserRepo) FindByID(id uint64) (*model.SysUser, error) {
	if u, ok := m.users[id]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *testJoinUserRepo) FindByUsername(username string) (*model.SysUser, error) {
	for _, u := range m.users {
		if u.Username == username {
			cp := *u
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *testJoinUserRepo) FindByEmail(email string) (*model.SysUser, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *testJoinUserRepo) GetUserRoles(userID uint64) ([]string, error) {
	return []string{model.RoleDeveloper}, nil
}
func (m *testJoinUserRepo) UpdateProfile(userID uint64, nickname, avatar, bio string, favoritesPublic bool) error {
	return nil
}
func (m *testJoinUserRepo) UpdateEmail(userID uint64, email string, verified bool) error {
	return nil
}
func (m *testJoinUserRepo) UpdatePasswordAndTokenVersion(userID uint64, passwordHash string) error {
	return nil
}
func (m *testJoinUserRepo) IncrementTokenVersion(userID uint64) error          { return nil }
func (m *testJoinUserRepo) UpdateLastLogin(userID uint64, t time.Time) error { return nil }
func (m *testJoinUserRepo) SearchActiveUsers(keyword string, excludeUserID uint64, limit int) ([]model.UserBrief, error) {
	return []model.UserBrief{}, nil
}
func (m *testJoinUserRepo) FindBriefsByIDs(ids []uint64) (map[uint64]model.UserBrief, error) {
	res := make(map[uint64]model.UserBrief)
	for _, id := range ids {
		if u, ok := m.users[id]; ok {
			res[id] = model.ToUserBrief(u)
		}
	}
	return res, nil
}

func TestDomainJoinModesAndSystemNotice(t *testing.T) {
	userRepo := newTestJoinUserRepo()
	userRepo.users[1] = &model.SysUser{ID: 1, Username: "owner", Nickname: "Owner", Status: model.UserStatusActive}
	userRepo.users[2] = &model.SysUser{ID: 2, Username: "domadmin", Nickname: "DomAdmin", Status: model.UserStatusActive}
	userRepo.users[3] = &model.SysUser{ID: 3, Username: "applicant", Nickname: "Applicant", Status: model.UserStatusActive}
	userRepo.users[4] = &model.SysUser{ID: 4, Username: "quizuser", Nickname: "QuizUser", Status: model.UserStatusActive}
	userRepo.nextID = 5

	orgRepo := newMockOrgRepo()
	domainRepo := newMockSquareDomainRepo()
	govRepo := newMockP1GovRepo()
	govRepo.domainRepo = domainRepo
	msgRepo := newTestJoinMsgRepo()

	domainSvc := NewDomainService(domainRepo, userRepo, orgRepo)
	msgSvc := NewMessageService(msgRepo, userRepo)
	govSvc := NewGovernanceService(govRepo, &mockP1AssetRepo{assets: make(map[uint64]*model.AIAsset)}, domainRepo, orgRepo)
	govSvc.SetMessageService(msgSvc)

	// 1. Create domain without join_mode -> defaults to approval_only
	domItem, err := domainSvc.CreateDomain(1, model.CreateDomainRequest{
		DomainCode: "pub-join-test",
		DomainName: "Public Join Test",
		Visibility: model.DomainVisibilityPublic,
	})
	if err != nil {
		t.Fatalf("CreateDomain failed: %v", err)
	}
	if domItem.JoinMode != model.DomainJoinModeApprovalOnly {
		t.Fatalf("expected default join_mode %q, got %q", model.DomainJoinModeApprovalOnly, domItem.JoinMode)
	}
	if domItem.JoinQuestion != "" {
		t.Fatalf("expected empty join_question for approval_only, got %q", domItem.JoinQuestion)
	}

	// Verify BizDomain JSON serialization never exposes JoinAnswerHash
	domModel, _ := domainRepo.FindByID(domItem.ID)
	domModel.JoinAnswerHash = "$2a$10$secret"
	rawJSON, _ := json.Marshal(domModel)
	if strings.Contains(string(rawJSON), "secret") || strings.Contains(string(rawJSON), "join_answer_hash") {
		t.Fatalf("BizDomain JSON exposed answer hash: %s", string(rawJSON))
	}

	// Add user 2 as domain admin
	if err := domainSvc.AddMember(1, domItem.ID, model.AddDomainMemberRequest{
		UserID:     2,
		DomainRole: model.DomainRoleAdmin,
	}); err != nil {
		t.Fatalf("AddMember failed: %v", err)
	}

	// 2. Non-member applies to join public domain
	joinReq, err := govSvc.RequestJoinDomain(3, domItem.ID, "想学习域内知识库")
	if err != nil {
		t.Fatalf("RequestJoinDomain failed: %v", err)
	}
	if joinReq.TargetType != model.ApprovalTargetDomainJoin || joinReq.Reason != "想学习域内知识库" {
		t.Fatalf("unexpected join request: %+v", joinReq)
	}

	// Duplicate pending request -> 409
	var appErr *AppError
	if _, err := govSvc.RequestJoinDomain(3, domItem.ID, "dup"); !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate pending join, got %v", err)
	}

	// Domain admin (user 2) AND owner (user 1) can see pending request
	pendingAdmin, err := govSvc.ListPendingRequests(2, false)
	if err != nil || len(pendingAdmin) != 1 || pendingAdmin[0].ID != joinReq.ID {
		t.Fatalf("expected domain admin to see pending domain_join request, got %+v, err=%v", pendingAdmin, err)
	}
	pendingOwner, err := govSvc.ListPendingRequests(1, false)
	if err != nil || len(pendingOwner) != 1 || pendingOwner[0].ID != joinReq.ID {
		t.Fatalf("expected domain owner to see pending domain_join request, got %+v, err=%v", pendingOwner, err)
	}

	// Domain admin approves -> applicant becomes member + receives system_notice
	handled, err := govSvc.HandleApproval(2, false, joinReq.ID, model.HandleApprovalRequest{
		Action:  model.ApprovalActionApprove,
		Comment: "欢迎加入",
	})
	if err != nil {
		t.Fatalf("HandleApproval failed: %v", err)
	}
	if handled.Status != model.ApprovalStatusApproved {
		t.Fatalf("expected approved status, got %s", handled.Status)
	}
	role, err := domainRepo.GetMemberRole(domItem.ID, 3)
	if err != nil || role != model.DomainRoleMember {
		t.Fatalf("expected user 3 to become member, got role=%q err=%v", role, err)
	}
	if len(msgRepo.msgs) != 1 || msgRepo.msgs[0].MsgType != model.MessageTypeSystemNotice || msgRepo.msgs[0].ReceiverID != 3 {
		t.Fatalf("expected 1 system_notice message to user 3, got %+v", msgRepo.msgs)
	}

	// 3. Switch to approval_or_quiz
	updatedDom, err := domainSvc.UpdateJoinSettings(2, domItem.ID, model.UpdateDomainJoinSettingsRequest{
		JoinMode:     model.DomainJoinModeApprovalOrQuiz,
		JoinQuestion: "口令是什么？",
		JoinAnswer:   "  ASCP-2026 ",
	})
	if err != nil {
		t.Fatalf("UpdateJoinSettings failed: %v", err)
	}
	if updatedDom.JoinMode != model.DomainJoinModeApprovalOrQuiz || updatedDom.JoinQuestion != "口令是什么？" {
		t.Fatalf("unexpected updated domain join settings: %+v", updatedDom)
	}

	// Switch back to approval_only and then back to approval_or_quiz without re-entering answer (should keep old hash)
	if _, err := domainSvc.UpdateJoinSettings(2, domItem.ID, model.UpdateDomainJoinSettingsRequest{
		JoinMode: model.DomainJoinModeApprovalOnly,
	}); err != nil {
		t.Fatalf("switch back to approval_only failed: %v", err)
	}
	if _, err := domainSvc.UpdateJoinSettings(2, domItem.ID, model.UpdateDomainJoinSettingsRequest{
		JoinMode:     model.DomainJoinModeApprovalOrQuiz,
		JoinQuestion: "新问题：口令是什么？",
	}); err != nil {
		t.Fatalf("switch back to approval_or_quiz keeping history answer failed: %v", err)
	}

	// User 4 submits a pending join request first, then answers quiz
	pendingReq4, err := govSvc.RequestJoinDomain(4, domItem.ID, "先提交申请")
	if err != nil {
		t.Fatalf("RequestJoinDomain user 4 failed: %v", err)
	}

	// Wrong answer -> 400
	if err := govSvc.JoinDomainByQuiz(4, domItem.ID, "wrong"); !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 for wrong quiz answer, got %v", err)
	}

	// Correct answer (case-insensitive, trimmed) -> joins as member + auto-closes pending + sends system_notice
	if err := govSvc.JoinDomainByQuiz(4, domItem.ID, "ascp-2026"); err != nil {
		t.Fatalf("JoinDomainByQuiz failed: %v", err)
	}
	role4, _ := domainRepo.GetMemberRole(domItem.ID, 4)
	if role4 != model.DomainRoleMember {
		t.Fatalf("expected user 4 to become member, got %q", role4)
	}
	closedReq4, _ := govRepo.FindApprovalByID(pendingReq4.ID)
	if closedReq4.Status != model.ApprovalStatusApproved {
		t.Fatalf("expected pending request %d to be auto-approved, got %s", pendingReq4.ID, closedReq4.Status)
	}
	if len(msgRepo.msgs) != 2 || msgRepo.msgs[1].ReceiverID != 4 || msgRepo.msgs[1].MsgType != model.MessageTypeSystemNotice {
		t.Fatalf("expected system_notice for user 4, got %+v", msgRepo.msgs)
	}
}

func TestSystemUserCannotLoginOrReceiveDirectMessage(t *testing.T) {
	userRepo := newTestJoinUserRepo()
	hash, _ := password.Hash("SomePass123!")
	userRepo.users[1] = &model.SysUser{
		ID:           1,
		Username:     "system",
		PasswordHash: hash,
		Nickname:     "系统通知",
		Status:       model.UserStatusActive,
	}
	userRepo.users[2] = &model.SysUser{
		ID:       2,
		Username: "alice",
		Nickname: "Alice",
		Status:   model.UserStatusActive,
	}

	authSvc := NewAuthService(userRepo, config.JWTConfig{Secret: "test-secret", ExpireHours: 24})
	var appErr *AppError
	if _, err := authSvc.Login(model.LoginRequest{Username: "system", Password: "SomePass123!"}); !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusForbidden {
		t.Fatalf("expected 403 when system user tries to login, got %v", err)
	}

	msgSvc := NewMessageService(newTestJoinMsgRepo(), userRepo)
	if _, err := msgSvc.CreateOrGetConversation(2, 1); !errors.As(err, &appErr) || appErr.HTTPStatus != http.StatusBadRequest {
		t.Fatalf("expected 400 when user tries to start DM with system user, got %v", err)
	}
}
