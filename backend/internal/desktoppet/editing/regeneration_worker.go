package editing

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/u-ai/backend/log"
)

const (
	regenLeaseDuration     = 5 * time.Minute
	regenPollInterval      = 5 * time.Second
	regenHeartbeatInterval = 30 * time.Second
	regenWorkerID          = "regen-worker-1"
	regenMaxConcurrency    = 1
)

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

func (w *RegenerationWorker) pollAndProcess(ctx context.Context) {
	jobs, err := w.repo.ListPendingJobs()
	if err != nil {
		log.Logger.Errorf("regeneration worker poll jobs failed: %v", err)
		return
	}
	if len(jobs) == 0 {
		return
	}
	for i := range jobs {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case w.sem <- struct{}{}:
		}
		job := jobs[i]
		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			defer func() { <-w.sem }()
			w.processJob(ctx, &job)
		}()
	}
}

func (w *RegenerationWorker) processJob(ctx context.Context, job *RegenerationJob) {
	if job.Status == JobStatusCompleted || job.Status == JobStatusFailed || job.Status == JobStatusCancelled {
		return
	}

	if job.Status == JobStatusCreated || job.Status == JobStatusQueued {
		if err := w.repo.UpdateJobStatus(job.ID, JobStatusRunning); err != nil {
			log.Logger.Errorf("regeneration worker update job %s to running failed: %v", job.ID, err)
			return
		}
	}

	jobCtx, jobCancel := context.WithCancel(ctx)
	defer jobCancel()

	heartbeatCtx, heartbeatCancel := context.WithCancel(jobCtx)
	w.startHeartbeat(heartbeatCtx, job.ID, jobCancel)
	defer heartbeatCancel()

	session, err := w.repo.GetEditSession(job.SessionID)
	if err != nil {
		w.failJob(job.ID, "SESSION_FETCH_FAILED", fmt.Sprintf("获取会话失败: %v", err))
		return
	}
	if session == nil {
		w.failJob(job.ID, "SESSION_NOT_FOUND", "会话不存在")
		return
	}

	baseRev, err := w.repo.GetActionRevision(session.BaseRevisionID)
	if err != nil {
		w.failJob(job.ID, "REVISION_FETCH_FAILED", fmt.Sprintf("获取基线Revision失败: %v", err))
		return
	}

	var processErr error
	switch job.JobType {
	case JobTypeSingleFrame:
		processErr = w.processSingleFrameJob(ctx, job, session, baseRev)
	case JobTypeFullAction:
		processErr = w.processFullActionJob(ctx, job, session, baseRev)
	case JobTypeBackgroundReprocess:
		processErr = w.processSingleFrameJob(ctx, job, session, baseRev)
	case JobTypeNormalizeUpload:
		processErr = w.processSingleFrameJob(ctx, job, session, baseRev)
	default:
		processErr = fmt.Errorf("未知 job 类型: %s", job.JobType)
	}

	if processErr != nil {
		w.failJob(job.ID, "PROCESS_FAILED", processErr.Error())
		return
	}

	if err := w.repo.UpdateJobStatus(job.ID, JobStatusCompleted); err != nil {
		log.Logger.Errorf("regeneration worker update job %s to completed failed: %v", job.ID, err)
	}
	log.Logger.Infof("regeneration worker completed job: %s", job.ID)
}

func (w *RegenerationWorker) processSingleFrameJob(ctx context.Context, job *RegenerationJob, session *EditSession, baseRev *ActionRevision) error {
	frames, err := w.repo.ListRevisionFrames(session.BaseRevisionID)
	if err != nil {
		return fmt.Errorf("获取Revision帧失败: %w", err)
	}

	var targetFrame *ActionRevisionFrame
	targetIndex := -1
	for i := range frames {
		if frames[i].FrameID == job.TargetFrameID {
			targetFrame = &frames[i]
			targetIndex = i
			break
		}
	}
	if targetFrame == nil {
		return fmt.Errorf("目标帧 %s 不存在", job.TargetFrameID)
	}

	adjacentFrames := w.buildAdjacentFrames(frames, targetIndex)

	result, err := w.genAdapter.GenerateSingleFrame(ctx, SingleFrameGenerationRequest{
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

	if err := w.repo.UpdateJobStatus(job.ID, JobStatusProviderSucceeded); err != nil {
		return fmt.Errorf("更新job状态为provider_succeeded失败: %w", err)
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

	if err := w.repo.UpdateJobStatus(job.ID, JobStatusMaterializing); err != nil {
		return fmt.Errorf("更新job状态为materializing失败: %w", err)
	}

	imgData, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("读取生成图片失败: %w", err)
	}

	asset, err := w.assetStore.WriteAsset(ctx, imgData, "image/png", AssetSourceRegenerated, result.ProviderAttemptID)
	if err != nil {
		return fmt.Errorf("持久化生成资产失败: %w", err)
	}

	now := nowUTC()
	candidateID := generateID("cand")
	candidate := &EditCandidate{
		ID:            candidateID,
		SessionID:     job.SessionID,
		JobID:         job.ID,
		TargetFrameID: job.TargetFrameID,
		CandidateType: AssetSourceRegenerated,
		AssetID:       asset.ID,
		Status:        CandidateStatusPending,
		CreatedAt:     now,
	}
	if err := w.repo.CreateCandidate(candidate); err != nil {
		return fmt.Errorf("创建候选失败: %w", err)
	}

	if err := w.repo.UpdateJobResult(job.ID, result.ProviderAttemptID, nil); err != nil {
		return fmt.Errorf("更新job结果失败: %w", err)
	}

	return nil
}

func (w *RegenerationWorker) processFullActionJob(ctx context.Context, job *RegenerationJob, session *EditSession, baseRev *ActionRevision) error {
	result, err := w.genAdapter.GenerateFullAction(ctx, FullActionGenerationRequest{
		GenerationTaskID: baseRev.GenerationTaskID,
		ActionKey:        job.ActionKey,
		UserID:           session.UserID,
	})
	if err != nil {
		return fmt.Errorf("提交整动作生成请求失败: %w", err)
	}

	if err := w.repo.UpdateJobStatus(job.ID, JobStatusProviderSucceeded); err != nil {
		return fmt.Errorf("更新job状态为provider_succeeded失败: %w", err)
	}

	framePaths := result.FramePaths
	if len(framePaths) == 0 {
		artifacts, artErr := w.genAdapter.GetGenerationArtifacts(ctx, baseRev.GenerationTaskID, job.ActionKey, 0)
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

	if err := w.repo.UpdateJobStatus(job.ID, JobStatusMaterializing); err != nil {
		return fmt.Errorf("更新job状态为materializing失败: %w", err)
	}

	revNum, err := w.repo.GetNextRevisionNumber(session.ProcessingTaskID, session.ActionKey)
	if err != nil {
		return fmt.Errorf("获取下一个Revision号失败: %w", err)
	}

	now := nowUTC()
	rootRevID := baseRev.RootRevisionID
	if rootRevID == "" {
		rootRevID = baseRev.ID
	}
	candidateRevID := generateID("rev")
	candidateRev := &ActionRevision{
		ID:                   candidateRevID,
		ProcessingTaskID:     session.ProcessingTaskID,
		GenerationTaskID:     baseRev.GenerationTaskID,
		ActionKey:            session.ActionKey,
		ParentRevisionID:     session.BaseRevisionID,
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
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := w.repo.CreateActionRevision(candidateRev); err != nil {
		return fmt.Errorf("创建候选Revision失败: %w", err)
	}

	revFrames := make([]ActionRevisionFrame, 0, len(framePaths))
	for i, framePath := range framePaths {
		imgData, readErr := os.ReadFile(framePath)
		if readErr != nil {
			return fmt.Errorf("读取生成帧 %d 失败: %w", i, readErr)
		}
		asset, assetErr := w.assetStore.WriteAsset(ctx, imgData, "image/png", AssetSourceRegenerated, result.ProviderAttemptID)
		if assetErr != nil {
			return fmt.Errorf("持久化生成帧 %d 失败: %w", i, assetErr)
		}
		frameID := fmt.Sprintf("frame-regen-%d-%d", time.Now().UnixNano(), i)
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
	if err := w.repo.CreateRevisionFrames(revFrames); err != nil {
		return fmt.Errorf("创建Revision帧失败: %w", err)
	}

	if err := w.repo.UpdateActionRevisionStatus(candidateRevID, RevisionStatusReady); err != nil {
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
		Status:              CandidateStatusPending,
		CreatedAt:           now,
	}
	if err := w.repo.CreateCandidate(candidate); err != nil {
		return fmt.Errorf("创建候选失败: %w", err)
	}

	if _, evalErr := w.qualAdapter.EvaluateRevision(ctx, candidateRevID); evalErr != nil {
		log.Logger.Warnf("regeneration worker trigger quality evaluation for revision %s failed: %v", candidateRevID, evalErr)
	}

	if err := w.repo.UpdateJobResult(job.ID, result.ProviderAttemptID, nil); err != nil {
		return fmt.Errorf("更新job结果失败: %w", err)
	}

	return nil
}

func (w *RegenerationWorker) buildAdjacentFrames(frames []ActionRevisionFrame, targetIndex int) []AdjacentFrameContext {
	var adjacent []AdjacentFrameContext
	if targetIndex > 0 {
		prev := frames[targetIndex-1]
		path, _ := w.assetStore.GetAssetPath(prev.AssetID)
		adjacent = append(adjacent, AdjacentFrameContext{
			FrameID:    prev.FrameID,
			FrameIndex: targetIndex - 1,
			ImagePath:  path,
		})
	}
	if targetIndex < len(frames)-1 {
		next := frames[targetIndex+1]
		path, _ := w.assetStore.GetAssetPath(next.AssetID)
		adjacent = append(adjacent, AdjacentFrameContext{
			FrameID:    next.FrameID,
			FrameIndex: targetIndex + 1,
			ImagePath:  path,
		})
	}
	return adjacent
}

func (w *RegenerationWorker) startHeartbeat(ctx context.Context, jobID string, leaseLostCancel context.CancelFunc) {
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
				if err := w.repo.UpdateJobHeartbeat(jobID); err != nil {
					log.Logger.Errorf("regeneration worker heartbeat for job %s failed: %v", jobID, err)
				}
			}
		}
	}()
}

func (w *RegenerationWorker) recoverStaleJobs(ctx context.Context) {
	jobs, err := w.repo.ListStaleJobs(w.leaseDuration)
	if err != nil {
		log.Logger.Errorf("regeneration worker list stale jobs failed: %v", err)
		return
	}
	for _, job := range jobs {
		if ctx.Err() != nil {
			return
		}
		if err := w.repo.UpdateJobStatus(job.ID, JobStatusQueued); err != nil {
			log.Logger.Errorf("regeneration worker recover stale job %s failed: %v", job.ID, err)
			continue
		}
		log.Logger.Infof("regeneration worker recovered stale job: %s", job.ID)
	}
}

func (w *RegenerationWorker) failJob(jobID, errorCode, errorMessage string) {
	if err := w.repo.UpdateJobError(jobID, errorCode, errorMessage); err != nil {
		log.Logger.Errorf("regeneration worker update job %s error failed: %v", jobID, err)
	}
}
