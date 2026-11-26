package service

import (
	"errors"
	"net/http"
	"sort"
	"testing"
	"time"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

// --- In-memory mocks for DomainEventService tests ---

type mockDERepo struct {
	nextEventID uint64
	nextCandID  uint64
	events      map[uint64]*model.DomainEvent
	candidates  map[uint64]*model.DomainEventCandidate
	votes       []model.DomainEventVote
}

func newMockDERepo() *mockDERepo {
	return &mockDERepo{
		nextEventID: 1,
		nextCandID:  1,
		events:      make(map[uint64]*model.DomainEvent),
		candidates:  make(map[uint64]*model.DomainEventCandidate),
	}
}

func (m *mockDERepo) CreateEvent(event *model.DomainEvent) error {
	event.ID = m.nextEventID
	m.nextEventID++
	now := time.Now()
	event.CreatedAt = now
	event.UpdatedAt = now
	cp := *event
	m.events[event.ID] = &cp
	return nil
}

func (m *mockDERepo) FindEventByID(domainID, eventID uint64) (*model.DomainEvent, error) {
	ev, ok := m.events[eventID]
	if !ok || ev.DomainID != domainID {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *ev
	return &cp, nil
}

func (m *mockDERepo) ListEventsByDomain(domainID uint64) ([]model.DomainEvent, error) {
	var out []model.DomainEvent
	for _, ev := range m.events {
		if ev.DomainID == domainID {
			out = append(out, *ev)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID > out[j].ID })
	return out, nil
}

func (m *mockDERepo) CloseEvent(domainID, eventID uint64, closedAt time.Time) error {
	ev, ok := m.events[eventID]
	if !ok || ev.DomainID != domainID {
		return gorm.ErrRecordNotFound
	}
	t := closedAt
	ev.ClosedAt = &t
	return nil
}

func (m *mockDERepo) DeleteEvent(domainID, eventID uint64) error {
	ev, ok := m.events[eventID]
	if !ok || ev.DomainID != domainID {
		return gorm.ErrRecordNotFound
	}
	delete(m.events, eventID)
	return nil
}

func (m *mockDERepo) ListCandidatesByEvent(eventID uint64) ([]model.DomainEventCandidate, error) {
	var out []model.DomainEventCandidate
	for _, c := range m.candidates {
		if c.EventID == eventID {
			out = append(out, *c)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].VoteCount != out[j].VoteCount {
			return out[i].VoteCount > out[j].VoteCount
		}
		return out[i].ID < out[j].ID
	})
	return out, nil
}

func (m *mockDERepo) GetEventCandidateAndVoteStats(eventIDs []uint64) (map[uint64]int64, map[uint64]int64, error) {
	candCount := make(map[uint64]int64)
	voteSum := make(map[uint64]int64)
	set := make(map[uint64]bool, len(eventIDs))
	for _, id := range eventIDs {
		set[id] = true
	}
	for _, c := range m.candidates {
		if set[c.EventID] {
			candCount[c.EventID]++
			voteSum[c.EventID] += c.VoteCount
		}
	}
	return candCount, voteSum, nil
}

func (m *mockDERepo) FindCandidateByID(eventID, candidateID uint64) (*model.DomainEventCandidate, error) {
	c, ok := m.candidates[candidateID]
	if !ok || c.EventID != eventID {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *c
	return &cp, nil
}

func (m *mockDERepo) AddCandidateWithInitialVote(eventID uint64, allowMultiVote bool, candidate *model.DomainEventCandidate, userID uint64) error {
	for _, c := range m.candidates {
		if c.EventID == eventID && c.KBAssetID == candidate.KBAssetID && c.FolderDocID == candidate.FolderDocID {
			return repository.ErrCandidateExists
		}
	}
	if !allowMultiVote {
		for _, v := range m.votes {
			if v.EventID == eventID && v.UserID == userID {
				return repository.ErrAlreadyVotedInEvent
			}
		}
	}
	candidate.ID = m.nextCandID
	m.nextCandID++
	candidate.EventID = eventID
	candidate.AddedByUserID = userID
	candidate.VoteCount = 1
	cp := *candidate
	m.candidates[candidate.ID] = &cp
	m.votes = append(m.votes, model.DomainEventVote{
		EventID:     eventID,
		CandidateID: candidate.ID,
		UserID:      userID,
	})
	return nil
}

func (m *mockDERepo) CastVote(eventID, candidateID, userID uint64, allowMultiVote bool) error {
	cand, ok := m.candidates[candidateID]
	if !ok || cand.EventID != eventID {
		return gorm.ErrRecordNotFound
	}
	if !allowMultiVote {
		for _, v := range m.votes {
			if v.EventID == eventID && v.UserID == userID {
				return repository.ErrAlreadyVotedInEvent
			}
		}
	} else {
		for _, v := range m.votes {
			if v.EventID == eventID && v.CandidateID == candidateID && v.UserID == userID {
				return repository.ErrAlreadyVotedCandidate
			}
		}
	}
	m.votes = append(m.votes, model.DomainEventVote{
		EventID:     eventID,
		CandidateID: candidateID,
		UserID:      userID,
	})
	cand.VoteCount++
	return nil
}

func (m *mockDERepo) CancelVote(eventID, candidateID, userID uint64) error {
	idx := -1
	for i, v := range m.votes {
		if v.EventID == eventID && v.CandidateID == candidateID && v.UserID == userID {
			idx = i
			break
		}
	}
	if idx == -1 {
		return gorm.ErrRecordNotFound
	}
	m.votes = append(m.votes[:idx], m.votes[idx+1:]...)
	if cand, ok := m.candidates[candidateID]; ok && cand.VoteCount > 0 {
		cand.VoteCount--
	}
	return nil
}

func (m *mockDERepo) ListUserVotedCandidateIDs(eventID, userID uint64) ([]uint64, error) {
	ids := []uint64{}
	for _, v := range m.votes {
		if v.EventID == eventID && v.UserID == userID {
			ids = append(ids, v.CandidateID)
		}
	}
	return ids, nil
}

type mockDEDomainRepo struct {
	domains map[uint64]*model.BizDomain
	roles   map[uint64]map[uint64]string
}

func (m *mockDEDomainRepo) ListDomains(userID uint64, q model.DomainQuery, userOrgIDs, adminOrgIDs []uint64) ([]model.DomainListItem, int64, error) {
	return nil, 0, nil
}
func (m *mockDEDomainRepo) FindByID(id uint64) (*model.BizDomain, error) {
	d, ok := m.domains[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return d, nil
}
func (m *mockDEDomainRepo) FindByCode(code string) (*model.BizDomain, error) { return nil, gorm.ErrRecordNotFound }
func (m *mockDEDomainRepo) FindByUID(uid string) (*model.BizDomain, error)   { return nil, gorm.ErrRecordNotFound }
func (m *mockDEDomainRepo) CreateWithOwner(domain *model.BizDomain) error    { return nil }
func (m *mockDEDomainRepo) UpdateDomain(domain *model.BizDomain) error       { return nil }
func (m *mockDEDomainRepo) DeleteDomain(id uint64) error                     { return nil }
func (m *mockDEDomainRepo) ListMembers(domainID uint64) ([]model.DomainMemberItem, error) {
	return nil, nil
}
func (m *mockDEDomainRepo) GetMemberRole(domainID, userID uint64) (string, error) {
	if dm, ok := m.roles[domainID]; ok {
		return dm[userID], nil
	}
	return "", nil
}
func (m *mockDEDomainRepo) UpsertMember(member *model.DomainMember) error          { return nil }
func (m *mockDEDomainRepo) RemoveMember(domainID, userID uint64) error             { return nil }
func (m *mockDEDomainRepo) GetUserDomainIDs(userID uint64) ([]uint64, error)       { return nil, nil }
func (m *mockDEDomainRepo) GetUserAdminDomainIDs(userID uint64) ([]uint64, error)  { return nil, nil }
func (m *mockDEDomainRepo) GetAccessibleDomainIDs(userID uint64, userOrgIDs, adminOrgIDs []uint64) ([]uint64, error) {
	return nil, nil
}

type mockDEAssetRepo struct {
	assets map[uint64]*model.AssetDetail
}

func (m *mockDEAssetRepo) ListAssets(q model.AssetQuery, userID uint64, isSuperAdmin bool, memberDomainIDs, adminDomainIDs, permittedAssetIDs []uint64) ([]model.AssetListItem, int64, error) {
	return nil, 0, nil
}
func (m *mockDEAssetRepo) ListUserAssets(q model.AssetQuery) ([]model.AssetListItem, int64, error) {
	return nil, 0, nil
}
func (m *mockDEAssetRepo) FindByID(id uint64) (*model.AIAsset, error) { return nil, gorm.ErrRecordNotFound }
func (m *mockDEAssetRepo) FindByCode(code string) (*model.AIAsset, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockDEAssetRepo) GetDetailByID(id uint64) (*model.AssetDetail, error) {
	a, ok := m.assets[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *a
	return &cp, nil
}
func (m *mockDEAssetRepo) GetDetailByCode(code string) (*model.AssetDetail, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockDEAssetRepo) CreateAsset(asset *model.AIAsset, ext interface{}) error { return nil }
func (m *mockDEAssetRepo) UpdateAsset(asset *model.AIAsset, ext interface{}) error { return nil }
func (m *mockDEAssetRepo) DeleteAsset(id uint64) error                             { return nil }
func (m *mockDEAssetRepo) UpdateStatus(id uint64, status string, version string) error {
	return nil
}
func (m *mockDEAssetRepo) PublishWithSnapshot(assetID uint64, version, changelog string, snapshot model.JSONRaw, publisherID uint64) error {
	return nil
}
func (m *mockDEAssetRepo) SubmitPublishApproval(assetID, domainID uint64, version, changelog string, snapshot model.JSONRaw, applicantID, approverID uint64, requestNo string) error {
	return nil
}
func (m *mockDEAssetRepo) ListVersions(assetID uint64) ([]model.AIAssetVersion, error) { return nil, nil }
func (m *mockDEAssetRepo) FindVersionByID(assetID, versionID uint64) (*model.AIAssetVersion, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockDEAssetRepo) RestoreFromSnapshot(asset *model.AIAsset, ext interface{}, versionID uint64, version string) error {
	return nil
}
func (m *mockDEAssetRepo) UpdateMCPHealth(assetID uint64, healthStatus string, checkedAt time.Time) error {
	return nil
}
func (m *mockDEAssetRepo) UpdateMCPTools(assetID uint64, toolsSchema model.JSONRaw) error { return nil }
func (m *mockDEAssetRepo) UpdateSkillPackage(assetID uint64, packagePath string, packageFiles model.StringSlice, content string) error {
	return nil
}
func (m *mockDEAssetRepo) IncrementInvokeCount(assetID uint64) error { return nil }

type mockDEKBRepo struct {
	docs map[uint64]*model.KBDocument
}

func (m *mockDEKBRepo) CreateDocument(doc *model.KBDocument) error { return nil }
func (m *mockDEKBRepo) EnsureFolderHierarchy(kbAssetID, uploaderID uint64, folderPath string) (*model.KBDocument, error) {
	return nil, nil
}
func (m *mockDEKBRepo) ListDocuments(kbAssetID uint64) ([]model.KBDocument, error) { return nil, nil }
func (m *mockDEKBRepo) ListDocumentsByParent(kbAssetID uint64, parentFolder string) ([]model.KBDocument, error) {
	return nil, nil
}
func (m *mockDEKBRepo) ListDocumentsByParentPaged(kbAssetID uint64, parentFolder string, offset, limit int, extGroup string) ([]model.KBDocument, int64, error) {
	return nil, 0, nil
}
func (m *mockDEKBRepo) GetFolderSummary(kbAssetID uint64, parentFolder string, extGroup string) (int64, time.Time, error) {
	return 0, time.Time{}, nil
}
func (m *mockDEKBRepo) FindDocumentByID(kbAssetID, docID uint64) (*model.KBDocument, error) {
	d, ok := m.docs[docID]
	if !ok || d.KBAssetID != kbAssetID {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *d
	return &cp, nil
}
func (m *mockDEKBRepo) FindDeletedDocumentByID(kbAssetID, docID uint64) (*model.KBDocument, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockDEKBRepo) ListMatchingFiles(kbAssetID uint64, scope string, docID uint64, folderPath string) ([]model.KBDocument, error) {
	return nil, nil
}
func (m *mockDEKBRepo) ListSubtreeDocuments(kbAssetID uint64, folderPath string) ([]model.KBDocument, error) {
	return nil, nil
}
func (m *mockDEKBRepo) FileExistsInFolder(kbAssetID uint64, folderPath, fileName string, excludeDocID uint64) (bool, error) {
	return false, nil
}
func (m *mockDEKBRepo) FolderExists(kbAssetID uint64, folderPath string, excludeDocID uint64) (bool, error) {
	return false, nil
}
func (m *mockDEKBRepo) RenameFile(kbAssetID, docID uint64, newName, newExt string) error { return nil }
func (m *mockDEKBRepo) RenameFolder(kbAssetID, folderDocID uint64, oldPath, newName, newPath string) error {
	return nil
}
func (m *mockDEKBRepo) MoveDocument(kbAssetID, docID uint64, targetFolder string) error { return nil }
func (m *mockDEKBRepo) ReplaceDocumentChunks(kbAssetID uint64, doc *model.KBDocument, chunks []model.KBChunk) error {
	return nil
}
func (m *mockDEKBRepo) CompleteDocumentParsing(kbAssetID, docID uint64, chunks []model.KBChunk) error {
	return nil
}
func (m *mockDEKBRepo) FailDocumentParsing(docID uint64, errMsg string) error { return nil }
func (m *mockDEKBRepo) RecalculateKBCounts(kbAssetID uint64) error            { return nil }
func (m *mockDEKBRepo) ListChunks(kbAssetID, docID uint64, page, pageSize int) ([]model.KBChunk, int64, error) {
	return nil, 0, nil
}
func (m *mockDEKBRepo) DeleteDocument(kbAssetID, docID uint64) error { return nil }
func (m *mockDEKBRepo) DeleteFolderRecursive(kbAssetID uint64, folderPath string) ([]string, error) {
	return nil, nil
}
func (m *mockDEKBRepo) RestoreDocument(kbAssetID, docID uint64) error { return nil }
func (m *mockDEKBRepo) RestoreFolderRecursive(kbAssetID, folderDocID uint64, folderPath string, deletedAt time.Time) error {
	return nil
}
func (m *mockDEKBRepo) SearchChunks(kbAssetID uint64, query string, topK int) ([]model.KBSearchHit, error) {
	return nil, nil
}
func (m *mockDEKBRepo) FindDocumentByFilePath(filePath string) (*model.KBDocument, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockDEKBRepo) UpdateStorageSyncState(docID uint64, tier, ossKey string, syncedAt, lastAccessedAt *time.Time) error {
	return nil
}
func (m *mockDEKBRepo) TouchLastAccessed(docID uint64, accessedAt time.Time) error { return nil }
func (m *mockDEKBRepo) ListUnsyncedLocalDocs(limit int) ([]model.KBDocument, error) {
	return nil, nil
}
func (m *mockDEKBRepo) ListEvictableSyncedDocs(cutoff time.Time, limit int) ([]model.KBDocument, error) {
	return nil, nil
}
func (m *mockDEKBRepo) ListLRUSyncedDocs(limit int) ([]model.KBDocument, error) { return nil, nil }
func (m *mockDEKBRepo) TryMarkOSSOnly(docID uint64, cutoff *time.Time) (bool, error) {
	return false, nil
}

type mockDEUserRepo struct {
	users map[uint64]*model.SysUser
}

func (m *mockDEUserRepo) CreateWithRole(user *model.SysUser, roleCode string) error { return nil }
func (m *mockDEUserRepo) FindByID(id uint64) (*model.SysUser, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (m *mockDEUserRepo) FindByUsername(username string) (*model.SysUser, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockDEUserRepo) FindByEmail(email string) (*model.SysUser, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockDEUserRepo) GetUserRoles(userID uint64) ([]string, error) {
	return []string{model.RoleDeveloper}, nil
}
func (m *mockDEUserRepo) UpdateProfile(userID uint64, nickname, avatar, bio string, favoritesPublic bool) error {
	return nil
}
func (m *mockDEUserRepo) UpdateEmail(userID uint64, email string, verified bool) error { return nil }
func (m *mockDEUserRepo) UpdatePasswordAndTokenVersion(userID uint64, passwordHash string) error {
	return nil
}
func (m *mockDEUserRepo) IncrementTokenVersion(userID uint64) error           { return nil }
func (m *mockDEUserRepo) UpdateLastLogin(userID uint64, t time.Time) error    { return nil }
func (m *mockDEUserRepo) SearchActiveUsers(keyword string, excludeUserID uint64, limit int) ([]model.UserBrief, error) {
	return nil, nil
}
func (m *mockDEUserRepo) FindBriefsByIDs(ids []uint64) (map[uint64]model.UserBrief, error) {
	out := make(map[uint64]model.UserBrief)
	for _, id := range ids {
		if u, ok := m.users[id]; ok {
			out[id] = model.ToUserBrief(u)
		}
	}
	return out, nil
}

func setupDomainEventTestEnv() (DomainEventService, *mockDERepo) {
	eventRepo := newMockDERepo()
	domainRepo := &mockDEDomainRepo{
		domains: map[uint64]*model.BizDomain{
			1: {ID: 1, DomainCode: "d1", DomainName: "Domain 1", Visibility: model.DomainVisibilityPublic},
			2: {ID: 2, DomainCode: "d2", DomainName: "Domain 2", Visibility: model.DomainVisibilityPublic},
		},
		roles: map[uint64]map[uint64]string{
			1: {
				10: model.DomainRoleOwner,
				11: model.DomainRoleAdmin,
				12: model.DomainRoleMember,
				13: model.DomainRoleMember,
			},
			2: {
				20: model.DomainRoleOwner,
			},
		},
	}
	assetRepo := &mockDEAssetRepo{
		assets: map[uint64]*model.AssetDetail{
			100: {AIAsset: model.AIAsset{ID: 100, AssetType: model.AssetTypeKB, Name: "Domain 1 KB", DomainID: 1}},
			101: {AIAsset: model.AIAsset{ID: 101, AssetType: model.AssetTypeSkill, Name: "Domain 1 Skill", DomainID: 1}},
			200: {AIAsset: model.AIAsset{ID: 200, AssetType: model.AssetTypeKB, Name: "Domain 2 KB", DomainID: 2}},
		},
	}
	kbRepo := &mockDEKBRepo{
		docs: map[uint64]*model.KBDocument{
			1001: {ID: 1001, KBAssetID: 100, IsFolder: true, FolderPath: "proposals/alpha", FileName: "alpha"},
			1002: {ID: 1002, KBAssetID: 100, IsFolder: true, FolderPath: "proposals/beta", FileName: "beta"},
			1003: {ID: 1003, KBAssetID: 100, IsFolder: false, FolderPath: "proposals", FileName: "readme.md"},
			2001: {ID: 2001, KBAssetID: 200, IsFolder: true, FolderPath: "other/folder", FileName: "folder"},
		},
	}
	userRepo := &mockDEUserRepo{
		users: map[uint64]*model.SysUser{
			10: {ID: 10, Username: "owner1", Nickname: "Owner One"},
			11: {ID: 11, Username: "admin1", Nickname: "Admin One"},
			12: {ID: 12, Username: "member1", Nickname: "Member One"},
			13: {ID: 13, Username: "member2", Nickname: "Member Two"},
		},
	}

	svc := NewDomainEventService(eventRepo, domainRepo, nil, assetRepo, kbRepo, userRepo)
	return svc, eventRepo
}

func assertAppErrorStatus(t *testing.T, err error, wantStatus int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with status %d, got nil", wantStatus)
	}
	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected *AppError, got %T (%v)", err, err)
	}
	if appErr.HTTPStatus != wantStatus {
		t.Fatalf("expected HTTP status %d, got %d (msg=%q)", wantStatus, appErr.HTTPStatus, appErr.Message)
	}
}

func TestDomainEvent_CreateAndTypeValidation(t *testing.T) {
	svc, _ := setupDomainEventTestEnv()
	now := time.Now()

	// Default event type -> folder_vote
	ev, err := svc.CreateEvent(12, false, 1, model.CreateDomainEventRequest{
		Title:     "Q4 Proposal Vote",
		StartTime: now.Add(-time.Hour),
		EndTime:   now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}
	if ev.EventType != model.DomainEventTypeFolderVote {
		t.Fatalf("expected default event_type folder_vote, got %q", ev.EventType)
	}
	if ev.Status != model.DomainEventStatusActive {
		t.Fatalf("expected active status, got %q", ev.Status)
	}

	// Reject lottery & matching
	for _, et := range []string{model.DomainEventTypeLottery, model.DomainEventTypeMatching} {
		_, err := svc.CreateEvent(12, false, 1, model.CreateDomainEventRequest{
			Title:     "Unsupported",
			EventType: et,
			StartTime: now,
			EndTime:   now.Add(time.Hour),
		})
		assertAppErrorStatus(t, err, http.StatusBadRequest)
	}

	// Reject end_time <= start_time
	_, err = svc.CreateEvent(12, false, 1, model.CreateDomainEventRequest{
		Title:     "Bad Window",
		StartTime: now.Add(time.Hour),
		EndTime:   now.Add(time.Hour),
	})
	assertAppErrorStatus(t, err, http.StatusBadRequest)
}

func TestDomainEvent_TimeWindowEnforcement(t *testing.T) {
	svc, _ := setupDomainEventTestEnv()
	now := time.Now()

	upcoming, err := svc.CreateEvent(10, false, 1, model.CreateDomainEventRequest{
		Title:     "Upcoming Vote",
		StartTime: now.Add(time.Hour),
		EndTime:   now.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("create upcoming failed: %v", err)
	}
	if upcoming.Status != model.DomainEventStatusUpcoming {
		t.Fatalf("expected upcoming, got %q", upcoming.Status)
	}

	_, err = svc.AddCandidate(12, false, 1, upcoming.ID, model.AddDomainEventCandidateRequest{
		KBAssetID:   100,
		FolderDocID: 1001,
	})
	assertAppErrorStatus(t, err, http.StatusBadRequest)

	ended, err := svc.CreateEvent(10, false, 1, model.CreateDomainEventRequest{
		Title:     "Ended Vote",
		StartTime: now.Add(-2 * time.Hour),
		EndTime:   now.Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("create ended failed: %v", err)
	}
	if ended.Status != model.DomainEventStatusEnded {
		t.Fatalf("expected ended, got %q", ended.Status)
	}

	_, err = svc.VoteCandidate(12, false, 1, ended.ID, 1)
	assertAppErrorStatus(t, err, http.StatusBadRequest)
}

func TestDomainEvent_SingleAndMultiVoteRules(t *testing.T) {
	svc, _ := setupDomainEventTestEnv()
	now := time.Now()

	// 1. Single-vote event
	singleEv, err := svc.CreateEvent(10, false, 1, model.CreateDomainEventRequest{
		Title:          "Single Vote",
		AllowMultiVote: false,
		StartTime:      now.Add(-time.Hour),
		EndTime:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create single-vote event failed: %v", err)
	}

	// Member 12 adds folder 1001 -> auto-casts 1 vote
	detail, err := svc.AddCandidate(12, false, 1, singleEv.ID, model.AddDomainEventCandidateRequest{
		KBAssetID:   100,
		FolderDocID: 1001,
	})
	if err != nil {
		t.Fatalf("AddCandidate failed: %v", err)
	}
	if len(detail.Candidates) != 1 || detail.Candidates[0].VoteCount != 1 || !detail.Candidates[0].HasVoted {
		t.Fatalf("unexpected candidate state after add: %+v", detail.Candidates)
	}
	cand1ID := detail.Candidates[0].ID

	// Duplicate folder -> 409 Conflict
	_, err = svc.AddCandidate(13, false, 1, singleEv.ID, model.AddDomainEventCandidateRequest{
		KBAssetID:   100,
		FolderDocID: 1001,
	})
	assertAppErrorStatus(t, err, http.StatusConflict)

	// Member 12 cannot add another candidate in single-vote event (already voted) -> 409 Conflict
	_, err = svc.AddCandidate(12, false, 1, singleEv.ID, model.AddDomainEventCandidateRequest{
		KBAssetID:   100,
		FolderDocID: 1002,
	})
	assertAppErrorStatus(t, err, http.StatusConflict)

	// Member 13 adds folder 1002
	detail, err = svc.AddCandidate(13, false, 1, singleEv.ID, model.AddDomainEventCandidateRequest{
		KBAssetID:   100,
		FolderDocID: 1002,
	})
	if err != nil {
		t.Fatalf("member 13 AddCandidate failed: %v", err)
	}
	var cand2ID uint64
	for _, c := range detail.Candidates {
		if c.FolderDocID == 1002 {
			cand2ID = c.ID
		}
	}

	// Member 12 cannot vote for cand2 while still holding vote on cand1 -> 409 Conflict
	_, err = svc.VoteCandidate(12, false, 1, singleEv.ID, cand2ID)
	assertAppErrorStatus(t, err, http.StatusConflict)

	// Member 12 unvotes cand1, then votes for cand2
	detail, err = svc.UnvoteCandidate(12, false, 1, singleEv.ID, cand1ID)
	if err != nil {
		t.Fatalf("UnvoteCandidate failed: %v", err)
	}
	if len(detail.MyVotedCandidateIDs) != 0 {
		t.Fatalf("expected 0 voted candidates after unvote, got %v", detail.MyVotedCandidateIDs)
	}

	detail, err = svc.VoteCandidate(12, false, 1, singleEv.ID, cand2ID)
	if err != nil {
		t.Fatalf("VoteCandidate after unvote failed: %v", err)
	}
	if len(detail.MyVotedCandidateIDs) != 1 || detail.MyVotedCandidateIDs[0] != cand2ID {
		t.Fatalf("expected voted candidate %d, got %v", cand2ID, detail.MyVotedCandidateIDs)
	}

	// 2. Multi-vote event
	multiEv, err := svc.CreateEvent(10, false, 1, model.CreateDomainEventRequest{
		Title:          "Multi Vote",
		AllowMultiVote: true,
		StartTime:      now.Add(-time.Hour),
		EndTime:        now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create multi-vote event failed: %v", err)
	}

	detail, err = svc.AddCandidate(12, false, 1, multiEv.ID, model.AddDomainEventCandidateRequest{
		KBAssetID:   100,
		FolderDocID: 1001,
	})
	if err != nil {
		t.Fatalf("multi AddCandidate 1 failed: %v", err)
	}
	mCand1 := detail.Candidates[0].ID

	detail, err = svc.AddCandidate(12, false, 1, multiEv.ID, model.AddDomainEventCandidateRequest{
		KBAssetID:   100,
		FolderDocID: 1002,
	})
	if err != nil {
		t.Fatalf("multi AddCandidate 2 failed: %v", err)
	}
	if len(detail.MyVotedCandidateIDs) != 2 {
		t.Fatalf("expected 2 votes in multi-vote event, got %v", detail.MyVotedCandidateIDs)
	}

	// Duplicate vote on same candidate in multi-vote -> 409 Conflict
	_, err = svc.VoteCandidate(12, false, 1, multiEv.ID, mCand1)
	assertAppErrorStatus(t, err, http.StatusConflict)
}

func TestDomainEvent_CrossDomainKBAndNonFolderRejection(t *testing.T) {
	svc, _ := setupDomainEventTestEnv()
	now := time.Now()

	ev, err := svc.CreateEvent(10, false, 1, model.CreateDomainEventRequest{
		Title:     "Validation Vote",
		StartTime: now.Add(-time.Hour),
		EndTime:   now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}

	// Cross-domain KB -> 403 Forbidden
	_, err = svc.AddCandidate(12, false, 1, ev.ID, model.AddDomainEventCandidateRequest{
		KBAssetID:   200,
		FolderDocID: 2001,
	})
	assertAppErrorStatus(t, err, http.StatusForbidden)

	// Non-KB asset -> 400 BadRequest
	_, err = svc.AddCandidate(12, false, 1, ev.ID, model.AddDomainEventCandidateRequest{
		KBAssetID:   101,
		FolderDocID: 1001,
	})
	assertAppErrorStatus(t, err, http.StatusBadRequest)

	// Non-folder document -> 400 BadRequest
	_, err = svc.AddCandidate(12, false, 1, ev.ID, model.AddDomainEventCandidateRequest{
		KBAssetID:   100,
		FolderDocID: 1003,
	})
	assertAppErrorStatus(t, err, http.StatusBadRequest)
}

func TestDomainEvent_CloseAndDeletePermissions(t *testing.T) {
	svc, _ := setupDomainEventTestEnv()
	now := time.Now()

	// Created by member 12
	ev, err := svc.CreateEvent(12, false, 1, model.CreateDomainEventRequest{
		Title:     "Member Event",
		StartTime: now.Add(-time.Hour),
		EndTime:   now.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateEvent failed: %v", err)
	}

	// Non-creator member 13 cannot close or delete -> 403 Forbidden
	_, err = svc.CloseEvent(13, false, 1, ev.ID)
	assertAppErrorStatus(t, err, http.StatusForbidden)
	err = svc.DeleteEvent(13, false, 1, ev.ID)
	assertAppErrorStatus(t, err, http.StatusForbidden)

	// Domain admin 11 can close
	closed, err := svc.CloseEvent(11, false, 1, ev.ID)
	if err != nil {
		t.Fatalf("admin CloseEvent failed: %v", err)
	}
	if closed.Status != model.DomainEventStatusEnded || closed.ClosedAt == nil {
		t.Fatalf("expected ended status with ClosedAt, got %+v", closed)
	}

	// Creator 12 can delete
	if err := svc.DeleteEvent(12, false, 1, ev.ID); err != nil {
		t.Fatalf("creator DeleteEvent failed: %v", err)
	}
	_, err = svc.GetEventDetail(12, false, 1, ev.ID)
	assertAppErrorStatus(t, err, http.StatusNotFound)
}
