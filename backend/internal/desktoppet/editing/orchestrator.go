package editing

import (
	"context"
	"fmt"
	"os"

	"github.com/google/uuid"
	"github.com/u-ai/backend/log"
)

type RegenerationOrchestrator struct {
	repo        Repository
	genAdapter  GenerationAdapter
	assetStore  RevisionAssetStore
	qualAdapter QualityAdapter
}

func NewRegenerationOrchestrator(repo Repository, genAdapter GenerationAdapter, assetStore RevisionAssetStore, qualAdapter QualityAdapter) *RegenerationOrchestrator {
	return &RegenerationOrchestrator{
		repo:        repo,
		genAdapter:  genAdapter,
		assetStore:  assetStore,
		qualAdapter: qualAdapter,
	}
}

func (o *RegenerationOrchestrator) Orchestrate(ctx context.Context, jobID string) error {
	job, err := o.repo.GetRegenerationJob(jobID)
	if err != nil {
		return fmt.Errorf("获取重生成Job失败: %w", err)
	}
	if job == nil {
		return fmt.Errorf("重生成Job %s 不存在", jobID)
	}
	if IsTerminalJobStatus(job.Status) {
		return nil
	}

	session, err := o.repo.GetEditSession(job.SessionID)
	if err != nil {
		o.failJob(job.ID, "SESSION_FETCH_FAILED", fmt.Sprintf("获取会话失败: %v", err))
		return fmt.Errorf("获取会话失败: %w", err)
	}
	if session == nil {
		o.failJob(job.ID, "SESSION_NOT_FOUND", "会话不存在")
		return fmt.Errorf("会话 %s 不存在", job.SessionID)
	}

	baseRev, err := o.repo.GetActionRevision(session.BaseRevisionID)
	if err != nil {
		o.failJob(job.ID, "REVISION_FETCH_FAILED", fmt.Sprintf("获取基线Revision失败: %v", err))
		return fmt.Errorf("获取基线Revision失败: %w", err)
	}
	if baseRev == nil {
		o.failJob(job.ID, "REVISION_NOT_FOUND", "基线Revision不存在")
		return fmt.Errorf("基线Revision %s 不存在", session.BaseRevisionID)
	}

	o.writeJournal(&RegenerationJournal{
		JobID: job.ID,
		State: JournalStatePlanCreated,
	})

	mode := RegenModeSingleFrame
	if job.JobType == JobTypeFullAction {
		mode = RegenModeFullAction
	}
	if err := o.repo.UpdateJobFields(job.ID, map[string]any{
		"status":                  JobStatusPreparing,
		"mode":                    mode,
		"base_action_revision_id": baseRev.ID,
		"base_content_hash":       baseRev.ContentHash,
		"base_binding_revision":   session.BaseBindingRevision,
	}); err != nil {
		o.failJob(job.ID, "JOB_UPDATE_FAILED", fmt.Sprintf("更新Job准备状态失败: %v", err))
		return fmt.Errorf("更新Job准备状态失败: %w", err)
	}

	var processErr error
	switch job.JobType {
	case JobTypeSingleFrame:
		processErr = o.processSingleFrame(ctx, job, session, baseRev)
	case JobTypeFullAction:
		processErr = o.processFullAction(ctx, job, session, baseRev)
	default:
		processErr = fmt.Errorf("未知Job类型: %s", job.JobType)
	}

	if processErr != nil {
		o.failJob(job.ID, "PROCESS_FAILED", processErr.Error())
		return processErr
	}

	return nil
}

func (o *RegenerationOrchestrator) processSingleFrame(ctx context.Context, job *RegenerationJob, session *EditSession, baseRev *ActionRevision) error {
	if err := o.repo.UpdateJobStatus(job.ID, JobStatusSubmitting); err != nil {
		return fmt.Errorf("更新Job状态为submitting失败: %w", err)
	}

	frames, err := o.repo.ListRevisionFrames(baseRev.ID)
	if err != nil {
		return fmt.Errorf("获取Revision帧列表失败: %w", err)
	}

	targetIndex := -1
	for i := range frames {
		if frames[i].FrameID == job.TargetFrameID {
			targetIndex = i
			break
		}
	}
	if targetIndex < 0 {
		return fmt.Errorf("目标帧 %s 不存在", job.TargetFrameID)
	}

	adjacentFrames := o.buildAdjacentFrames(frames, targetIndex)

	result, err := o.genAdapter.GenerateSingleFrame(ctx, SingleFrameGenerationRequest{
		GenerationTaskID: baseRev.GenerationTaskID,
		ActionKey:        job.ActionKey,
		TargetFrameID:    job.TargetFrameID,
		FrameIndex:       targetIndex,
		TotalFrames:      len(frames),
		AdjacentFrames:   adjacentFrames,
		UserID:           session.UserID,
	})
	if err != nil {
		return fmt.Errorf("提交单帧生成请求失败: %w", err)
	}

	attemptID := uuid.NewString()
	o.writeJournal(&RegenerationJournal{
		JobID:     job.ID,
		State:     JournalStateProviderSubmitted,
		AttemptID: attemptID,
	})

	if err := o.repo.UpdateJobFields(job.ID, map[string]any{
		"active_attempt_id":   attemptID,
		"provider_attempt_id": result.ProviderAttemptID,
	}); err != nil {
		return fmt.Errorf("更新Job尝试信息失败: %w", err)
	}

	if err := o.repo.UpdateJobStatus(job.ID, JobStatusArtifactReady); err != nil {
		return fmt.Errorf("更新Job状态为artifact_ready失败: %w", err)
	}

	imgData, err := os.ReadFile(result.ImagePath)
	if err != nil {
		return fmt.Errorf("读取生成图片失败: %w", err)
	}

	asset, err := o.assetStore.WriteAsset(ctx, imgData, "image/png", AssetSourceRegenerated, result.ProviderAttemptID)
	if err != nil {
		return fmt.Errorf("持久化生成资产失败: %w", err)
	}

	o.writeJournal(&RegenerationJournal{
		JobID:      job.ID,
		State:      JournalStateArtifactPersisted,
		ArtifactID: asset.ID,
	})

	if err := o.repo.UpdateJobFields(job.ID, map[string]any{
		"artifact_id": asset.ID,
	}); err != nil {
		return fmt.Errorf("更新Job产物信息失败: %w", err)
	}

	now := nowUTC()
	candidateID := generateID("cand")
	candidate := &EditCandidate{
		ID:                  candidateID,
		SessionID:           job.SessionID,
		JobID:               job.ID,
		TargetFrameID:       job.TargetFrameID,
		CandidateType:       AssetSourceRegenerated,
		AssetID:             asset.ID,
		Status:              CandidateStatusPending,
		SourceType:          CandidateSourceSingleFrame,
		ParentRevisionID:    baseRev.ID,
		BaseBindingRevision: session.BaseBindingRevision,
		CreatedAt:           now,
	}
	if err := o.repo.CreateCandidate(candidate); err != nil {
		return fmt.Errorf("创建候选失败: %w", err)
	}

	o.writeJournal(&RegenerationJournal{
		JobID:               job.ID,
		State:               JournalStateCandidateCreated,
		CandidateRevisionID: candidateID,
	})

	if err := o.repo.UpdateJobStatus(job.ID, JobStatusReadyForReview); err != nil {
		return fmt.Errorf("更新Job状态为ready_for_review失败: %w", err)
	}

	o.writeJournal(&RegenerationJournal{
		JobID: job.ID,
		State: JournalStateReadyForReview,
	})

	return nil
}

func (o *RegenerationOrchestrator) processFullAction(ctx context.Context, job *RegenerationJob, session *EditSession, baseRev *ActionRevision) error {
	if err := o.repo.UpdateJobStatus(job.ID, JobStatusSubmitting); err != nil {
		return fmt.Errorf("更新Job状态为submitting失败: %w", err)
	}

	result, err := o.genAdapter.GenerateFullAction(ctx, FullActionGenerationRequest{
		GenerationTaskID: baseRev.GenerationTaskID,
		ActionKey:        job.ActionKey,
		UserID:           session.UserID,
	})
	if err != nil {
		return fmt.Errorf("提交整动作生成请求失败: %w", err)
	}

	attemptID := uuid.NewString()
	o.writeJournal(&RegenerationJournal{
		JobID:     job.ID,
		State:     JournalStateProviderSubmitted,
		AttemptID: attemptID,
	})

	if err := o.repo.UpdateJobFields(job.ID, map[string]any{
		"active_attempt_id":   attemptID,
		"provider_attempt_id": result.ProviderAttemptID,
	}); err != nil {
		return fmt.Errorf("更新Job尝试信息失败: %w", err)
	}

	if err := o.repo.UpdateJobStatus(job.ID, JobStatusArtifactReady); err != nil {
		return fmt.Errorf("更新Job状态为artifact_ready失败: %w", err)
	}

	framePaths := result.FramePaths
	if len(framePaths) == 0 {
		artifacts, artErr := o.genAdapter.GetGenerationArtifacts(ctx, baseRev.GenerationTaskID, job.ActionKey, 0)
		if artErr != nil {
			return fmt.Errorf("获取生成产物失败: %w", artErr)
		}
		for _, art := range artifacts {
			if art.AttemptID == result.ProviderAttemptID {
				framePaths = append(framePaths, art.ImagePath)
			}
		}
	}
	if len(framePaths) == 0 {
		return fmt.Errorf("未找到生成产物")
	}

	if err := o.repo.UpdateJobStatus(job.ID, JobStatusProcessing); err != nil {
		return fmt.Errorf("更新Job状态为processing失败: %w", err)
	}

	revNum, err := o.repo.GetNextRevisionNumber(session.ProcessingTaskID, session.ActionKey)
	if err != nil {
		return fmt.Errorf("获取下一个Revision号失败: %w", err)
	}

	now := nowUTC()
	rootRevID := baseRev.RootRevisionID
	if rootRevID == "" {
		rootRevID = baseRev.ID
	}
	candidateRevID := uuid.NewString()
	candidateRev := &ActionRevision{
		ID:                   candidateRevID,
		ProcessingTaskID:     session.ProcessingTaskID,
		GenerationTaskID:     baseRev.GenerationTaskID,
		ActionKey:            session.ActionKey,
		ParentRevisionID:     baseRev.ID,
		RootRevisionID:       rootRevID,
		RevisionNumber:       revNum,
		RevisionType:         RevisionTypeRegenerated,
		Status:               RevisionStatusBuilding,
		FrameCount:           len(framePaths),
		DefaultFPS:           baseRev.DefaultFPS,
		LoopType:             baseRev.LoopType,
		ReturnAction:         baseRev.ReturnAction,
		Interruptible:        baseRev.Interruptible,
		CreatedByUserID:      session.UserID,
		CreatedFromSessionID: session.ID,
		ChangeSummary:        "整动作重生成候选",
		SourceType:           AssetSourceRegenerated,
		Origin:               AttemptOriginEditingRegen,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := o.repo.CreateActionRevision(candidateRev); err != nil {
		return fmt.Errorf("创建候选Revision失败: %w", err)
	}

	revFrames := make([]ActionRevisionFrame, 0, len(framePaths))
	for i, framePath := range framePaths {
		imgData, readErr := os.ReadFile(framePath)
		if readErr != nil {
			return fmt.Errorf("读取生成帧 %d 失败: %w", i, readErr)
		}
		asset, assetErr := o.assetStore.WriteAsset(ctx, imgData, "image/png", AssetSourceRegenerated, result.ProviderAttemptID)
		if assetErr != nil {
			return fmt.Errorf("持久化生成帧 %d 失败: %w", i, assetErr)
		}
		frameID := fmt.Sprintf("frame-%s-%d", candidateRevID[:8], i)
		revFrames = append(revFrames, ActionRevisionFrame{
			ID:               generateID("rf"),
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
	}
	if err := o.repo.CreateRevisionFrames(revFrames); err != nil {
		return fmt.Errorf("创建Revision帧失败: %w", err)
	}

	meta := &CandidateRevisionMetadata{
		ID:                  generateID("crm"),
		CandidateRevisionID: candidateRevID,
		SourceType:          CandidateSourceFullAction,
		ParentRevisionID:    baseRev.ID,
		BaseBindingRevision: session.BaseBindingRevision,
		RegenerationJobID:   job.ID,
		Status:              RevisionStatusBuilding,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := o.repo.CreateCandidateRevisionMetadata(meta); err != nil {
		return fmt.Errorf("创建候选Revision元数据失败: %w", err)
	}

	o.writeJournal(&RegenerationJournal{
		JobID:               job.ID,
		State:               JournalStateCandidateCreated,
		CandidateRevisionID: candidateRevID,
	})

	if err := o.repo.UpdateJobFields(job.ID, map[string]any{
		"candidate_revision_id":  candidateRevID,
		"processing_revision_id": candidateRevID,
	}); err != nil {
		return fmt.Errorf("更新Job候选Revision信息失败: %w", err)
	}

	if err := o.repo.UpdateActionRevisionStatus(candidateRevID, RevisionStatusReady); err != nil {
		return fmt.Errorf("更新候选Revision状态失败: %w", err)
	}

	candidateID := generateID("cand")
	candidate := &EditCandidate{
		ID:                  candidateID,
		SessionID:           job.SessionID,
		JobID:               job.ID,
		TargetFrameID:       job.TargetFrameID,
		CandidateType:       AssetSourceRegenerated,
		CandidateRevisionID: candidateRevID,
		Status:              CandidateStatusReadyForReview,
		SourceType:          CandidateSourceFullAction,
		ParentRevisionID:    baseRev.ID,
		BaseBindingRevision: session.BaseBindingRevision,
		CreatedAt:           nowUTC(),
	}
	if err := o.repo.CreateCandidate(candidate); err != nil {
		return fmt.Errorf("创建候选失败: %w", err)
	}

	if err := o.repo.UpdateJobStatus(job.ID, JobStatusQualityPending); err != nil {
		return fmt.Errorf("更新Job状态为quality_pending失败: %w", err)
	}

	evaluationID, evalErr := o.qualAdapter.EvaluateRevision(ctx, candidateRevID)
	if evalErr != nil {
		log.Logger.Warnf("regeneration orchestrator trigger quality evaluation for revision %s failed: %v", candidateRevID, evalErr)
	}

	o.writeJournal(&RegenerationJournal{
		JobID:               job.ID,
		State:               JournalStateQualityCreated,
		QualityEvaluationID: evaluationID,
	})

	if err := o.repo.UpdateJobStatus(job.ID, JobStatusReadyForReview); err != nil {
		return fmt.Errorf("更新Job状态为ready_for_review失败: %w", err)
	}

	o.writeJournal(&RegenerationJournal{
		JobID: job.ID,
		State: JournalStateReadyForReview,
	})

	return nil
}

func (o *RegenerationOrchestrator) buildAdjacentFrames(frames []ActionRevisionFrame, targetIndex int) []AdjacentFrameContext {
	var adjacent []AdjacentFrameContext
	if targetIndex > 0 {
		prev := frames[targetIndex-1]
		path, _ := o.assetStore.GetAssetPath(prev.AssetID)
		adjacent = append(adjacent, AdjacentFrameContext{
			FrameID:    prev.FrameID,
			FrameIndex: targetIndex - 1,
			ImagePath:  path,
		})
	}
	if targetIndex < len(frames)-1 {
		next := frames[targetIndex+1]
		path, _ := o.assetStore.GetAssetPath(next.AssetID)
		adjacent = append(adjacent, AdjacentFrameContext{
			FrameID:    next.FrameID,
			FrameIndex: targetIndex + 1,
			ImagePath:  path,
		})
	}
	return adjacent
}

func (o *RegenerationOrchestrator) writeJournal(j *RegenerationJournal) {
	if j.ID == "" {
		j.ID = generateID("journal")
	}
	if j.CreatedAt == "" {
		j.CreatedAt = nowUTC()
	}
	if err := o.repo.CreateRegenerationJournal(j); err != nil {
		log.Logger.Errorf("regeneration orchestrator write journal failed: %v", err)
	}
}

func (o *RegenerationOrchestrator) failJob(jobID, errorCode, errorMessage string) {
	o.writeJournal(&RegenerationJournal{
		JobID:        jobID,
		State:        JournalStateFailed,
		ErrorMessage: errorMessage,
	})
	if err := o.repo.UpdateJobFields(jobID, map[string]any{
		"status":        JobStatusFailedRetryable,
		"error_code":    errorCode,
		"error_message": errorMessage,
	}); err != nil {
		log.Logger.Errorf("regeneration orchestrator fail job %s failed: %v", jobID, err)
	}
}
