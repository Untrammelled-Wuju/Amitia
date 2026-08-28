package editing

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/log"
)

const (
	regenLeaseDuration     = 5 * time.Minute
	regenPollInterval      = 5 * time.Second
	regenHeartbeatInterval = 30 * time.Second
	regenWorkerID          = "regen-worker-1"
	regenMaxConcurrency    = 1
	regenMaxAttempts       = 3
)

func stableRegenerationAttemptID(jobID, scope string) string {
	return "attempt-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(jobID+"|"+scope+"|provider-attempt")).String()
}

func stableFullActionRevisionID(jobID string) string {
	return "rev-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(jobID+"|full-action-revision")).String()
}

func stableFullActionCandidateID(jobID string) string {
	return "cand-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(jobID+"|full-action-candidate")).String()
}

func stableFullActionFrameID(jobID string, index int) string {
	return "frame-regen-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s|full-action-frame|%d", jobID, index))).String()
}

func stableFullActionRevisionFrameID(jobID string, index int) string {
	return "rf-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s|full-action-revision-frame|%d", jobID, index))).String()
}

func (w *RegenerationWorker) updateClaimedJob(job *RegenerationJob, fields map[string]any) error {
	if job == nil || job.ID == "" || job.ExecutionID == "" {
		return fmt.Errorf("regeneration job lease identity is missing")
	}
	ok, err := w.repo.UpdateClaimedJobFields(job.ID, w.workerID, job.ExecutionID, fields)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("regeneration job lease lost: %s", job.ID)
	}
	return nil
}

type RegenerationWorker struct {
	repo              Repository
	genAdapter        GenerationAdapter
	assetStore        RevisionAssetStore
	qualAdapter       QualityAdapter
	procAdapter       ProcessingAdapter
	leaseDuration     time.Duration
	heartbeatInterval time.Duration
	pollInterval      time.Duration
	maxConcurrency    int
	workerID          string
	stopCh            chan struct{}
	wg                sync.WaitGroup
	sem               chan struct{}
}

func NewRegenerationWorker(repo Repository, genAdapter GenerationAdapter, assetStore RevisionAssetStore, qualAdapter QualityAdapter, procAdapter ProcessingAdapter) *RegenerationWorker {
	return &RegenerationWorker{
		repo:              repo,
		genAdapter:        genAdapter,
		assetStore:        assetStore,
		qualAdapter:       qualAdapter,
		procAdapter:       procAdapter,
		leaseDuration:     regenLeaseDuration,
		heartbeatInterval: regenHeartbeatInterval,
		pollInterval:      regenPollInterval,
		maxConcurrency:    regenMaxConcurrency,
		workerID:          regenWorkerID,
		stopCh:            make(chan struct{}),
		sem:               make(chan struct{}, regenMaxConcurrency),
	}
}

func (w *RegenerationWorker) Start(ctx context.Context) {
	w.recoverStaleJobs(ctx)
	w.wg.Add(1)
	go w.run(ctx)
}

func (w *RegenerationWorker) Stop() {
	close(w.stopCh)
	w.wg.Wait()
}

func (w *RegenerationWorker) run(ctx context.Context) {
	defer w.wg.Done()
	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()
	w.pollAndProcess(ctx)
	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.pollAndProcess(ctx)
		}
	}
}

// pollAndProcess uses the repository's atomic claim operation. The previous
// ListPendingJobs implementation could return jobs that were already running,
// allowing a later poll to submit the same provider request again.
func (w *RegenerationWorker) pollAndProcess(ctx context.Context) {
	for slot := 0; slot < w.maxConcurrency; slot++ {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case w.sem <- struct{}{}:
		}

		executionID := "regen-exec-" + uuid.NewString()
		job, err := w.repo.ClaimNextJob(w.workerID, executionID, w.leaseDuration)
		if err != nil {
			<-w.sem
			log.Logger.Errorf("regeneration worker claim job failed: %v", err)
			return
		}
		if job == nil {
			<-w.sem
			return
		}

		w.wg.Add(1)
		go func(claimed *RegenerationJob) {
			defer w.wg.Done()
			defer func() { <-w.sem }()
			w.processJob(ctx, claimed)
		}(job)
	}
}

func (w *RegenerationWorker) processJob(ctx context.Context, job *RegenerationJob) {
	if job == nil || IsTerminalJobStatus(job.Status) {
		return
	}

	jobCtx, jobCancel := context.WithCancel(ctx)
	defer jobCancel()
	heartbeatCtx, heartbeatCancel := context.WithCancel(jobCtx)
	w.startHeartbeat(heartbeatCtx, job, jobCancel)
	defer heartbeatCancel()

	// A crashed worker can be retried after it already materialized a candidate.
	// Reconcile that durable result before making another provider call.
	if done, err := w.reconcileExistingCandidate(jobCtx, job); err != nil {
		w.failJob(job, "CANDIDATE_RECOVERY_FAILED", err.Error())
		return
	} else if done {
		return
	}

	session, err := w.repo.GetEditSession(job.SessionID)
	if err != nil {
		w.failJob(job, "SESSION_FETCH_FAILED", fmt.Sprintf("获取会话失败: %v", err))
		return
	}
	if session == nil || session.UserID != job.UserID {
		w.failJob(job, "SESSION_NOT_FOUND", "会话不存在或所有权不匹配")
		return
	}

	baseRevisionID := job.BaseActionRevisionID
	if baseRevisionID == "" {
		baseRevisionID = session.BaseRevisionID
	}
	baseRev, err := w.repo.GetActionRevision(baseRevisionID)
	if err != nil {
		w.failJob(job, "REVISION_FETCH_FAILED", fmt.Sprintf("获取基线Revision失败: %v", err))
		return
	}
	if job.BaseContentHash != "" && baseRev.ContentHash != "" && job.BaseContentHash != baseRev.ContentHash {
		w.failJob(job, "BASE_CONTENT_DRIFTED", "基线Revision内容哈希已漂移")
		return
	}
	if job.DraftSnapshotID == "" {
		w.failJob(job, "DRAFT_SNAPSHOT_MISSING", "重生成任务缺少不可变Draft Snapshot")
		return
	}
	snapshot, err := w.repo.GetDraftSnapshot(job.DraftSnapshotID)
	if err != nil || snapshot == nil {
		w.failJob(job, "DRAFT_SNAPSHOT_FETCH_FAILED", fmt.Sprintf("读取Draft Snapshot失败: %v", err))
		return
	}
	if job.DraftSnapshotHash != "" && snapshot.SnapshotHash != job.DraftSnapshotHash {
		w.failJob(job, "DRAFT_SNAPSHOT_HASH_MISMATCH", "Draft Snapshot哈希不匹配")
		return
	}

	var processErr error
	switch job.JobType {
	case JobTypeSingleFrame, JobTypeBackgroundReprocess, JobTypeNormalizeUpload:
		processErr = w.processSingleFrameJob(jobCtx, job, session, baseRev, snapshot)
	case JobTypeFullAction:
		processErr = w.processFullActionJob(jobCtx, job, session, baseRev, snapshot)
	default:
		processErr = fmt.Errorf("未知 job 类型: %s", job.JobType)
	}
	if processErr != nil {
		w.failJob(job, "PROCESS_FAILED", processErr.Error())
		return
	}
	log.Logger.Infof("regeneration worker materialized candidate for job: %s", job.ID)
}

func (w *RegenerationWorker) ensureRevisionQualityEvaluation(ctx context.Context, rev *ActionRevision) (string, error) {
	if rev == nil || rev.ID == "" {
		return "", fmt.Errorf("candidate revision is required for quality evaluation")
	}
	if rev.QualityEvaluationID != "" {
		return rev.QualityEvaluationID, nil
	}

	// EvaluateRevision persists the evaluation row before CAS-updating the
	// revision. If the process dies between those two writes, recover that
	// pending evaluation instead of creating another one.
	var pending struct {
		ID string `gorm:"column:id"`
	}
	if err := w.repo.DB().Table("desktop_pet_quality_evaluations").
		Where("action_revision_id = ? AND is_active = 1 AND execution_status IN ?", rev.ID, []string{"pending", "running"}).
		Order("created_at DESC").Limit(1).Find(&pending).Error; err != nil {
		return "", err
	}
	if pending.ID != "" {
		now := nowUTC()
		if err := w.repo.DB().Model(&ActionRevision{}).Where("id = ? AND quality_evaluation_id = ''", rev.ID).Updates(map[string]any{
			"quality_evaluation_id": pending.ID,
			"status":                RevisionStatusQualityPending,
			"updated_at":            now,
		}).Error; err != nil {
			return "", err
		}
		return pending.ID, nil
	}
	if rev.Status != RevisionStatusReady {
		return "", fmt.Errorf("candidate revision %s cannot start quality evaluation from status %s", rev.ID, rev.Status)
	}
	return w.qualAdapter.EvaluateRevision(ctx, rev.ID)
}

func (w *RegenerationWorker) reconcileExistingCandidate(ctx context.Context, job *RegenerationJob) (bool, error) {
	var candidate EditCandidate
	result := w.repo.DB().Where("job_id = ?", job.ID).Order("created_at DESC").Limit(1).Find(&candidate)
	if result.Error != nil {
		return false, result.Error
	}
	if candidate.ID == "" {
		return false, nil
	}
	switch candidate.Status {
	case CandidateStatusAccepted:
		return true, w.updateClaimedJob(job, map[string]any{
			"status": JobStatusAccepted, "completed_at": nowUTC(),
			"lease_owner": "", "lease_expires_at": "", "heartbeat_at": "", "execution_id": "",
		})
	case CandidateStatusRejected, CandidateStatusStaleCandidate:
		return true, w.updateClaimedJob(job, map[string]any{
			"status": JobStatusRejected, "completed_at": nowUTC(),
			"lease_owner": "", "lease_expires_at": "", "heartbeat_at": "", "execution_id": "",
		})
	case CandidateStatusReadyForReview, CandidateStatusPending:
		return true, w.updateClaimedJob(job, map[string]any{
			"status": JobStatusReadyForReview, "stage": JobStatusReadyForReview,
			"lease_owner": "", "lease_expires_at": "", "heartbeat_at": "", "execution_id": "",
		})
	case CandidateStatusQualityPending, CandidateStatusQualityRunning:
		evalID := candidate.QualityEvaluationID
		if candidate.CandidateRevisionID == "" {
			return false, fmt.Errorf("quality candidate %s has no candidate revision", candidate.ID)
		}
		rev, err := w.repo.GetActionRevision(candidate.CandidateRevisionID)
		if err != nil {
			return false, err
		}
		if evalID == "" {
			evalID, err = w.ensureRevisionQualityEvaluation(ctx, rev)
			if err != nil {
				return false, err
			}
		}
		if err := w.repo.UpdateCandidateFields(candidate.ID, map[string]any{
			"quality_evaluation_id": evalID,
			"quality_status":        CandidateStatusQualityPending,
		}); err != nil {
			return false, err
		}
		return true, w.updateClaimedJob(job, map[string]any{
			"status": JobStatusQualityPending, "stage": JobStatusQualityPending,
			"candidate_revision_id": candidate.CandidateRevisionID,
			"quality_evaluation_id": evalID,
			"lease_owner":           "", "lease_expires_at": "", "heartbeat_at": "", "execution_id": "",
		})
	}
	return false, nil
}

func (w *RegenerationWorker) loadDraftFrames(snapshot *EditDraftSnapshot) ([]draftFrame, error) {
	var frames []draftFrame
	if err := json.Unmarshal([]byte(snapshot.FramesJSON), &frames); err != nil {
		return nil, fmt.Errorf("解析Draft Snapshot帧失败: %w", err)
	}
	if len(frames) == 0 {
		return nil, ErrFrameCountInvalid
	}
	return frames, nil
}

func (w *RegenerationWorker) processSingleFrameJob(ctx context.Context, job *RegenerationJob, session *EditSession, baseRev *ActionRevision, snapshot *EditDraftSnapshot) error {
	frames, err := w.loadDraftFrames(snapshot)
	if err != nil {
		return err
	}
	targetIndex := -1
	for i := range frames {
		if frames[i].FrameID == job.TargetFrameID {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return fmt.Errorf("目标帧 %s 不存在于捕获的Draft Snapshot", job.TargetFrameID)
	}
	if err := w.updateClaimedJob(job, map[string]any{"status": JobStatusSubmitting, "stage": JobStatusSubmitting}); err != nil {
		return err
	}

	adjacentFrames := w.buildAdjacentDraftFrames(frames, targetIndex)
	result, err := w.genAdapter.GenerateSingleFrame(ctx, SingleFrameGenerationRequest{
		JobID:            job.ID,
		GenerationTaskID: baseRev.GenerationTaskID,
		ActionKey:        job.ActionKey,
		TargetFrameID:    job.TargetFrameID,
		FrameIndex:       targetIndex,
		TotalFrames:      len(frames),
		AdjacentFrames:   adjacentFrames,
		UserID:           session.UserID,
		AttemptID:        stableRegenerationAttemptID(job.ID, "single_frame"),
	})
	if err != nil {
		return fmt.Errorf("提交单帧生成请求失败: %w", err)
	}
	if err := w.updateClaimedJob(job, map[string]any{
		"status":                JobStatusArtifactReady,
		"stage":                 JobStatusArtifactReady,
		"provider_attempt_id":   result.ProviderAttemptID,
		"generation_attempt_id": result.ProviderAttemptID,
	}); err != nil {
		return err
	}

	imagePath := result.ImagePath
	if imagePath == "" {
		artifacts, artErr := w.genAdapter.GetGenerationArtifacts(ctx, baseRev.GenerationTaskID, job.ActionKey, 0)
		if artErr != nil {
			return fmt.Errorf("获取生成产物失败: %w", artErr)
		}
		for _, art := range artifacts {
			if art.AttemptID == result.ProviderAttemptID && art.FrameIndex == targetIndex {
				imagePath = art.ImagePath
				break
			}
		}
	}
	if imagePath == "" {
		return fmt.Errorf("生成产物图片路径为空")
	}
	if err := w.updateClaimedJob(job, map[string]any{"status": JobStatusProcessing, "stage": JobStatusProcessing}); err != nil {
		return err
	}
	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("读取生成图片失败: %w", err)
	}
	asset, err := w.assetStore.WriteAsset(ctx, imgData, "image/png", AssetSourceRegenerated, result.ProviderAttemptID)
	if err != nil {
		return fmt.Errorf("持久化生成资产失败: %w", err)
	}

	candidateID := "cand-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(job.ID+"|single-frame-candidate")).String()
	candidate := &EditCandidate{
		ID:                  candidateID,
		SessionID:           job.SessionID,
		JobID:               job.ID,
		UserID:              job.UserID,
		CharacterID:         job.CharacterID,
		ActionStreamID:      job.ActionStreamID,
		CandidateVersion:    snapshot.SessionVersion,
		DraftSnapshotID:     snapshot.ID,
		DraftSnapshotHash:   snapshot.SnapshotHash,
		TargetFrameID:       job.TargetFrameID,
		CandidateType:       AssetSourceRegenerated,
		AssetID:             asset.ID,
		Status:              CandidateStatusReadyForReview,
		SourceType:          AssetSourceRegenerated,
		ParentRevisionID:    job.BaseActionRevisionID,
		ParentContentHash:   job.BaseContentHash,
		BaseBindingRevision: job.BaseBindingRevision,
		ActivationPolicy:    ActivationPolicyManual,
		CreatedAt:           nowUTC(),
	}
	if err := w.repo.CreateCandidate(candidate); err != nil {
		var existing EditCandidate
		if readErr := w.repo.DB().Where("id = ?", candidateID).First(&existing).Error; readErr != nil {
			return fmt.Errorf("创建候选失败: %w", err)
		}
	}
	if err := w.repo.UpdateJobResult(job.ID, result.ProviderAttemptID, nil); err != nil {
		return err
	}
	return w.updateClaimedJob(job, map[string]any{
		"status":           JobStatusReadyForReview,
		"stage":            JobStatusReadyForReview,
		"lease_owner":      "",
		"lease_expires_at": "",
		"heartbeat_at":     "",
		"execution_id":     "",
	})
}

func (w *RegenerationWorker) processFullActionJob(ctx context.Context, job *RegenerationJob, session *EditSession, baseRev *ActionRevision, snapshot *EditDraftSnapshot) error {
	if err := w.updateClaimedJob(job, map[string]any{"status": JobStatusSubmitting, "stage": JobStatusSubmitting}); err != nil {
		return err
	}

	attemptID := stableRegenerationAttemptID(job.ID, "full_action")
	result, err := w.genAdapter.GenerateFullAction(ctx, FullActionGenerationRequest{
		JobID:            job.ID,
		GenerationTaskID: baseRev.GenerationTaskID,
		ActionKey:        job.ActionKey,
		UserID:           session.UserID,
		AttemptID:        attemptID,
	})
	if err != nil {
		return fmt.Errorf("提交整动作生成请求失败: %w", err)
	}
	if result.ProviderAttemptID == "" {
		result.ProviderAttemptID = attemptID
	}
	if err := w.updateClaimedJob(job, map[string]any{
		"status":                JobStatusArtifactReady,
		"stage":                 JobStatusArtifactReady,
		"provider_attempt_id":   result.ProviderAttemptID,
		"generation_attempt_id": result.ProviderAttemptID,
	}); err != nil {
		return err
	}

	// If a prior worker completed the durable revision but crashed before the
	// candidate/job bookkeeping, finish from that revision without re-materializing.
	existingRev, err := w.getFullActionRevision(job)
	if err != nil {
		return err
	}
	if existingRev != nil {
		if existingRev.CreatedFromSessionID != job.SessionID || existingRev.ParentActionRevisionID != job.BaseActionRevisionID || existingRev.ActionKey != job.ActionKey {
			return fmt.Errorf("stable candidate revision %s is bound to a different regeneration job", existingRev.ID)
		}
		if fullActionRevisionMaterialized(existingRev) {
			return w.ensureFullActionCandidate(ctx, job, snapshot, existingRev, result.ProviderAttemptID)
		}
	}

	framePaths := result.FramePaths
	if len(framePaths) == 0 {
		artifacts, artErr := w.genAdapter.GetGenerationArtifacts(ctx, baseRev.GenerationTaskID, job.ActionKey, 0)
		if artErr != nil {
			return fmt.Errorf("获取生成产物失败: %w", artErr)
		}
		for _, art := range artifacts {
			if art.AttemptID == result.ProviderAttemptID && art.ImagePath != "" {
				framePaths = append(framePaths, art.ImagePath)
			}
		}
	}
	if len(framePaths) == 0 {
		return fmt.Errorf("未找到生成产物")
	}
	if err := w.updateClaimedJob(job, map[string]any{"status": JobStatusCandidateCommitting, "stage": JobStatusCandidateCommitting}); err != nil {
		return err
	}

	candidateRevID := stableFullActionRevisionID(job.ID)
	var revNum int
	if existingRev != nil {
		// Reuse the already allocated revision number and deterministic revision
		// identity after a partial crash. The materialization below is rebuilt
		// from content-addressed assets.
		revNum = existingRev.RevisionNumber
		if err := w.repo.DeleteRevisionFrames(candidateRevID); err != nil {
			return err
		}
		if err := w.repo.DB().Delete(&ActionRevision{}, "id = ?", candidateRevID).Error; err != nil {
			return err
		}
	} else if job.ActionStreamID != "" {
		revNum, err = allocateActionStreamRevisionNumber(w.repo, job.ActionStreamID)
	} else {
		revNum, err = w.repo.GetNextRevisionNumber(job.ProcessingTaskID, job.ActionKey)
	}
	if err != nil {
		return fmt.Errorf("获取下一个Revision号失败: %w", err)
	}

	now := nowUTC()
	rootRevID := baseRev.RootRevisionID
	if rootRevID == "" {
		rootRevID = baseRev.ID
	}
	rootActionRevID := baseRev.RootActionRevisionID
	if rootActionRevID == "" {
		rootActionRevID = baseRev.ID
	}
	candidateRev := &ActionRevision{
		ID:                         candidateRevID,
		UserID:                     job.UserID,
		CharacterID:                job.CharacterID,
		ProcessingTaskID:           job.ProcessingTaskID,
		ProcessingActionID:         baseRev.ProcessingActionID,
		GenerationTaskID:           baseRev.GenerationTaskID,
		ActionKey:                  job.ActionKey,
		ParentRevisionID:           job.BaseActionRevisionID,
		RootRevisionID:             rootRevID,
		RevisionNumber:             revNum,
		RevisionType:               RevisionTypeRegenerated,
		Status:                     RevisionStatusBuilding,
		FrameCount:                 len(framePaths),
		DefaultFPS:                 baseRev.DefaultFPS,
		LoopType:                   baseRev.LoopType,
		ReturnAction:               baseRev.ReturnAction,
		Interruptible:              baseRev.Interruptible,
		PriorityOverride:           baseRev.PriorityOverride,
		CooldownMSOverride:         baseRev.CooldownMSOverride,
		CreatedByUserID:            session.UserID,
		CreatedFromSessionID:       session.ID,
		ChangeSummary:              "整动作重生成候选",
		CreatedAt:                  now,
		UpdatedAt:                  now,
		SourceType:                 canonicalSourceFullActionRegeneration,
		ContentHashVersion:         canonicalContentHashVersionManifestV1,
		Origin:                     canonicalOriginUser,
		PlaybackMode:               baseRev.PlaybackMode,
		ActionStreamID:             job.ActionStreamID,
		SourceProcessingRevisionID: baseRev.SourceProcessingRevisionID,
		SourceProcessingTaskID:     baseRev.SourceProcessingTaskID,
		SourceProcessingActionID:   baseRev.SourceProcessingActionID,
		SourceProcessingAttemptID:  result.ProviderAttemptID,
		ParentActionRevisionID:     job.BaseActionRevisionID,
		RootActionRevisionID:       rootActionRevID,
		ActionSpecHash:             baseRev.ActionSpecHash,
	}
	if candidateRev.SourceProcessingTaskID == "" {
		candidateRev.SourceProcessingTaskID = job.ProcessingTaskID
	}
	if candidateRev.SourceProcessingActionID == "" {
		candidateRev.SourceProcessingActionID = baseRev.ProcessingActionID
	}
	if err := w.repo.CreateActionRevision(candidateRev); err != nil {
		return fmt.Errorf("创建候选Revision失败: %w", err)
	}

	revFrames := make([]ActionRevisionFrame, 0, len(framePaths))
	manifestFrames := make([]ManifestFrame, 0, len(framePaths))
	for i, framePath := range framePaths {
		imgData, readErr := os.ReadFile(framePath)
		if readErr != nil {
			return fmt.Errorf("读取生成帧 %d 失败: %w", i, readErr)
		}
		asset, assetErr := w.assetStore.WriteAsset(ctx, imgData, "image/png", AssetSourceRegenerated, result.ProviderAttemptID)
		if assetErr != nil {
			return fmt.Errorf("持久化生成帧 %d 失败: %w", i, assetErr)
		}
		frameID := stableFullActionFrameID(job.ID, i)
		revFrames = append(revFrames, ActionRevisionFrame{
			ID:               stableFullActionRevisionFrameID(job.ID, i),
			RevisionID:       candidateRevID,
			FrameID:          frameID,
			AssetID:          asset.ID,
			LogicalIndex:     i,
			DurationMS:       DefaultFrameDurationMS,
			SourceRevisionID: candidateRevID,
			SourceAttemptID:  result.ProviderAttemptID,
			AnchorX:          DefaultAnchorX,
			AnchorY:          DefaultAnchorY,
			AnchorSpace:      AnchorSpaceNormalizedCanvas,
			CreatedAt:        now,
		})
		manifestFrames = append(manifestFrames, ManifestFrame{
			FrameID:      frameID,
			LogicalIndex: i,
			AssetID:      asset.ID,
			ContentHash:  asset.ContentHash,
			DurationMS:   DefaultFrameDurationMS,
			Anchor:       ManifestAnchor{X: DefaultAnchorX, Y: DefaultAnchorY, Space: AnchorSpaceNormalizedCanvas},
			Lineage:      ManifestLineage{Type: LineageRegenerated, SourceRevisionID: candidateRevID, SourceAttemptID: result.ProviderAttemptID},
		})
	}
	if err := w.repo.CreateRevisionFrames(revFrames); err != nil {
		return fmt.Errorf("创建Revision帧失败: %w", err)
	}
	manifest := &RevisionManifest{
		SchemaVersion:    ManifestSchemaVersion,
		RevisionID:       candidateRevID,
		ParentRevisionID: job.BaseActionRevisionID,
		ProcessingTaskID: job.ProcessingTaskID,
		ActionKey:        job.ActionKey,
		Playback: ManifestPlayback{
			LoopType:      baseRev.LoopType,
			DefaultFPS:    baseRev.DefaultFPS,
			ReturnAction:  baseRev.ReturnAction,
			Interruptible: baseRev.Interruptible != 0,
		},
		Frames:    manifestFrames,
		Quality:   ManifestQuality{},
		CreatedAt: now,
	}
	manifestPath, manifestHash, err := w.assetStore.WriteManifest(candidateRevID, manifest)
	if err != nil {
		return fmt.Errorf("写入候选Revision manifest失败: %w", err)
	}
	durationMS := len(revFrames) * DefaultFrameDurationMS
	if err := w.repo.UpdateActionRevisionManifest(candidateRevID, manifestPath, manifestHash, len(revFrames), durationMS); err != nil {
		return err
	}
	actionConfigJSON, err := marshalCanonicalJSON(draftActionConfigSnapshot{
		DefaultFPS:         baseRev.DefaultFPS,
		LoopType:           baseRev.LoopType,
		ReturnAction:       baseRev.ReturnAction,
		Interruptible:      baseRev.Interruptible != 0,
		PriorityOverride:   baseRev.PriorityOverride,
		CooldownMSOverride: baseRev.CooldownMSOverride,
	})
	if err != nil {
		return err
	}
	frameSetJSON, err := marshalCanonicalJSON(manifestFrames)
	if err != nil {
		return err
	}
	actionConfigHash := w.assetStore.ComputeHash([]byte(actionConfigJSON))
	frameSetHash := w.assetStore.ComputeHash([]byte(frameSetJSON))
	revisionSnapshotJSON, err := marshalCanonicalJSON(map[string]any{"manifestHash": manifestHash, "actionConfigHash": actionConfigHash, "frameSetHash": frameSetHash})
	if err != nil {
		return err
	}
	revisionSnapshotHash := w.assetStore.ComputeHash([]byte(revisionSnapshotJSON))
	if err := w.repo.DB().Model(&ActionRevision{}).Where("id = ?", candidateRevID).Updates(map[string]any{
		"content_hash":                manifestHash,
		"content_hash_version":        canonicalContentHashVersionManifestV1,
		"action_config_snapshot_json": actionConfigJSON,
		"action_config_hash":          actionConfigHash,
		"frame_set_hash":              frameSetHash,
		"revision_snapshot_json":      revisionSnapshotJSON,
		"revision_snapshot_hash":      revisionSnapshotHash,
		"updated_at":                  nowUTC(),
	}).Error; err != nil {
		return err
	}
	if err := w.repo.UpdateActionRevisionStatus(candidateRevID, RevisionStatusReady); err != nil {
		return err
	}
	candidateRev.ManifestPath = manifestPath
	candidateRev.ManifestHash = manifestHash
	candidateRev.ContentHash = manifestHash
	candidateRev.ActionConfigHash = actionConfigHash
	candidateRev.FrameSetHash = frameSetHash
	candidateRev.Status = RevisionStatusReady
	return w.ensureFullActionCandidate(ctx, job, snapshot, candidateRev, result.ProviderAttemptID)
}

func (w *RegenerationWorker) getFullActionRevision(job *RegenerationJob) (*ActionRevision, error) {
	rev, err := w.repo.GetActionRevision(stableFullActionRevisionID(job.ID))
	if err == ErrRevisionNotFound {
		return nil, nil
	}
	return rev, err
}

func fullActionRevisionMaterialized(rev *ActionRevision) bool {
	if rev == nil {
		return false
	}
	return rev.ManifestHash != "" && rev.ContentHash != "" && rev.FrameCount > 0 && rev.Status != RevisionStatusBuilding && rev.Status != RevisionStatusFailed
}

func (w *RegenerationWorker) ensureFullActionCandidate(ctx context.Context, job *RegenerationJob, snapshot *EditDraftSnapshot, rev *ActionRevision, providerAttemptID string) error {
	if rev == nil || !fullActionRevisionMaterialized(rev) {
		return fmt.Errorf("candidate revision is not fully materialized")
	}
	candidateID := stableFullActionCandidateID(job.ID)
	candidate, err := w.repo.GetCandidate(candidateID)
	if err == ErrCandidateNotFound {
		candidate = nil
	} else if err != nil {
		return err
	}
	if candidate != nil && (candidate.JobID != job.ID || candidate.CandidateRevisionID != rev.ID) {
		return fmt.Errorf("candidate id %s is bound to a different regeneration result", candidateID)
	}

	if candidate == nil {
		candidate = &EditCandidate{
			ID:                  candidateID,
			SessionID:           job.SessionID,
			JobID:               job.ID,
			UserID:              job.UserID,
			CharacterID:         job.CharacterID,
			ActionStreamID:      job.ActionStreamID,
			CandidateVersion:    snapshot.SessionVersion,
			DraftSnapshotID:     snapshot.ID,
			DraftSnapshotHash:   snapshot.SnapshotHash,
			CandidateType:       AssetSourceRegenerated,
			CandidateRevisionID: rev.ID,
			Status:              CandidateStatusQualityPending,
			SourceType:          canonicalSourceFullActionRegeneration,
			ParentRevisionID:    job.BaseActionRevisionID,
			ParentContentHash:   job.BaseContentHash,
			BaseBindingRevision: job.BaseBindingRevision,
			ContentHash:         rev.ContentHash,
			FrameSetHash:        rev.FrameSetHash,
			ActionConfigHash:    rev.ActionConfigHash,
			ActivationPolicy:    ActivationPolicyImmediate,
			CreatedAt:           nowUTC(),
		}
		if err := w.repo.CreateCandidate(candidate); err != nil {
			existing, readErr := w.repo.GetCandidate(candidateID)
			if readErr != nil || existing == nil || existing.JobID != job.ID || existing.CandidateRevisionID != rev.ID {
				return fmt.Errorf("创建候选失败: %w", err)
			}
			candidate = existing
		}
	}

	meta, err := w.repo.GetCandidateRevisionMetadata(rev.ID)
	if err != nil {
		return err
	}
	if meta == nil {
		now := nowUTC()
		meta = &CandidateRevisionMetadata{
			ID:                  "crm-" + uuid.NewSHA1(uuid.NameSpaceOID, []byte(job.ID+"|candidate-revision-metadata")).String(),
			CandidateRevisionID: rev.ID,
			SourceType:          canonicalSourceFullActionRegeneration,
			ParentRevisionID:    job.BaseActionRevisionID,
			ParentContentHash:   job.BaseContentHash,
			BaseBindingRevision: job.BaseBindingRevision,
			RegenerationJobID:   job.ID,
			ContentHash:         rev.ContentHash,
			FrameSetHash:        rev.FrameSetHash,
			ActionConfigHash:    rev.ActionConfigHash,
			Status:              CandidateStatusQualityPending,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
		if err := w.repo.CreateCandidateRevisionMetadata(meta); err != nil {
			existing, readErr := w.repo.GetCandidateRevisionMetadata(rev.ID)
			if readErr != nil || existing == nil || existing.RegenerationJobID != job.ID {
				return err
			}
			meta = existing
		}
	}

	evalID := candidate.QualityEvaluationID
	if evalID == "" {
		evalID, err = w.ensureRevisionQualityEvaluation(ctx, rev)
		if err != nil {
			return fmt.Errorf("触发候选Revision质量评估失败: %w", err)
		}
	}
	if err := w.repo.UpdateCandidateFields(candidateID, map[string]any{
		"status":                CandidateStatusQualityPending,
		"quality_status":        CandidateStatusQualityPending,
		"quality_evaluation_id": evalID,
		"content_hash":          rev.ContentHash,
		"frame_set_hash":        rev.FrameSetHash,
		"action_config_hash":    rev.ActionConfigHash,
	}); err != nil {
		return err
	}
	_ = w.repo.UpdateCandidateRevisionMetadataStatus(rev.ID, CandidateStatusQualityPending)
	if providerAttemptID != "" {
		if err := w.repo.UpdateJobResult(job.ID, providerAttemptID, nil); err != nil {
			return err
		}
	}
	return w.updateClaimedJob(job, map[string]any{
		"status":                JobStatusQualityPending,
		"stage":                 JobStatusQualityPending,
		"provider_attempt_id":   providerAttemptID,
		"generation_attempt_id": providerAttemptID,
		"candidate_revision_id": rev.ID,
		"quality_evaluation_id": evalID,
		"lease_owner":           "",
		"lease_expires_at":      "",
		"heartbeat_at":          "",
		"execution_id":          "",
	})
}

func (w *RegenerationWorker) buildAdjacentDraftFrames(frames []draftFrame, targetIndex int) []AdjacentFrameContext {
	var adjacent []AdjacentFrameContext
	if targetIndex > 0 {
		prev := frames[targetIndex-1]
		path, _ := w.assetStore.GetAssetPath(prev.AssetID)
		adjacent = append(adjacent, AdjacentFrameContext{FrameID: prev.FrameID, FrameIndex: targetIndex - 1, ImagePath: path})
	}
	if targetIndex < len(frames)-1 {
		next := frames[targetIndex+1]
		path, _ := w.assetStore.GetAssetPath(next.AssetID)
		adjacent = append(adjacent, AdjacentFrameContext{FrameID: next.FrameID, FrameIndex: targetIndex + 1, ImagePath: path})
	}
	return adjacent
}

func (w *RegenerationWorker) startHeartbeat(ctx context.Context, job *RegenerationJob, leaseLostCancel context.CancelFunc) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			case <-ticker.C:
				leaseExpires := time.Now().UTC().Add(w.leaseDuration).Format(time.RFC3339)
				ok, err := w.repo.UpdateJobLease(job.ID, w.workerID, job.ExecutionID, leaseExpires)
				if err != nil || !ok {
					log.Logger.Errorf("regeneration worker lost lease for job %s: %v", job.ID, err)
					leaseLostCancel()
					return
				}
			}
		}
	}()
}

func (w *RegenerationWorker) recoverStaleJobs(ctx context.Context) {
	jobs, err := w.repo.ListJobsForRecovery(w.leaseDuration)
	if err != nil {
		log.Logger.Errorf("regeneration worker list stale jobs failed: %v", err)
		return
	}
	for _, job := range jobs {
		if ctx.Err() != nil {
			return
		}
		if err := w.repo.UpdateJobFields(job.ID, map[string]any{
			"status":           JobStatusFailedRetryable,
			"lease_owner":      "",
			"lease_expires_at": "",
			"heartbeat_at":     "",
			"execution_id":     "",
		}); err != nil {
			log.Logger.Errorf("regeneration worker recover stale job %s failed: %v", job.ID, err)
			continue
		}
		log.Logger.Infof("regeneration worker recovered stale job: %s", job.ID)
	}
}

func (w *RegenerationWorker) failJob(job *RegenerationJob, errorCode, errorMessage string) {
	status := JobStatusFailedRetryable
	if job != nil && job.AttemptCount >= regenMaxAttempts {
		status = JobStatusFailedTerminal
	}
	if job == nil {
		return
	}
	fields := map[string]any{
		"status":           status,
		"error_code":       errorCode,
		"error_message":    errorMessage,
		"lease_owner":      "",
		"lease_expires_at": "",
		"heartbeat_at":     "",
		"execution_id":     "",
	}
	if status == JobStatusFailedTerminal {
		fields["completed_at"] = nowUTC()
	}
	if err := w.updateClaimedJob(job, fields); err != nil {
		log.Logger.Errorf("regeneration worker update job %s error failed: %v", job.ID, err)
	}
}
