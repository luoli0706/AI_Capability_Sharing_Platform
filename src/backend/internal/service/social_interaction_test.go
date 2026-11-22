package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"testing"
	"time"

	"ascp-backend/internal/config"
	"ascp-backend/internal/model"

	"gorm.io/gorm"
)

// --- In-memory mocks for social/messaging/interaction tests ---

type mockSocialUserRepo struct {
	users map[uint64]*model.SysUser
	roles map[uint64][]string
}

func (m *mockSocialUserRepo) CreateWithRole(user *model.SysUser, roleCode string) error { return nil }
func (m *mockSocialUserRepo) FindByID(id uint64) (*model.SysUser, error) {
	if u, ok := m.users[id]; ok {
		cp := *u
		return &cp, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockSocialUserRepo) FindByUsername(username string) (*model.SysUser, error) {
	for _, u := range m.users {
		if u.Username == username {
			cp := *u
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockSocialUserRepo) FindByEmail(email string) (*model.SysUser, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockSocialUserRepo) GetUserRoles(userID uint64) ([]string, error) {
	if m.roles != nil {
		if r, ok := m.roles[userID]; ok {
			return r, nil
		}
	}
	return []string{model.RoleDeveloper}, nil
}
func (m *mockSocialUserRepo) UpdateProfile(userID uint64, nickname string, email *string, avatar, bio string, favoritesPublic bool) error {
	u, ok := m.users[userID]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	u.Nickname = nickname
	u.Email = email
	u.Avatar = avatar
	u.Bio = bio
	u.FavoritesPublic = favoritesPublic
	return nil
}
func (m *mockSocialUserRepo) UpdatePasswordAndTokenVersion(userID uint64, passwordHash string) error {
	return nil
}
func (m *mockSocialUserRepo) IncrementTokenVersion(userID uint64) error          { return nil }
func (m *mockSocialUserRepo) UpdateLastLogin(userID uint64, t time.Time) error { return nil }
func (m *mockSocialUserRepo) SearchActiveUsers(keyword string, excludeUserID uint64, limit int) ([]model.UserBrief, error) {
	kw := strings.ToLower(strings.TrimSpace(keyword))
	var out []model.UserBrief
	for _, u := range m.users {
		if u.Status != model.UserStatusActive || u.ID == excludeUserID {
			continue
		}
		if kw == "" || strings.Contains(strings.ToLower(u.Username), kw) || strings.Contains(strings.ToLower(u.Nickname), kw) {
			out = append(out, model.ToUserBrief(u))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (m *mockSocialUserRepo) FindBriefsByIDs(ids []uint64) (map[uint64]model.UserBrief, error) {
	res := make(map[uint64]model.UserBrief)
	for _, id := range ids {
		if u, ok := m.users[id]; ok {
			res[id] = model.ToUserBrief(u)
		}
	}
	return res, nil
}

type mockMessageRepo struct {
	nextConvID uint64
	nextMsgID  uint64
	convs      map[uint64]*model.UserConversation
	msgs       []*model.UserMessage
}

func newMockMessageRepo() *mockMessageRepo {
	return &mockMessageRepo{
		nextConvID: 1,
		nextMsgID:  1,
		convs:      make(map[uint64]*model.UserConversation),
	}
}

func (m *mockMessageRepo) GetTotalUnreadCount(userID uint64) (int64, error) {
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

func (m *mockMessageRepo) ListConversations(userID uint64) ([]model.UserConversation, error) {
	var out []model.UserConversation
	for _, c := range m.convs {
		if c.User1ID == userID || c.User2ID == userID {
			out = append(out, *c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		var ti, tj time.Time
		if out[i].LastMessageAt != nil {
			ti = *out[i].LastMessageAt
		}
		if out[j].LastMessageAt != nil {
			tj = *out[j].LastMessageAt
		}
		if !ti.Equal(tj) {
			return ti.After(tj)
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (m *mockMessageRepo) FindOrCreateConversation(user1ID, user2ID uint64) (*model.UserConversation, error) {
	for _, c := range m.convs {
		if c.User1ID == user1ID && c.User2ID == user2ID {
			cp := *c
			return &cp, nil
		}
	}
	now := time.Now()
	c := &model.UserConversation{
		ID:        m.nextConvID,
		User1ID:   user1ID,
		User2ID:   user2ID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	m.nextConvID++
	m.convs[c.ID] = c
	cp := *c
	return &cp, nil
}

func (m *mockMessageRepo) FindConversationByID(convID uint64) (*model.UserConversation, error) {
	c, ok := m.convs[convID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *mockMessageRepo) ListMessages(convID uint64, beforeID uint64, limit int) ([]model.UserMessage, bool, error) {
	var filtered []model.UserMessage
	for _, msg := range m.msgs {
		if msg.ConversationID == convID && (beforeID == 0 || msg.ID < beforeID) {
			filtered = append(filtered, *msg)
		}
	}
	hasMore := len(filtered) > limit
	if hasMore {
		filtered = filtered[len(filtered)-limit:]
	}
	return filtered, hasMore, nil
}

func (m *mockMessageRepo) CreateMessage(conv *model.UserConversation, msg *model.UserMessage, preview string) error {
	msg.ID = m.nextMsgID
	m.nextMsgID++
	cp := *msg
	m.msgs = append(m.msgs, &cp)

	stored := m.convs[conv.ID]
	stored.LastMessageID = msg.ID
	stored.LastMessagePreview = preview
	t := msg.CreatedAt
	stored.LastMessageAt = &t
	stored.UpdatedAt = t
	if msg.ReceiverID == stored.User1ID {
		stored.User1UnreadCount++
	} else {
		stored.User2UnreadCount++
	}
	return nil
}

func (m *mockMessageRepo) MarkConversationRead(convID, readerID uint64, readAt time.Time) error {
	for _, msg := range m.msgs {
		if msg.ConversationID == convID && msg.ReceiverID == readerID && !msg.IsRead {
			msg.IsRead = true
			t := readAt
			msg.ReadAt = &t
		}
	}
	if c, ok := m.convs[convID]; ok {
		if c.User1ID == readerID {
			c.User1UnreadCount = 0
		} else if c.User2ID == readerID {
			c.User2UnreadCount = 0
		}
	}
	return nil
}

type mockKBInteractionRepo struct {
	nextID    uint64
	likes     []model.KBDocumentLike
	favorites []model.KBDocumentFavorite
	comments  []model.KBDocumentComment
	docs      map[uint64]model.KBDocument
	assets    map[uint64]model.AIAsset
	domains   map[uint64]model.BizDomain
}

func newMockKBInteractionRepo() *mockKBInteractionRepo {
	return &mockKBInteractionRepo{
		nextID:  1,
		docs:    make(map[uint64]model.KBDocument),
		assets:  make(map[uint64]model.AIAsset),
		domains: make(map[uint64]model.BizDomain),
	}
}

func (m *mockKBInteractionRepo) GetLikeStats(docID, userID uint64) (int64, bool, []uint64, error) {
	var uids []uint64
	liked := false
	for _, l := range m.likes {
		if l.DocID == docID {
			uids = append(uids, l.UserID)
			if l.UserID == userID {
				liked = true
			}
		}
	}
	return int64(len(uids)), liked, uids, nil
}

func (m *mockKBInteractionRepo) ToggleLike(kbAssetID, docID, userID uint64) (bool, error) {
	for i, l := range m.likes {
		if l.DocID == docID && l.UserID == userID {
			m.likes = append(m.likes[:i], m.likes[i+1:]...)
			return false, nil
		}
	}
	m.likes = append(m.likes, model.KBDocumentLike{
		ID:        m.nextID,
		KBAssetID: kbAssetID,
		DocID:     docID,
		UserID:    userID,
		CreatedAt: time.Now(),
	})
	m.nextID++
	return true, nil
}

func (m *mockKBInteractionRepo) IsFavorited(docID, userID uint64) (bool, error) {
	for _, f := range m.favorites {
		if f.DocID == docID && f.UserID == userID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockKBInteractionRepo) ToggleFavorite(kbAssetID, docID, userID uint64) (bool, error) {
	for i, f := range m.favorites {
		if f.DocID == docID && f.UserID == userID {
			m.favorites = append(m.favorites[:i], m.favorites[i+1:]...)
			return false, nil
		}
	}
	m.favorites = append(m.favorites, model.KBDocumentFavorite{
		ID:        m.nextID,
		KBAssetID: kbAssetID,
		DocID:     docID,
		UserID:    userID,
		CreatedAt: time.Now(),
	})
	m.nextID++
	return true, nil
}

func (m *mockKBInteractionRepo) ListUserFavorites(userID uint64) ([]model.UserFavoriteItem, error) {
	var out []model.UserFavoriteItem
	for _, f := range m.favorites {
		if f.UserID != userID {
			continue
		}
		doc, ok1 := m.docs[f.DocID]
		asset, ok2 := m.assets[f.KBAssetID]
		if !ok1 || !ok2 {
			continue
		}
		dom := m.domains[asset.DomainID]
		out = append(out, model.UserFavoriteItem{
			ID:               f.ID,
			KBAssetID:        f.KBAssetID,
			DocID:            f.DocID,
			DomainID:         asset.DomainID,
			DomainName:       dom.DomainName,
			KBName:           asset.Name,
			FileName:         doc.FileName,
			FolderPath:       doc.FolderPath,
			FileExt:          doc.FileExt,
			CreatedAt:        f.CreatedAt,
			DomainVisibility: dom.Visibility,
			AssetOwnerID:     asset.OwnerID,
			AssetVisibility:  asset.Visibility,
			AssetStatus:      asset.Status,
		})
	}
	if out == nil {
		out = []model.UserFavoriteItem{}
	}
	return out, nil
}

func (m *mockKBInteractionRepo) CountComments(docID uint64) (int64, error) {
	var count int64
	for _, c := range m.comments {
		if c.DocID == docID {
			count++
		}
	}
	return count, nil
}

func (m *mockKBInteractionRepo) ListComments(docID uint64) ([]model.KBDocumentComment, error) {
	var out []model.KBDocumentComment
	for _, c := range m.comments {
		if c.DocID == docID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *mockKBInteractionRepo) CreateComment(comment *model.KBDocumentComment) error {
	comment.ID = m.nextID
	m.nextID++
	now := time.Now()
	comment.CreatedAt = now
	comment.UpdatedAt = now
	m.comments = append(m.comments, *comment)
	return nil
}

func (m *mockKBInteractionRepo) FindCommentByID(kbAssetID, docID, commentID uint64) (*model.KBDocumentComment, error) {
	for _, c := range m.comments {
		if c.ID == commentID && c.KBAssetID == kbAssetID && c.DocID == docID {
			cp := c
			return &cp, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (m *mockKBInteractionRepo) DeleteComment(commentID uint64) error {
	for i, c := range m.comments {
		if c.ID == commentID {
			m.comments = append(m.comments[:i], m.comments[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockKBInteractionRepo) DeleteByDocIDs(docIDs []uint64) error {
	set := make(map[uint64]bool, len(docIDs))
	for _, id := range docIDs {
		set[id] = true
	}
	var likes []model.KBDocumentLike
	for _, l := range m.likes {
		if !set[l.DocID] {
			likes = append(likes, l)
		}
	}
	m.likes = likes

	var favs []model.KBDocumentFavorite
	for _, f := range m.favorites {
		if !set[f.DocID] {
			favs = append(favs, f)
		}
	}
	m.favorites = favs

	var comments []model.KBDocumentComment
	for _, c := range m.comments {
		if !set[c.DocID] {
			comments = append(comments, c)
		}
	}
	m.comments = comments
	return nil
}

func assertAppErrStatus(t *testing.T, err error, expectedStatus int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with status %d, got nil", expectedStatus)
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T: %v", err, err)
	}
	if appErr.HTTPStatus != expectedStatus {
		t.Fatalf("expected HTTPStatus %d, got %d (%s)", expectedStatus, appErr.HTTPStatus, appErr.Message)
	}
}

// --- Tests ---

func TestUserProfileAndFavoritesPrivacy(t *testing.T) {
	userRepo := &mockSocialUserRepo{
		users: map[uint64]*model.SysUser{
			1: {ID: 1, Username: "alice", Nickname: "Alice", Bio: "AI builder", Status: model.UserStatusActive, FavoritesPublic: false},
			2: {ID: 2, Username: "bob", Nickname: "Bob", Status: model.UserStatusActive, FavoritesPublic: false},
			3: {ID: 3, Username: "disabled_user", Nickname: "Disabled", Status: model.UserStatusDisabled},
		},
	}
	interRepo := newMockKBInteractionRepo()
	interRepo.domains[10] = model.BizDomain{ID: 10, DomainName: "Core AI", Visibility: model.DomainVisibilityPublic}
	interRepo.assets[20] = model.AIAsset{
		ID:         20,
		Name:       "Prompt KB",
		DomainID:   10,
		OwnerID:    1,
		AssetType:  model.AssetTypeKB,
		Visibility: model.VisibilityPublic,
		Status:     model.AssetStatusPublished,
	}
	interRepo.docs[30] = model.KBDocument{ID: 30, KBAssetID: 20, FileName: "guide.md", FileExt: ".md"}
	if _, err := interRepo.ToggleFavorite(20, 30, 1); err != nil {
		t.Fatalf("ToggleFavorite failed: %v", err)
	}

	domainRepo := &mockP1DomainRepo{
		domains: map[uint64]*model.BizDomain{
			10: {ID: 10, DomainName: "Core AI", Visibility: model.DomainVisibilityPublic},
		},
		members: map[uint64]map[uint64]string{
			10: {1: model.DomainRoleOwner},
		},
	}
	govRepo := &mockP1GovRepo{}

	authSvc := NewAuthService(userRepo, config.JWTConfig{Secret: "test", ExpireHours: 1})
	userSvc := NewUserService(userRepo, interRepo, domainRepo, nil, govRepo)

	// Search excludes self and disabled users
	results, err := userSvc.SearchUsers(1, "")
	if err != nil {
		t.Fatalf("SearchUsers failed: %v", err)
	}
	if len(results) != 1 || results[0].ID != 2 {
		t.Fatalf("expected only user 2 in search results, got %+v", results)
	}

	// Public profile 404 when missing
	_, err = userSvc.GetPublicProfile(999)
	assertAppErrStatus(t, err, http.StatusNotFound)

	// When favorites_public == false: owner sees favorites, other viewer sees is_public=false & empty list
	isPub, items, err := userSvc.GetUserFavorites(1, 1)
	if err != nil || !isPub || len(items) != 1 {
		t.Fatalf("owner should see favorites, got isPub=%v len=%d err=%v", isPub, len(items), err)
	}

	isPub, items, err = userSvc.GetUserFavorites(2, 1)
	if err != nil || isPub || len(items) != 0 {
		t.Fatalf("other viewer should see is_public=false and empty list, got isPub=%v len=%d err=%v", isPub, len(items), err)
	}

	// Toggle favorites_public to true via AuthService.UpdateProfile
	pubTrue := true
	updated, err := authSvc.UpdateProfile(1, model.UpdateProfileRequest{FavoritesPublic: &pubTrue})
	if err != nil || !updated.FavoritesPublic {
		t.Fatalf("expected favorites_public=true after UpdateProfile, got %+v err=%v", updated, err)
	}

	// Now other viewer can see favorites
	isPub, items, err = userSvc.GetUserFavorites(2, 1)
	if err != nil || !isPub || len(items) != 1 {
		t.Fatalf("other viewer should see public favorites, got isPub=%v len=%d err=%v", isPub, len(items), err)
	}
	if items[0].FileName != "guide.md" || items[0].DomainName != "Core AI" || items[0].KBName != "Prompt KB" {
		t.Fatalf("unexpected favorite item fields: %+v", items[0])
	}
}

type mockFavDomainRepo struct {
	*mockP1DomainRepo
}

func (m *mockFavDomainRepo) GetAccessibleDomainIDs(userID uint64, userOrgIDs, adminOrgIDs []uint64) ([]uint64, error) {
	var ids []uint64
	for domID, members := range m.members {
		if role := members[userID]; role != "" {
			ids = append(ids, domID)
		}
	}
	return ids, nil
}

func TestUserFavoritesDomainAndAssetVisibility(t *testing.T) {
	userRepo := &mockSocialUserRepo{
		users: map[uint64]*model.SysUser{
			1:  {ID: 1, Username: "alice", Nickname: "Alice", Status: model.UserStatusActive, FavoritesPublic: true},
			2:  {ID: 2, Username: "bob", Nickname: "Bob", Status: model.UserStatusActive},
			3:  {ID: 3, Username: "charlie", Nickname: "Charlie", Status: model.UserStatusActive},
			99: {ID: 99, Username: "admin", Nickname: "Admin", Status: model.UserStatusActive},
		},
		roles: map[uint64][]string{
			99: {model.RoleSuperAdmin},
		},
	}

	domainRepo := &mockFavDomainRepo{
		mockP1DomainRepo: &mockP1DomainRepo{
			domains: map[uint64]*model.BizDomain{
				10: {ID: 10, DomainName: "Secret Lab", Visibility: model.DomainVisibilityPrivate},
				11: {ID: 11, DomainName: "Open Commons", Visibility: model.DomainVisibilityPublic},
			},
			members: map[uint64]map[uint64]string{
				10: {
					1: model.DomainRoleOwner,
					3: model.DomainRoleMember,
				},
				11: {
					1: model.DomainRoleOwner,
				},
			},
		},
	}

	interRepo := newMockKBInteractionRepo()
	interRepo.domains[10] = *domainRepo.domains[10]
	interRepo.domains[11] = *domainRepo.domains[11]
	interRepo.assets[20] = model.AIAsset{
		ID: 20, Name: "Private Domain KB", DomainID: 10, OwnerID: 1,
		AssetType: model.AssetTypeKB, Visibility: model.VisibilityDomain, Status: model.AssetStatusPublished,
	}
	interRepo.assets[21] = model.AIAsset{
		ID: 21, Name: "Owner Only KB", DomainID: 10, OwnerID: 1,
		AssetType: model.AssetTypeKB, Visibility: model.VisibilityPrivate, Status: model.AssetStatusPublished,
	}
	interRepo.assets[22] = model.AIAsset{
		ID: 22, Name: "Public KB", DomainID: 11, OwnerID: 1,
		AssetType: model.AssetTypeKB, Visibility: model.VisibilityPublic, Status: model.AssetStatusPublished,
	}
	interRepo.docs[30] = model.KBDocument{ID: 30, KBAssetID: 20, FileName: "secret-domain.md", FileExt: ".md"}
	interRepo.docs[31] = model.KBDocument{ID: 31, KBAssetID: 21, FileName: "owner-private.md", FileExt: ".md"}
	interRepo.docs[32] = model.KBDocument{ID: 32, KBAssetID: 22, FileName: "public-guide.md", FileExt: ".md"}

	for _, pair := range [][2]uint64{{20, 30}, {21, 31}, {22, 32}} {
		if _, err := interRepo.ToggleFavorite(pair[0], pair[1], 1); err != nil {
			t.Fatalf("ToggleFavorite(%d, %d) failed: %v", pair[0], pair[1], err)
		}
	}

	userSvc := NewUserService(userRepo, interRepo, domainRepo, nil, &mockP1GovRepo{})

	// Non-member Bob must NOT see private domain documents, only public domain published public asset
	isPub, bobItems, err := userSvc.GetUserFavorites(2, 1)
	if err != nil || !isPub {
		t.Fatalf("expected isPub=true and nil err for Bob, got isPub=%v err=%v", isPub, err)
	}
	if len(bobItems) != 1 || bobItems[0].DocID != 32 {
		t.Fatalf("expected Bob to see only doc 32, got %+v", bobItems)
	}

	// Private domain member Charlie sees domain-visible doc 30 and public doc 32, but NOT owner-private doc 31
	isPub, charlieItems, err := userSvc.GetUserFavorites(3, 1)
	if err != nil || !isPub {
		t.Fatalf("expected isPub=true and nil err for Charlie, got isPub=%v err=%v", isPub, err)
	}
	if len(charlieItems) != 2 {
		t.Fatalf("expected Charlie to see 2 items (30, 32), got %+v", charlieItems)
	}
	for _, it := range charlieItems {
		if it.DocID == 31 {
			t.Fatalf("Charlie should not see owner-private doc 31")
		}
	}

	// Owner Alice sees all 3
	isPub, aliceItems, err := userSvc.GetUserFavorites(1, 1)
	if err != nil || !isPub || len(aliceItems) != 3 {
		t.Fatalf("expected Alice to see all 3 items, got isPub=%v len=%d err=%v", isPub, len(aliceItems), err)
	}

	// Super admin sees all 3
	isPub, adminItems, err := userSvc.GetUserFavorites(99, 1)
	if err != nil || !isPub || len(adminItems) != 3 {
		t.Fatalf("expected super admin to see all 3 items, got isPub=%v len=%d err=%v", isPub, len(adminItems), err)
	}
}

func TestDirectMessagingDisabledUserRejected(t *testing.T) {
	userRepo := &mockSocialUserRepo{
		users: map[uint64]*model.SysUser{
			1: {ID: 1, Username: "alice", Nickname: "Alice", Status: model.UserStatusActive},
			2: {ID: 2, Username: "bob", Nickname: "Bob", Status: model.UserStatusDisabled},
			3: {ID: 3, Username: "charlie", Nickname: "Charlie", Status: model.UserStatusActive},
		},
	}
	msgRepo := newMockMessageRepo()
	msgSvc := NewMessageService(msgRepo, userRepo)

	// Disabled target user cannot start a conversation
	_, err := msgSvc.CreateOrGetConversation(1, 2)
	assertAppErrStatus(t, err, http.StatusBadRequest)

	// Active target user can start conversation, but if later disabled, SendMessage is rejected
	conv, err := msgSvc.CreateOrGetConversation(1, 3)
	if err != nil {
		t.Fatalf("CreateOrGetConversation failed: %v", err)
	}
	userRepo.users[3].Status = model.UserStatusDisabled
	_, err = msgSvc.SendMessage(1, conv.ID, "Hi Charlie")
	assertAppErrStatus(t, err, http.StatusBadRequest)
}

func TestDirectMessagingLifecycle(t *testing.T) {
	userRepo := &mockSocialUserRepo{
		users: map[uint64]*model.SysUser{
			1: {ID: 1, Username: "alice", Nickname: "Alice", Status: model.UserStatusActive},
			2: {ID: 2, Username: "bob", Nickname: "Bob", Status: model.UserStatusActive},
			3: {ID: 3, Username: "charlie", Nickname: "Charlie", Status: model.UserStatusActive},
		},
	}
	msgRepo := newMockMessageRepo()
	msgSvc := NewMessageService(msgRepo, userRepo)

	// Prevent self-chat
	_, err := msgSvc.CreateOrGetConversation(1, 1)
	assertAppErrStatus(t, err, http.StatusBadRequest)

	// 404 if target user missing
	_, err = msgSvc.CreateOrGetConversation(1, 999)
	assertAppErrStatus(t, err, http.StatusNotFound)

	// Create conversation between 1 and 2
	conv, err := msgSvc.CreateOrGetConversation(2, 1)
	if err != nil || conv.Peer.ID != 1 {
		t.Fatalf("expected conversation with peer 1, got %+v err=%v", conv, err)
	}

	// Send two messages from 1 -> 2
	if _, err := msgSvc.SendMessage(1, conv.ID, "Hello Bob!"); err != nil {
		t.Fatalf("SendMessage 1 failed: %v", err)
	}
	if _, err := msgSvc.SendMessage(1, conv.ID, "How is the KB?"); err != nil {
		t.Fatalf("SendMessage 2 failed: %v", err)
	}

	// Unread count for user 2 should be 2, for user 1 should be 0
	unread2, err := msgSvc.GetUnreadCount(2)
	if err != nil || unread2 != 2 {
		t.Fatalf("expected unread2=2, got %d err=%v", unread2, err)
	}
	unread1, err := msgSvc.GetUnreadCount(1)
	if err != nil || unread1 != 0 {
		t.Fatalf("expected unread1=0, got %d err=%v", unread1, err)
	}

	// Non-participant (user 3) gets 403
	_, _, err = msgSvc.ListMessages(3, conv.ID, 0, 50)
	assertAppErrStatus(t, err, http.StatusForbidden)

	// User 2 lists messages -> marks as read, resets unread count, returns chronological ascending
	msgs, hasMore, err := msgSvc.ListMessages(2, conv.ID, 0, 50)
	if err != nil || hasMore || len(msgs) != 2 {
		t.Fatalf("expected 2 messages with hasMore=false, got len=%d hasMore=%v err=%v", len(msgs), hasMore, err)
	}
	if msgs[0].Content != "Hello Bob!" || msgs[1].Content != "How is the KB?" {
		t.Fatalf("unexpected message order: %+v", msgs)
	}

	unread2After, err := msgSvc.GetUnreadCount(2)
	if err != nil || unread2After != 0 {
		t.Fatalf("expected unread2After=0, got %d err=%v", unread2After, err)
	}
}

func TestKBInteractionsAndUploaderEnrichment(t *testing.T) {
	userRepo := &mockSocialUserRepo{
		users: map[uint64]*model.SysUser{
			1: {ID: 1, Username: "owner", Nickname: "Owner", Status: model.UserStatusActive},
			2: {ID: 2, Username: "alice", Nickname: "Alice", Status: model.UserStatusActive},
			3: {ID: 3, Username: "bob", Nickname: "Bob", Status: model.UserStatusActive},
		},
	}
	domainRepo := &mockP1DomainRepo{
		domains: map[uint64]*model.BizDomain{
			10: {ID: 10, DomainName: "Platform", Visibility: model.DomainVisibilityPublic},
		},
		members: map[uint64]map[uint64]string{
			10: {
				1: model.DomainRoleOwner,
				2: model.DomainRoleMember,
				3: model.DomainRoleMember,
			},
		},
	}
	assetRepo := &mockP1AssetRepo{
		assets: map[uint64]*model.AIAsset{
			20: {
				ID:         20,
				AssetCode:  "kb-test",
				AssetType:  model.AssetTypeKB,
				Name:       "Docs",
				DomainID:   10,
				OwnerID:    1,
				Visibility: model.VisibilityPublic,
				Status:     model.AssetStatusPublished,
			},
		},
	}
	kbRepo := newMockFileKBRepo()
	kbRepo.docs[100] = &model.KBDocument{
		ID:         100,
		KBAssetID:  20,
		FileName:   "readme.md",
		FileExt:    ".md",
		UploaderID: 2,
	}

	interRepo := newMockKBInteractionRepo()
	interSvc := NewKBInteractionService(interRepo, kbRepo, assetRepo, domainRepo, nil, nil, userRepo)
	kbSvc := NewKBService(kbRepo, assetRepo, domainRepo, nil, nil, t.TempDir(), "secret")
	kbSvc.SetAuditRepos(nil, userRepo)
	kbSvc.SetInteractionRepo(interRepo)

	// Uploader enrichment in ListDocuments & ListDocumentsLazy
	docs, err := kbSvc.ListDocuments(2, false, 20)
	if err != nil || len(docs) != 1 || docs[0].Uploader == nil || docs[0].Uploader.Username != "alice" {
		t.Fatalf("expected uploader alice in ListDocuments, got %+v err=%v", docs, err)
	}

	lazyDocs, err := kbSvc.ListDocumentsLazy(2, false, 20, "")
	if err != nil || len(lazyDocs) != 1 || lazyDocs[0].Uploader == nil || lazyDocs[0].Uploader.Nickname != "Alice" {
		t.Fatalf("expected uploader Alice in ListDocumentsLazy, got %+v err=%v", lazyDocs, err)
	}

	// Like visibility: user 2 likes, user 3 sees like_count=1 and like_users=[alice]
	likeRes, err := interSvc.ToggleLike(2, false, 20, 100)
	if err != nil || likeRes.LikeCount != 1 || !likeRes.LikedByMe {
		t.Fatalf("unexpected ToggleLike response: %+v err=%v", likeRes, err)
	}

	// Favorite invisibility: user 2 favorites, user 3 sees favorited_by_me=false and NO favorite_count/favorite_users in JSON
	fav, err := interSvc.ToggleFavorite(2, false, 20, 100)
	if err != nil || !fav {
		t.Fatalf("expected favorited=true, got %v err=%v", fav, err)
	}

	interForBob, err := interSvc.GetInteractions(3, false, 20, 100)
	if err != nil {
		t.Fatalf("GetInteractions failed: %v", err)
	}
	if interForBob.LikeCount != 1 || interForBob.LikedByMe || len(interForBob.LikeUsers) != 1 || interForBob.LikeUsers[0].Username != "alice" || interForBob.FavoritedByMe {
		t.Fatalf("unexpected interactions for bob: %+v", interForBob)
	}

	rawJSON, err := json.Marshal(interForBob)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}
	if strings.Contains(string(rawJSON), "favorite_count") || strings.Contains(string(rawJSON), "favorite_users") {
		t.Fatalf("interactions JSON must not expose favorite_count or favorite_users: %s", string(rawJSON))
	}

	// Comments: user 2 creates annotation with quote_text, user 3 cannot delete, domain owner (user 1) can delete
	cm, err := interSvc.CreateComment(2, false, 20, 100, model.CreateKBCommentRequest{
		QuoteText: "important line",
		Content:   "Please clarify this section",
	})
	if err != nil || cm.QuoteText != "important line" || cm.Author.Username != "alice" {
		t.Fatalf("unexpected comment: %+v err=%v", cm, err)
	}

	err = interSvc.DeleteComment(3, false, 20, 100, cm.ID)
	assertAppErrStatus(t, err, http.StatusForbidden)

	if err := interSvc.DeleteComment(1, false, 20, 100, cm.ID); err != nil {
		t.Fatalf("domain owner should be able to delete comment, got err=%v", err)
	}

	// Cleanup on DeleteDocument
	if err := kbSvc.DeleteDocument(1, false, 20, 100); err != nil {
		t.Fatalf("DeleteDocument failed: %v", err)
	}
	likeCount, _, _, err := interRepo.GetLikeStats(100, 2)
	if err != nil || likeCount != 0 {
		t.Fatalf("expected likes cleaned up after DeleteDocument, got %d err=%v", likeCount, err)
	}
	isFav, err := interRepo.IsFavorited(100, 2)
	if err != nil || isFav {
		t.Fatalf("expected favorites cleaned up after DeleteDocument, got %v err=%v", isFav, err)
	}
}
