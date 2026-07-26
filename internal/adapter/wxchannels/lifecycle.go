package wxchannels

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/rs/zerolog"
	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	"wx_channel/internal/events"
	"wx_channel/internal/pipeline"
)

// RegisterLifecycleHooks subscribes to download lifecycle events and triggers
// the post-processing pipeline for wxchannels tasks.
func RegisterLifecycleHooks(bus *events.Bus, db *gorm.DB, logger *zerolog.Logger) {
	if bus == nil || db == nil {
		return
	}

	bus.Subscribe(events.TypeDownloadTaskFinished, func(e events.Event) {
		ev, ok := e.(events.DownloadTaskFinished)
		if !ok {
			return
		}
		onDownloadFinished(db, logger, ev.TaskID)
	})
}

// onDownloadFinished loads task and content data, populates the pipeline context,
// and runs the platform's post-processing pipeline.
func onDownloadFinished(db *gorm.DB, logger *zerolog.Logger, taskID int) {
	// 1. Load task from DB
	var task model.DownloadTaskV1
	if err := db.Where("id = ?", taskID).First(&task).Error; err != nil {
		logger.Warn().Err(err).Int("task_id", taskID).Msg("postprocess: load task failed")
		return
	}

	// 2. Verify this is a wxchannels task
	var meta struct {
		Platform string `json:"platform"`
		EncLimit int64  `json:"enc_limit"`
	}
	if err := json.Unmarshal([]byte(task.MetadataJSON), &meta); err != nil || meta.Platform != PlatformID {
		return
	}

	// 3. Parse config_json into a generic map for pipeline consumption.
	var cfg map[string]any
	if task.ConfigJSON != "" {
		json.Unmarshal([]byte(task.ConfigJSON), &cfg)
	}
	if cfg == nil {
		cfg = make(map[string]any)
	}

	// 4. Check if handler implements PostProcessor
	h := registry.Get(PlatformID)
	if h == nil {
		return
	}
	pp, ok := h.(registry.PostProcessor)
	if !ok {
		return
	}

	// 4. Load Content by DownloadTaskId (for decode_key)
	var content model.Content
	taskIDPtr := &taskID
	if err := db.Where("download_task_id = ?", taskIDPtr).First(&content).Error; err != nil {
		logger.Warn().Err(err).Int("task_id", taskID).Msg("postprocess: load content failed")
	}

	// 5. Load first resource to determine the input file path
	var resource model.DownloadResource
	if err := db.Where("task_id = ?", taskID).Order("merge_order ASC, id ASC").First(&resource).Error; err != nil {
		logger.Warn().Err(err).Int("task_id", taskID).Msg("postprocess: load resource failed")
		return
	}

	// 封面图片无需后处理（不需要解密、转码等）
	if resource.Kind == "cover" || resource.Kind == "picture" {
		logger.Info().Int("task_id", taskID).Str("kind", resource.Kind).Msg("postprocess: 封面/图片资源无需后处理，跳过")
		return
	}

	inputFile := filepath.Join(task.SavePath, resource.Name)

	// 6. Populate pipeline context
	pc := pipeline.NewContext()
	pc.Values["input_file"] = inputFile
	pc.Values["save_path"] = task.SavePath
	pc.Values["task_id"] = taskID
	pc.Values["db"] = db
	pc.Values["config"] = cfg // 用户提交的下载配置 map

	// 直播流（STREAM）使用专用后处理管道：ffmpeg remux 为 MP4
	if resource.ResourceType == model.ResourceTypeStream {
		logger.Info().Int("task_id", taskID).Str("file", inputFile).Msg("postprocess: stream detected, running stream pipeline")
		sp := StreamPostProcessPipeline(pc)
		sp.OnEvent = buildPipelineLogger(logger, taskID)
		go func() {
			if result, err := sp.Run(context.Background(), pc); err != nil {
				logger.Error().Err(err).Int("task_id", taskID).Msg("postprocess: stream pipeline failed")
			} else if result != nil {
				logger.Info().Int("task_id", taskID).Dur("duration", result.Duration).Msg("postprocess: stream pipeline completed")
			}
		}()
		return
	}

	// 非 STREAM 资源走常规管道
	if content.ExternalId3 != "" {
		pc.Values["decode_key"] = content.ExternalId3
	}

	encLimit := uint64(meta.EncLimit)
	if encLimit == 0 {
		encLimit = 131072
	}
	pc.Values["enc_limit"] = encLimit

	// 7. Build and run pipeline
	p := pp.PostProcessPipeline(pc)
	p.OnEvent = func(evt pipeline.Event) {
		switch evt.Kind {
		case pipeline.EventNodeError:
			logger.Error().
				Str("pipeline", evt.Pipeline).
				Str("node", evt.NodeID).
				Err(evt.Error).
				Int("task_id", taskID).
				Msg("postprocess: pipeline node error")
		case pipeline.EventPipelineDone:
			logger.Info().
				Str("pipeline", evt.Pipeline).
				Int("task_id", taskID).
				Msg("postprocess: pipeline done")
		}
	}

	logger.Info().Int("task_id", taskID).Str("file", inputFile).Msg("postprocess: starting pipeline")

	ctx := context.Background()
	if result, err := p.Run(ctx, pc); err != nil {
		logger.Error().Err(err).Int("task_id", taskID).Msg("postprocess: pipeline failed")
	} else if result != nil {
		logger.Info().Int("task_id", taskID).Dur("duration", result.Duration).Msg("postprocess: pipeline completed")
	}
}

// buildPipelineLogger 创建管道事件日志回调，供 STREAM 等管道复用。
func buildPipelineLogger(logger *zerolog.Logger, taskID int) func(pipeline.Event) {
	return func(evt pipeline.Event) {
		switch evt.Kind {
		case pipeline.EventNodeError:
			logger.Error().
				Str("pipeline", evt.Pipeline).
				Str("node", evt.NodeID).
				Err(evt.Error).
				Int("task_id", taskID).
				Msg("postprocess: pipeline node error")
		case pipeline.EventPipelineDone:
			logger.Info().
				Str("pipeline", evt.Pipeline).
				Int("task_id", taskID).
				Msg("postprocess: pipeline done")
		}
	}
}
