// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/u-ai/backend/internal/desktoppet"
	"github.com/u-ai/backend/internal/desktoppet/processing"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

func e2eSeedAction(t *testing.T, db *gorm.DB, actionID, taskID, actionKey, status string, supportsIdle, sortOrder, frameCount int) desktoppet.GenerationTaskAction {
	t.Helper()
	action := desktoppet.GenerationTaskAction{
		ID:                    actionID,
		TaskID:                taskID,
		ActionKey:             actionKey,
		ActionNameSnapshot:    actionKey,
		Status:                status,
		CurrentAttempt:        1,
		FrameCount:            frameCount,
		GenerationSpecVersion: "v1",
		SupportsDefaultIdle:   supportsIdle,
		SortOrder:             sortOrder,
	}
	if err := db.Create(&action).Error; err != nil {
		t.Fatalf("创建动作 %s 失败: %v", actionID, err)
	}
	return action
}

func e2eWriteColoredPNG(t *testing.T, dataDir, relPath string, width, height, frameIndex int, baseColor color.NRGBA) string {
	t.Helper()
	absPath := filepath.Join(dataDir, filepath.FromSlash(relPath))
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		t.Fatalf("创建目录 %s 失败: %v", absPath, err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	r := baseColor.R + uint8(frameIndex*10)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if x >= width/4 && x < width*3/4 && y >= height/4 && y < height*3/4 {
				img.Set(x, y, color.NRGBA{R: r, G: baseColor.G, B: baseColor.B, A: 255})
			} else {
				img.Set(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 0})
			}
		}
	}
	var buf []byte
	_ = buf
	f, err := os.Create(absPath)
	if err != nil {
		t.Fatalf("创建文件 %s 失败: %v", absPath, err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("编码 PNG 失败: %v", err)
	}
	return absPath
}

func TestE2EFourActionsAcceptance(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := newWorkerRepo(t, db)
	dataDir := t.TempDir()
	w := NewWorker(db, repo, dataDir)

	taskID := "gt-e2e-four-actions"
	userID := "user-e2e"
	seedWorkerGenerationTask(t, db, taskID, userID, "succeeded")

	actionDefs := []struct {
		key          string
		supportsIdle int
		sortOrder    int
		color        color.NRGBA
	}{
		{"idle_normal", 1, 10, color.NRGBA{R: 200, G: 100, B: 100, A: 255}},
		{"wave", 0, 30, color.NRGBA{R: 100, G: 200, B: 100, A: 255}},
		{"happy", 0, 40, color.NRGBA{R: 100, G: 100, B: 200, A: 255}},
		{"speaking", 0, 72, color.NRGBA{R: 200, G: 200, B: 100, A: 255}},
	}

	frameCount := 3
	for i, a := range actionDefs {
		actionID := fmt.Sprintf("gta-e2e-%d", i+1)
		e2eSeedAction(t, db, actionID, taskID, a.key, "succeeded", a.supportsIdle, a.sortOrder, frameCount)
		for j := 0; j < frameCount; j++ {
			relPath := fmt.Sprintf("desktop-pets/generation-tasks/%s/generated/%s/attempt-1/raw/frame-%04d.png", taskID, a.key, j+1)
			absPath := e2eWriteColoredPNG(t, dataDir, relPath, 64, 64, j, a.color)
			hash := fileSHA256Hex(t, absPath)
			seedWorkerFrame(t, db, fmt.Sprintf("gf-e2e-%d-%d", i+1, j+1), taskID, actionID, relPath, hash, j, 1, "succeeded")
		}
	}

	pt := &processing.ProcessingTask{
		ID:                         "pt-e2e-four-actions",
		GenerationTaskID:           taskID,
		ProcessingVersion:          1,
		Status:                     "queued",
		OutputWidth:                512,
		OutputHeight:               512,
		TargetCharacterHeightRatio: 0.8,
		AnchorMode:                 "feet_center",
		BackgroundMode:             "remove_background",
		DefaultFPS:                 10,
	}
	createProcessingTask(t, repo, db, pt)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	w.processTask(ctx, pt)

	gotTask, err := repo.GetProcessingTask(pt.ID)
	if err != nil {
		t.Fatalf("获取处理任务失败: %v", err)
	}
	if gotTask.Status != "succeeded" {
		t.Fatalf("处理任务状态 = %s, 期望 succeeded, 错误信息: %s", gotTask.Status, gotTask.ErrorMessage)
	}

	t.Run("SubTask33.2_处理记录与帧质量", func(t *testing.T) {
		actions, err := repo.ListProcessingActions(pt.ID)
		if err != nil {
			t.Fatalf("查询处理动作失败: %v", err)
		}
		if len(actions) != 4 {
			t.Fatalf("处理动作数量 = %d, 期望 4", len(actions))
		}
		for _, a := range actions {
			if a.Status != "succeeded" {
				t.Fatalf("动作 %s 状态 = %s, 期望 succeeded, 错误: %s", a.ActionKey, a.Status, a.ErrorMessage)
			}
		}

		subjectDetector := processing.NewSubjectDetector()
		actionSubjectSizes := make(map[string][2]int)
		for _, a := range actionDefs {
			framesDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "processed", "version-1", "actions", a.key, "frames")
			entries, err := os.ReadDir(framesDir)
			if err != nil {
				t.Fatalf("读取动作 %s 帧目录失败: %v", a.key, err)
			}
			frameFiles := []os.DirEntry{}
			for _, e := range entries {
				if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
					frameFiles = append(frameFiles, e)
				}
			}
			if len(frameFiles) == 0 {
				t.Fatalf("动作 %s 无处理后帧文件", a.key)
			}

			var firstSize [2]int
			for i, e := range frameFiles {
				framePath := filepath.Join(framesDir, e.Name())
				f, err := os.Open(framePath)
				if err != nil {
					t.Fatalf("打开帧文件 %s 失败: %v", framePath, err)
				}
				img, err := png.Decode(f)
				f.Close()
				if err != nil {
					t.Fatalf("解码帧文件 %s 失败: %v", framePath, err)
				}

				_, isNRGBA := img.(*image.NRGBA)
				_, isRGBA := img.(*image.RGBA)
				if !isNRGBA && !isRGBA {
					t.Fatalf("动作 %s 帧 %s 不含 alpha 通道", a.key, e.Name())
				}

				bounds := img.Bounds()
				if bounds.Dx() != 512 || bounds.Dy() != 512 {
					t.Fatalf("动作 %s 帧 %s 尺寸 = %dx%d, 期望 512x512", a.key, e.Name(), bounds.Dx(), bounds.Dy())
				}

				hasTransparent := false
				hasOpaque := false
				for y := bounds.Min.Y; y < bounds.Max.Y && !(hasTransparent && hasOpaque); y++ {
					for x := bounds.Min.X; x < bounds.Max.X; x++ {
						_, _, _, alpha := img.At(x, y).RGBA()
						if alpha == 0 {
							hasTransparent = true
						} else if alpha == 65535 {
							hasOpaque = true
						}
						if hasTransparent && hasOpaque {
							break
						}
					}
				}
				if !hasTransparent {
					t.Fatalf("动作 %s 帧 %s 无透明像素，背景未移除", a.key, e.Name())
				}
				if !hasOpaque {
					t.Fatalf("动作 %s 帧 %s 无不透明像素，主体缺失", a.key, e.Name())
				}

				box, err := subjectDetector.DetectSubject(img)
				if err != nil {
					t.Fatalf("动作 %s 帧 %s 主体检测失败: %v", a.key, e.Name(), err)
				}
				if box.Empty {
					t.Fatalf("动作 %s 帧 %s 未检测到主体", a.key, e.Name())
				}
				size := [2]int{box.Width, box.Height}
				if i == 0 {
					firstSize = size
				} else if size != firstSize {
					t.Fatalf("动作 %s 帧间主体大小不稳定: 帧0=%dx%d, 帧%d=%dx%d", a.key, firstSize[0], firstSize[1], i, size[0], size[1])
				}
			}
			actionSubjectSizes[a.key] = firstSize
		}

		var refSize [2]int
		var refKey string
		for _, a := range actionDefs {
			size := actionSubjectSizes[a.key]
			if refKey == "" {
				refSize = size
				refKey = a.key
			} else if size != refSize {
				t.Fatalf("不同动作角色基础尺寸不一致: %s=%dx%d, %s=%dx%d", refKey, refSize[0], refSize[1], a.key, size[0], size[1])
			}
		}
	})

	t.Run("SubTask33.3_默认动作与资源文件", func(t *testing.T) {
		succeededActions, err := repo.ListSucceededActions(taskID)
		if err != nil {
			t.Fatalf("查询成功动作失败: %v", err)
		}
		selector := processing.NewDefaultActionSelector("")
		defaultAction, err := selector.SelectDefaultAction(succeededActions)
		if err != nil {
			t.Fatalf("选择默认动作失败: %v", err)
		}
		if defaultAction != "idle_normal" {
			t.Fatalf("默认动作 = %s, 期望 idle_normal", defaultAction)
		}

		for _, a := range actionDefs {
			actionDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "processed", "version-1", "actions", a.key)
			actionJSONPath := filepath.Join(actionDir, "action.json")
			if _, err := os.Stat(actionJSONPath); err != nil {
				t.Fatalf("动作 %s 的 action.json 不存在: %v", a.key, err)
			}
			previewPath := filepath.Join(actionDir, "preview.png")
			if _, err := os.Stat(previewPath); err != nil {
				t.Fatalf("动作 %s 的 preview.png 不存在: %v", a.key, err)
			}

			data, err := os.ReadFile(actionJSONPath)
			if err != nil {
				t.Fatalf("读取动作 %s 的 action.json 失败: %v", a.key, err)
			}
			var actionJSON processing.ActionJSON
			if err := json.Unmarshal(data, &actionJSON); err != nil {
				t.Fatalf("解析动作 %s 的 action.json 失败: %v", a.key, err)
			}
			if actionJSON.Key != a.key {
				t.Fatalf("动作 %s 的 action.json key = %s, 期望 %s", a.key, actionJSON.Key, a.key)
			}
			if actionJSON.FrameCount <= 0 {
				t.Fatalf("动作 %s 的 action.json frameCount = %d, 期望 > 0", a.key, actionJSON.FrameCount)
			}
			if actionJSON.Fps <= 0 {
				t.Fatalf("动作 %s 的 action.json fps = %d, 期望 > 0", a.key, actionJSON.Fps)
			}
		}
	})

	t.Run("SubTask33.4_资源包与完整性", func(t *testing.T) {
		packages, err := repo.ListPackagesByGenerationTask(taskID)
		if err != nil {
			t.Fatalf("查询资源包失败: %v", err)
		}
		if len(packages) != 1 {
			t.Fatalf("资源包数量 = %d, 期望 1", len(packages))
		}
		pkg := packages[0]
		if pkg.Status != "ready" {
			t.Fatalf("资源包状态 = %s, 期望 ready", pkg.Status)
		}
		if pkg.DefaultActionKey != "idle_normal" {
			t.Fatalf("默认动作 = %s, 期望 idle_normal", pkg.DefaultActionKey)
		}
		if pkg.ActionCount != 4 {
			t.Fatalf("动作数量 = %d, 期望 4", pkg.ActionCount)
		}
		if pkg.PackageHash == "" {
			t.Fatal("资源包哈希为空")
		}

		packageDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "packages", pkg.ID)
		manifestPath := filepath.Join(packageDir, "manifest.json")
		manifestData, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatalf("读取 manifest.json 失败: %v", err)
		}
		var manifest processing.Manifest
		if err := json.Unmarshal(manifestData, &manifest); err != nil {
			t.Fatalf("解析 manifest.json 失败: %v", err)
		}
		if manifest.SchemaVersion != processing.ManifestSchemaVersion {
			t.Fatalf("schemaVersion = %d, 期望 %d", manifest.SchemaVersion, processing.ManifestSchemaVersion)
		}
		if len(manifest.Actions) != 4 {
			t.Fatalf("manifest 动作数量 = %d, 期望 4", len(manifest.Actions))
		}
		if manifest.DefaultAction != "idle_normal" {
			t.Fatalf("manifest defaultAction = %s, 期望 idle_normal", manifest.DefaultAction)
		}
		manifestActionKeys := map[string]bool{}
		for _, ma := range manifest.Actions {
			manifestActionKeys[ma.Key] = true
		}
		for _, a := range actionDefs {
			if !manifestActionKeys[a.key] {
				t.Fatalf("manifest 中缺少动作 %s", a.key)
			}
		}

		packager := processing.NewPackager(repo, dataDir)
		if err := packager.VerifyPackageIntegrity(packageDir, 4); err != nil {
			t.Fatalf("VerifyPackageIntegrity 失败: %v", err)
		}

		previewPath := filepath.Join(packageDir, "preview.png")
		if _, err := os.Stat(previewPath); err != nil {
			t.Fatalf("资源包 preview.png 不存在: %v", err)
		}
		metadataPath := filepath.Join(packageDir, "metadata.json")
		if _, err := os.Stat(metadataPath); err != nil {
			t.Fatalf("资源包 metadata.json 不存在: %v", err)
		}
	})

	t.Run("SubTask33.5_处理范围与隔离", func(t *testing.T) {
		actions, err := repo.ListProcessingActions(pt.ID)
		if err != nil {
			t.Fatalf("查询处理动作失败: %v", err)
		}
		if len(actions) != 4 {
			t.Fatalf("处理动作记录数 = %d, 期望 4", len(actions))
		}
		actionKeys := map[string]bool{}
		for _, a := range actions {
			actionKeys[a.ActionKey] = true
		}
		for _, key := range []string{"idle_normal", "wave", "happy", "speaking"} {
			if !actionKeys[key] {
				t.Fatalf("处理动作中缺少 %s", key)
			}
		}

		var callLogCount int64
		db.Table("desktop_pet_generation_call_logs").Where("task_id = ?", taskID).Count(&callLogCount)
		if callLogCount != 0 {
			t.Fatalf("处理过程中不应调用生图模型，但发现 %d 条调用记录", callLogCount)
		}

		runtimeDir := filepath.Join(dataDir, "desktop-pets", "runtime")
		if _, err := os.Stat(runtimeDir); !os.IsNotExist(err) {
			t.Fatalf("桌宠运行时目录不应被创建: %s", runtimeDir)
		}

		desktopPetsDir := filepath.Join(dataDir, "desktop-pets")
		entries, err := os.ReadDir(desktopPetsDir)
		if err != nil {
			t.Fatalf("读取 desktop-pets 目录失败: %v", err)
		}
		for _, e := range entries {
			if e.Name() == "runtime" {
				t.Fatalf("不应创建 runtime 目录")
			}
		}

		sourceRawDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "generated", "idle_normal", "attempt-1", "raw")
		if _, err := os.Stat(filepath.Join(sourceRawDir, "frame-0001.png")); err != nil {
			t.Fatalf("原始素材不应被删除: %v", err)
		}
	})

	t.Run("SubTask34.3_ProcessedFrame_Persistence", func(t *testing.T) {
		actions, err := repo.ListProcessingActions(pt.ID)
		if err != nil {
			t.Fatalf("查询处理动作失败: %v", err)
		}
		if len(actions) == 0 {
			t.Fatal("没有处理动作记录")
		}

		svc := processing.NewService(repo, db, &app.AppContext{DB: db, Context: context.Background()}, dataDir)

		for _, a := range actionDefs {
			pa, err := repo.GetProcessingActionByActionKey(pt.ID, a.key)
			if err != nil {
				t.Fatalf("获取动作 %s 失败: %v", a.key, err)
			}

			frames, err := repo.ListProcessedFramesByAction(pa.ID)
			if err != nil {
				t.Fatalf("查询动作 %s 的处理后帧失败: %v", a.key, err)
			}

			framesDir := filepath.Join(dataDir, "desktop-pets", "generation-tasks", taskID, "processed", "version-1", "actions", a.key, "frames")
			entries, err := os.ReadDir(framesDir)
			if err != nil {
				t.Fatalf("读取动作 %s 帧目录失败: %v", a.key, err)
			}
			actualFrameCount := 0
			for _, e := range entries {
				if !e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
					actualFrameCount++
				}
			}

			if len(frames) == 0 {
				t.Fatalf("动作 %s 的 ProcessedFrame 记录为空", a.key)
			}
			if len(frames) != actualFrameCount {
				t.Fatalf("动作 %s 的 ProcessedFrame 记录数 = %d, 期望 = %d (实际帧文件数)", a.key, len(frames), actualFrameCount)
			}

			for i, f := range frames {
				if f.ProcessedPath == "" {
					t.Fatalf("动作 %s 帧 %d 的 ProcessedPath 为空", a.key, i)
				}
				if f.Status != "succeeded" {
					t.Fatalf("动作 %s 帧 %d 的 Status = %s, 期望 succeeded", a.key, i, f.Status)
				}
				if f.SourceFrameID == "" {
					t.Fatalf("动作 %s 帧 %d 的 SourceFrameID 为空", a.key, i)
				}
				if f.ContentHash == "" {
					t.Fatalf("动作 %s 帧 %d 的 ContentHash 为空", a.key, i)
				}

				processedFullPath, _, err := svc.GetProcessedFrameImage(pt.ID, a.key, i)
				if err != nil {
					t.Fatalf("GetProcessedFrameImage 动作 %s 帧 %d 失败: %v", a.key, i, err)
				}
				if _, err := os.Stat(processedFullPath); err != nil {
					t.Fatalf("GetProcessedFrameImage 返回的文件不存在: %s, err: %v", processedFullPath, err)
				}

				sourceFullPath, _, err := svc.GetSourceFrameImage(pt.ID, a.key, i)
				if err != nil {
					t.Fatalf("GetSourceFrameImage 动作 %s 帧 %d 失败: %v", a.key, i, err)
				}
				if _, err := os.Stat(sourceFullPath); err != nil {
					t.Fatalf("GetSourceFrameImage 返回的文件不存在: %s, err: %v", sourceFullPath, err)
				}
			}
		}
	})
}
