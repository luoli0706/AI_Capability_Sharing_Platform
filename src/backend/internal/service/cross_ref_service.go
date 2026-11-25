package service

import (
	"errors"
	"fmt"
	"strings"

	"ascp-backend/internal/model"
	"ascp-backend/internal/repository"

	"gorm.io/gorm"
)

type CrossRefService interface {
	CreateCrossRef(userID uint64, isSuperAdmin bool, targetDomainID uint64, req model.CreateCrossRefRequest) (*model.DomainCrossRef, error)
	ListCrossRefs(userID uint64, isSuperAdmin bool, targetDomainID uint64, includePending bool) ([]model.CrossRefView, error)
	DeleteCrossRef(userID uint64, isSuperAdmin bool, targetDomainID, refID uint64) error
	ListSourceDomains(userID uint64, isSuperAdmin bool, excludeDomainID uint64) ([]model.BizDomain, error)
	ListSourceItems(userID uint64, isSuperAdmin bool, sourceDomainID uint64) ([]model.CrossRefSourceAssetItem, []model.CrossRefSourceKBFileItem, error)
	SetAuditRepos(auditRepo repository.AuditLogRepository, userRepo repository.UserRepository)
}

type crossRefService struct {
	crossRefRepo repository.CrossRefRepository
	domainRepo   repository.DomainRepository
	orgRepo      repository.OrgRepository
	assetRepo    repository.AssetRepository
	kbRepo       repository.KBRepository
	govRepo      repository.GovernanceRepository
	auditRepo    repository.AuditLogRepository
	userRepo     repository.UserRepository
}

func NewCrossRefService(
	crossRefRepo repository.CrossRefRepository,
	domainRepo repository.DomainRepository,
	orgRepo repository.OrgRepository,
	assetRepo repository.AssetRepository,
	kbRepo repository.KBRepository,
	govRepo repository.GovernanceRepository,
) CrossRefService {
	return &crossRefService{
		crossRefRepo: crossRefRepo,
		domainRepo:   domainRepo,
		orgRepo:      orgRepo,
		assetRepo:    assetRepo,
		kbRepo:       kbRepo,
		govRepo:      govRepo,
	}
}

func (s *crossRefService) SetAuditRepos(auditRepo repository.AuditLogRepository, userRepo repository.UserRepository) {
	s.auditRepo = auditRepo
	s.userRepo = userRepo
}

func (s *crossRefService) CreateCrossRef(userID uint64, isSuperAdmin bool, targetDomainID uint64, req model.CreateCrossRefRequest) (*model.DomainCrossRef, error) {
	targetDomain, err := s.domainRepo.FindByID(targetDomainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("target domain not found")
		}
		return nil, err
	}

	// 校验 1：当前用户必须是目标域成员（或 super_admin）
	if !isSuperAdmin && targetDomain.OwnerID != userID {
		ok, err := hasDomainAccess(s.domainRepo, s.orgRepo, targetDomainID, userID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, NewForbidden("only target domain members can create cross-domain references")
		}
	}

	// 校验 2：source_domain_id != target_domain_id
	if req.SourceDomainID == targetDomainID {
		return nil, NewBadRequest("source_domain_id cannot be equal to target_domain_id")
	}

	// 校验 3：源域可见 & 被引用对象可见
	sourceDomain, err := s.domainRepo.FindByID(req.SourceDomainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("source domain not found")
		}
		return nil, err
	}
	hasSourceAccess, err := hasDomainAccess(s.domainRepo, s.orgRepo, req.SourceDomainID, userID)
	if err != nil {
		return nil, err
	}
	if !isSuperAdmin && sourceDomain.Visibility != model.DomainVisibilityPublic && !hasSourceAccess {
		return nil, NewForbidden("no permission to access source domain")
	}

	asset, err := s.assetRepo.FindByID(req.SourceAssetID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("source asset not found")
		}
		return nil, err
	}
	if asset.DomainID != req.SourceDomainID {
		return nil, NewBadRequest("source asset does not belong to source domain")
	}

	if !s.canViewSourceAsset(userID, isSuperAdmin, asset, hasSourceAccess) {
		return nil, NewForbidden("no permission to access source asset")
	}

	var sourceDocID uint64
	title := asset.Name
	if req.RefType == model.CrossRefTypeKBFile {
		if asset.AssetType != model.AssetTypeKB {
			return nil, NewBadRequest("source asset must be a knowledge base when ref_type is kb_file")
		}
		if req.SourceDocID == 0 {
			return nil, NewBadRequest("source_doc_id is required when ref_type is kb_file")
		}
		doc, err := s.kbRepo.FindDocumentByID(asset.ID, req.SourceDocID)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, NewNotFound("source kb document not found")
			}
			return nil, err
		}
		if doc.IsFolder {
			return nil, NewBadRequest("cannot reference a folder, only files are allowed")
		}
		sourceDocID = doc.ID
		title = doc.FileName
	}

	// 校验 4：防重复创建同一目标域对同一对象的 pending/active 引用
	exists, err := s.crossRefRepo.ExistsPendingOrActive(targetDomainID, req.RefType, req.SourceAssetID, sourceDocID)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, NewConflict("该对象已在当前域存在有效或审批中的跨域引用")
	}

	// 权限分支：目标域 owner/admin 或 super_admin 直接生效，否则走审批
	isTargetAdmin := isSuperAdmin || targetDomain.OwnerID == userID
	if !isTargetAdmin {
		role, err := s.domainRepo.GetMemberRole(targetDomainID, userID)
		if err != nil {
			return nil, err
		}
		isTargetAdmin = role == model.DomainRoleOwner || role == model.DomainRoleAdmin
	}

	status := model.CrossRefStatusPending
	if isTargetAdmin {
		status = model.CrossRefStatusActive
	}

	ref := &model.DomainCrossRef{
		TargetDomainID: targetDomainID,
		SourceDomainID: req.SourceDomainID,
		RefType:        req.RefType,
		SourceAssetID:  req.SourceAssetID,
		SourceDocID:    sourceDocID,
		Title:          truncateStr(title, 200),
		Description:    strings.TrimSpace(req.Description),
		CreatedBy:      userID,
		Status:         status,
	}
	if err := s.crossRefRepo.Create(ref); err != nil {
		return nil, err
	}

	if !isTargetAdmin {
		reason := ref.Description
		if reason == "" {
			reason = fmt.Sprintf("申请跨域引用 %s", ref.Title)
		}
		requestNo := generateRequestNo()
		approvalReq := &model.ApprovalRequest{
			RequestNo:         requestNo,
			ApplicantID:       userID,
			ApplicantDomainID: targetDomainID,
			TargetType:        model.ApprovalTargetCrossRef,
			AssetID:           ref.ID,
			RequestedPerm:     model.PermLevelInvoke,
			DurationDays:      0,
			Reason:            truncateStr(reason, 512),
			CurrentStep:       1,
			TotalSteps:        1,
			Status:            model.ApprovalStatusPending,
		}
		records := []model.ApprovalRecord{
			{
				StepNo:     1,
				StepName:   "目标域管理员审批",
				ApproverID: targetDomain.OwnerID,
				Action:     model.ApprovalActionPending,
			},
		}
		if err := s.govRepo.CreateApprovalWithRecords(approvalReq, records); err != nil {
			_ = s.crossRefRepo.DeleteByID(ref.ID)
			return nil, err
		}
		ref.ApprovalRequestID = approvalReq.ID
		if err := s.crossRefRepo.UpdateApprovalRequestID(ref.ID, approvalReq.ID); err != nil {
			return nil, err
		}
	}

	recordAudit(s.auditRepo, s.userRepo, userID, targetDomain.ID, targetDomain.DomainName, ref.SourceAssetID, asset.AssetCode, ref.Title, ref.RefType, model.AuditActionCrossRefCreate, ref.Title, fmt.Sprintf("创建跨域引用 (%s)", ref.Status))
	return ref, nil
}

func (s *crossRefService) ListCrossRefs(userID uint64, isSuperAdmin bool, targetDomainID uint64, includePending bool) ([]model.CrossRefView, error) {
	targetDomain, err := s.domainRepo.FindByID(targetDomainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, NewNotFound("domain not found")
		}
		return nil, err
	}

	hasAccess := isSuperAdmin || targetDomain.OwnerID == userID
	if !hasAccess {
		hasAccess, err = hasDomainAccess(s.domainRepo, s.orgRepo, targetDomainID, userID)
		if err != nil {
			return nil, err
		}
	}
	if !hasAccess && targetDomain.Visibility != model.DomainVisibilityPublic {
		return nil, NewForbidden("no permission to access this domain")
	}

	allPending := false
	effectiveIncludePending := false
	if includePending && hasAccess {
		effectiveIncludePending = true
		if isSuperAdmin || targetDomain.OwnerID == userID {
			allPending = true
		} else {
			role, err := s.domainRepo.GetMemberRole(targetDomainID, userID)
			if err != nil {
				return nil, err
			}
			allPending = role == model.DomainRoleOwner || role == model.DomainRoleAdmin
		}
	}

	return s.crossRefRepo.ListByTargetDomain(targetDomainID, effectiveIncludePending, userID, allPending)
}

func (s *crossRefService) DeleteCrossRef(userID uint64, isSuperAdmin bool, targetDomainID, refID uint64) error {
	targetDomain, err := s.domainRepo.FindByID(targetDomainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewNotFound("domain not found")
		}
		return err
	}
	ref, err := s.crossRefRepo.FindByID(refID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return NewNotFound("cross-domain reference not found")
		}
		return err
	}
	if ref.TargetDomainID != targetDomainID {
		return NewNotFound("cross-domain reference not found in this domain")
	}

	allowed := isSuperAdmin || ref.CreatedBy == userID || targetDomain.OwnerID == userID
	if !allowed {
		role, err := s.domainRepo.GetMemberRole(targetDomainID, userID)
		if err != nil {
			return err
		}
		allowed = role == model.DomainRoleOwner || role == model.DomainRoleAdmin
	}
	if !allowed {
		return NewForbidden("only target domain admin, super_admin, or creator can delete cross-domain reference")
	}

	if err := s.crossRefRepo.DeleteByID(refID); err != nil {
		return err
	}
	recordAudit(s.auditRepo, s.userRepo, userID, targetDomain.ID, targetDomain.DomainName, ref.SourceAssetID, "", ref.Title, ref.RefType, model.AuditActionCrossRefDelete, ref.Title, "删除跨域引用 "+ref.Title)
	return nil
}

func (s *crossRefService) ListSourceDomains(userID uint64, isSuperAdmin bool, excludeDomainID uint64) ([]model.BizDomain, error) {
	var accessibleIDs []uint64
	if !isSuperAdmin {
		var err error
		accessibleIDs, err = getAccessibleDomainIDs(s.domainRepo, s.orgRepo, userID)
		if err != nil {
			return nil, err
		}
	}
	return s.crossRefRepo.ListSourceDomains(isSuperAdmin, accessibleIDs, excludeDomainID)
}

func (s *crossRefService) ListSourceItems(userID uint64, isSuperAdmin bool, sourceDomainID uint64) ([]model.CrossRefSourceAssetItem, []model.CrossRefSourceKBFileItem, error) {
	sourceDomain, err := s.domainRepo.FindByID(sourceDomainID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, NewNotFound("source domain not found")
		}
		return nil, nil, err
	}

	hasSourceAccess, err := hasDomainAccess(s.domainRepo, s.orgRepo, sourceDomainID, userID)
	if err != nil {
		return nil, nil, err
	}
	if !isSuperAdmin && sourceDomain.Visibility != model.DomainVisibilityPublic && !hasSourceAccess {
		return nil, nil, NewForbidden("no permission to access source domain")
	}

	allAssets, err := s.crossRefRepo.ListPublishedAssetsByDomain(sourceDomainID)
	if err != nil {
		return nil, nil, err
	}

	var permittedSet map[uint64]bool
	if !isSuperAdmin && s.govRepo != nil {
		ids, _ := s.govRepo.GetUserPermittedAssetIDs(userID)
		permittedSet = toUint64Set(ids)
	}

	assetsOut := make([]model.CrossRefSourceAssetItem, 0, len(allAssets))
	kbAssetNames := make(map[uint64]string)
	var kbAssetIDs []uint64
	for i := range allAssets {
		a := &allAssets[i]
		visible := isSuperAdmin ||
			a.OwnerID == userID ||
			a.Visibility == model.VisibilityPublic ||
			(a.Visibility == model.VisibilityDomain && hasSourceAccess) ||
			permittedSet[a.ID]
		if !visible {
			continue
		}
		assetsOut = append(assetsOut, model.CrossRefSourceAssetItem{
			ID:        a.ID,
			AssetCode: a.AssetCode,
			Name:      a.Name,
			AssetType: a.AssetType,
			Summary:   a.Summary,
		})
		if a.AssetType == model.AssetTypeKB {
			kbAssetIDs = append(kbAssetIDs, a.ID)
			kbAssetNames[a.ID] = a.Name
		}
	}

	kbFilesOut := make([]model.CrossRefSourceKBFileItem, 0)
	if len(kbAssetIDs) > 0 {
		docs, err := s.crossRefRepo.ListKBFilesByAssetIDs(kbAssetIDs)
		if err != nil {
			return nil, nil, err
		}
		for _, doc := range docs {
			if doc.IsFolder {
				continue
			}
			kbFilesOut = append(kbFilesOut, model.CrossRefSourceKBFileItem{
				ID:         doc.ID,
				KBAssetID:  doc.KBAssetID,
				KBName:     kbAssetNames[doc.KBAssetID],
				FolderPath: doc.FolderPath,
				FileName:   doc.FileName,
				FileExt:    doc.FileExt,
				FileSize:   doc.FileSize,
			})
		}
	}

	return assetsOut, kbFilesOut, nil
}

func (s *crossRefService) canViewSourceAsset(userID uint64, isSuperAdmin bool, asset *model.AIAsset, hasSourceDomainAccess bool) bool {
	if isSuperAdmin || asset.OwnerID == userID {
		return true
	}
	if asset.Status != model.AssetStatusPublished {
		return false
	}
	if asset.Visibility == model.VisibilityPublic {
		return true
	}
	if asset.Visibility == model.VisibilityDomain && hasSourceDomainAccess {
		return true
	}
	if s.govRepo != nil {
		if _, err := s.govRepo.GetActiveUserPermission(asset.ID, userID); err == nil {
			return true
		}
	}
	return false
}

func truncateStr(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	return string(r[:maxLen])
}
