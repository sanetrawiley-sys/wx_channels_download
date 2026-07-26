package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/gin-gonic/gin"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	"wx_channel/internal/events"
	"wx_channel/pkg/hermes"
	result "wx_channel/internal/util"
)

// CreateDownloadTaskV1Request 创建下载任务请求
type CreateDownloadTaskV1Request struct {
	Objects []CreateDownloadTaskV1Body `json:"objects"`
}

// CreateDownloadTaskV1Body 创建下载任务请求体
type CreateDownloadTaskV1Body struct {
	Platform string          `json:"platform"` // 内容平台
	Content  json.RawMessage `json:"content"`  // 平台内容原始 JSON
	Config   DownloadConfig  `json:"config"`   // 下载配置
}

// DownloadConfig 下载配置
type DownloadConfig struct {
	SavePath      string `json:"save_path"`
	Filename      string `json:"filename"`
	Spec          json.RawMessage `json:"spec"`
	Suffix        string `json:"suffix"`       // 文件后缀，如 ".mp3" 表示下载后转换为 mp3
	DownloadCover bool   `json:"download_cover"`
	Overwrite     bool   `json:"overwrite"`
	Duplicate     bool   `json:"duplicate"`
	ConvertMP3    bool   `json:"convert_mp3"`  // 下载后转换为 mp3
	UploadCloud   bool   `json:"upload_cloud"` // 下载后上传云存储
}

// taskV1IDBody 通用 task_id 请求体
type taskV1IDBody struct {
	TaskID int `json:"task_id"`
}

// CreateDownloadTaskByURLRequest 通过资源地址创建下载任务请求
type CreateDownloadTaskByURLRequest struct {
	Objects []CreateDownloadTaskByURLBody `json:"objects"`
}

// CreateDownloadTaskByURLBody 通过资源地址创建下载任务请求体
type CreateDownloadTaskByURLBody struct {
	URL      string         `json:"url"`       // 资源下载地址，必填
	SavePath string         `json:"save_path"` // 保存路径
	Filename string         `json:"filename"`  // 文件名（可选，默认从URL提取）
	Config   DownloadConfig `json:"config"`    // 下载配置
}

// resolveDownloadSaveDir 统一解析下载任务保存目录。
// 请求未指定目录时使用应用配置；相对路径相对于工作目录展开。
func (c *APIClient) resolveDownloadSaveDir(requested string) (string, error) {
	savePath := strings.TrimSpace(requested)
	if savePath == "" && c.cfg != nil {
		savePath = strings.TrimSpace(c.cfg.DownloadDir)
	}
	if savePath == "" {
		return "", fmt.Errorf("保存目录不能为空")
	}

	workDir := ""
	if c.cfg != nil {
		workDir = c.cfg.WorkDir
	}
	if workDir == "" {
		var err error
		workDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("获取工作目录失败: %w", err)
		}
	}

	savePath = strings.ReplaceAll(savePath, "%UserDownloads%", xdg.UserDirs.Download)
	savePath = strings.ReplaceAll(savePath, "%CWD%", workDir)
	savePath = filepath.Clean(savePath)
	if !filepath.IsAbs(savePath) {
		savePath = filepath.Join(workDir, savePath)
	}

	if err := os.MkdirAll(savePath, 0755); err != nil {
		return "", fmt.Errorf("创建保存目录 %q 失败: %w", savePath, err)
	}

	return savePath, nil
}

// downloadTaskSavePath 返回任务保存根目录。
func downloadTaskSavePath(saveDir string) string {
	return saveDir
}

// startCreatedDownloadTask 将新建任务交给 Hermes 调度。
// Hermes 通过 Store 接口管理所有内部状态（连接、状态变更、日志）并通过 EventHandler 回调触发广播。
func (c *APIClient) startCreatedDownloadTask(taskID int) error {
	if c.downloader == nil {
		c.logger.Error().Int("task_id", taskID).Msg("Hermes 下载器未初始化，无法启动下载任务")
		return fmt.Errorf("Hermes 下载器未初始化")
	}
	c.logger.Debug().Int("task_id", taskID).Msg("将下载任务提交到 Hermes 调度器")
	if err := c.downloader.Start(taskID); err != nil {
		c.logger.Error().Int("task_id", taskID).Err(err).Msg("Hermes 调度器启动下载任务失败")
		return err
	}
	c.logger.Info().Int("task_id", taskID).Msg("下载任务已提交到 Hermes 调度队列")
	return nil
}

// prepareDownloadTaskV1Single 预览单个平台下载任务（不写入数据库、不启动下载），返回将要创建的任务信息。
func (c *APIClient) prepareDownloadTaskV1Single(body CreateDownloadTaskV1Body) (gin.H, error) {
	if body.Platform == "" {
		return nil, fmt.Errorf("platform 不能为空")
	}

	h := registry.Get(body.Platform)
	if h == nil {
		return nil, fmt.Errorf("不支持的平台: %s", body.Platform)
	}

	saveDir, err := c.resolveDownloadSaveDir(body.Config.SavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}

	convertMP3 := body.Config.ConvertMP3 || strings.EqualFold(body.Config.Suffix, ".mp3")

	info, content, account, err := h.BuildDownloadTask(body.Content, registry.DownloadConfig{
		SavePath:      saveDir,
		Filename:      body.Config.Filename,
		Spec:          specFromJSON(body.Config.Spec),
		Suffix:        body.Config.Suffix,
		DownloadCover: body.Config.DownloadCover,
		Overwrite:     body.Config.Overwrite,
		Duplicate:     body.Config.Duplicate,
		ConvertMP3:    convertMP3,
		UploadCloud:   body.Config.UploadCloud,
	})
	if err != nil {
		return nil, fmt.Errorf("构建下载任务失败: %w", err)
	}
	if info == nil {
		return nil, fmt.Errorf("构建下载任务失败: 平台未返回下载任务")
	}

	resourceInfos := info.Resources
	if len(resourceInfos) == 0 {
		resourceInfos = []registry.DownloadResourceInfo{{
			Resource:  info.Resource,
			Endpoints: []model.DownloadEndpoint{info.Endpoint},
		}}
	}
	for _, resourceInfo := range resourceInfos {
		if len(resourceInfo.Endpoints) == 0 {
			return nil, fmt.Errorf("资源 %s 没有下载端点", resourceInfo.Resource.Name)
		}
	}

	info.Task.SavePath = downloadTaskSavePath(saveDir)

	// 构建预览数据（不写入数据库）
	resources := make([]gin.H, 0, len(resourceInfos))
	totalEndpoints := 0
	for i, ri := range resourceInfos {
		eps := make([]gin.H, 0, len(ri.Endpoints))
		for _, ep := range ri.Endpoints {
			eps = append(eps, gin.H{
				"protocol": ep.Protocol,
				"url":      ep.URL,
				"priority": ep.Priority,
			})
		}
		resources = append(resources, gin.H{
			"index":     i,
			"name":      ri.Resource.Name,
			"kind":      ri.Resource.Kind,
			"endpoints": eps,
		})
		totalEndpoints += len(ri.Endpoints)
	}
	tree := buildResourceTree(resources)

	return gin.H{
		"platform":       body.Platform,
		"task_name":      info.Task.Name,
		"save_path":      info.Task.SavePath,
		"resources":       resources,
		"tree":            tree,
		"resource_count":  len(resourceInfos),
		"endpoint_count":  totalEndpoints,
		"content":         content,
		"account":         account,
	}, nil
}

// handlePrepareDownloadTaskV1 批量预览平台下载任务
// POST /api/v1/download_task/prepare
func (c *APIClient) handlePrepareDownloadTaskV1(ctx *gin.Context) {
	var req CreateDownloadTaskV1Request
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(req.Objects) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	previews := make([]gin.H, 0, len(req.Objects))
	for _, body := range req.Objects {
		data, err := c.prepareDownloadTaskV1Single(body)
		if err != nil {
			previews = append(previews, gin.H{"success": false, "error": err.Error()})
		} else {
			previews = append(previews, gin.H{"success": true, "data": data})
		}
	}

	result.Ok(ctx, gin.H{"previews": previews})
}

// prepareDownloadTaskByURLV1Single 预览通过资源地址创建的下载任务（不写入数据库、不启动下载）。
func (c *APIClient) prepareDownloadTaskByURLV1Single(body CreateDownloadTaskByURLBody) (gin.H, error) {
	if body.URL == "" {
		return nil, fmt.Errorf("url 不能为空")
	}

	parsedURL, err := url.Parse(body.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("无效的下载地址")
	}

	protocol := strings.ToUpper(parsedURL.Scheme)

	requestedSavePath := body.SavePath
	if requestedSavePath == "" {
		requestedSavePath = body.Config.SavePath
	}
	saveDir, err := c.resolveDownloadSaveDir(requestedSavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}
	filename := body.Filename
	if filename == "" {
		filename = body.Config.Filename
	}
	if filename == "" {
		base := filepath.Base(parsedURL.Path)
		if base != "" && base != "." && base != "/" {
			if decoded, err := url.QueryUnescape(base); err == nil {
				filename = decoded
			} else {
				filename = base
			}
		}
	}
	if filename == "" {
		filename = body.URL
	}
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || filename == ".." || filename == string(filepath.Separator) {
		return nil, fmt.Errorf("无法确定下载文件名")
	}

	savePath := downloadTaskSavePath(saveDir)

	return gin.H{
		"url":       body.URL,
		"protocol":  protocol,
		"task_name": filename,
		"save_path": savePath,
		"resources": []gin.H{{
			"index": 0,
			"name":  filename,
			"kind":  "file",
			"endpoints": []gin.H{{
				"protocol": protocol,
				"url":      body.URL,
				"priority": 0,
			}},
		}},
		"resource_count": 1,
		"endpoint_count": 1,
	}, nil
}

// handlePrepareDownloadTaskByURLV1 批量预览通过资源地址创建的下载任务
// POST /api/v1/download_task/prepare_by_url
func (c *APIClient) handlePrepareDownloadTaskByURLV1(ctx *gin.Context) {
	var req CreateDownloadTaskByURLRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(req.Objects) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	previews := make([]gin.H, 0, len(req.Objects))
	for _, body := range req.Objects {
		data, err := c.prepareDownloadTaskByURLV1Single(body)
		if err != nil {
			previews = append(previews, gin.H{"success": false, "error": err.Error()})
		} else {
			previews = append(previews, gin.H{"success": true, "data": data})
		}
	}

	result.Ok(ctx, gin.H{"previews": previews})
}

// DuplicateTaskError 表示创建任务时发现已存在相同内容的下载任务。
type DuplicateTaskError struct {
	ExistingTaskID int
}

func (e *DuplicateTaskError) Error() string {
	return "已存在该下载内容"
}

func (e *DuplicateTaskError) StatusCode() int {
	return 409
}

// duplicateConflict 记录一个冲突信息。
type duplicateConflict struct {
	Type        string // "resource" | "file"
	TaskID      int    // 冲突的任务 ID（仅 resource 类型）
	FilePath    string // 冲突的文件路径（仅 file 类型）
	ResourceKey string // 用于展示的冲突资源标识
}

// checkDuplicateV1 检查两类重复：
// 1. 资源级重复 — 通过 resource.unique_id 查找未完成任务中是否已有相同资源
// 2. 文件级重复 — 输出目录下已存在同名文件
//
// resourceKeys: 每个 resource 的 unique_id（长度与 resourceNames 一致）
// duplicate:   允许重复下载，跳过冲突检查直接创建新任务
// overwrite:   发现冲突时删除旧任务/文件，继续创建新任务
// 默认:        返回 409 冲突
func (c *APIClient) checkDuplicateV1(saveDir string, resourceKeys []string, resourceNames []string, duplicate bool, overwrite bool) (bool, gin.H, error) {
	// 重复下载模式：允许重复创建，跳过冲突检查
	if duplicate {
		return false, nil, nil
	}

	var conflicts []duplicateConflict
	var existingTaskID int

	// 1. 资源级重复检查：通过 unique_id 查找未完成任务中引用了相同资源的记录
	for i, key := range resourceKeys {
		if key == "" {
			continue
		}
		var dup model.DownloadResource
		err := c.db.
			Joins("JOIN download_task_v1 ON download_task_v1.id = download_resource.task_id").
			Where("download_resource.unique_id = ?", key).
			Where("download_task_v1.status NOT IN (?, ?, ?)", model.TaskStatusFinished, model.TaskStatusFailed, model.TaskStatusCancelled).
			Where("download_task_v1.deleted_at IS NULL").
			First(&dup).Error
		if err == nil {
			existingTaskID = dup.TaskId
			conflicts = append(conflicts, duplicateConflict{
				Type:        "resource",
				TaskID:      dup.TaskId,
				ResourceKey: resourceNames[i],
			})
		}
	}

	// 2. 文件级重复检查：检查输出文件是否已存在
	for _, name := range resourceNames {
		filePath := filepath.Join(saveDir, filepath.Base(name))
		if fileInfo, err := os.Stat(filePath); err == nil && !fileInfo.IsDir() {
			conflicts = append(conflicts, duplicateConflict{
				Type:     "file",
				FilePath: filePath,
			})
		}
	}

	// 无冲突，继续创建
	if len(conflicts) == 0 {
		return false, nil, nil
	}

	// 覆盖模式：清理冲突后继续创建
	if overwrite {
		for _, conflict := range conflicts {
			switch conflict.Type {
			case "resource":
				if err := c.deleteTaskWithFiles(conflict.TaskID); err != nil {
					return false, nil, fmt.Errorf("覆盖已存在任务失败: %w", err)
				}
			case "file":
				if err := os.Remove(conflict.FilePath); err != nil && !os.IsNotExist(err) {
					return false, nil, fmt.Errorf("覆盖已存在文件失败: %w", err)
				}
			}
		}
		return false, nil, nil // 继续创建新任务
	}

	// 默认：返回 409
	errResp := &DuplicateTaskError{}
	if existingTaskID > 0 {
		errResp.ExistingTaskID = existingTaskID
	}
	return true, nil, errResp
}

// deleteTaskWithFiles 删除任务及其下载的物理文件。
func (c *APIClient) deleteTaskWithFiles(taskID int) error {
	var task model.DownloadTaskV1
	if err := c.db.First(&task, taskID).Error; err != nil {
		return fmt.Errorf("任务不存在: %w", err)
	}

	// 删除物理文件
	if task.SavePath != "" {
		os.RemoveAll(task.SavePath)
	}

	// 软删除任务（也会级联删除关联的 resources、endpoints）
	return c.db.Model(&task).Updates(map[string]any{
		"deleted_at": time.Now().UnixMilli(),
	}).Error
}

// createDownloadTaskV1Single 创建单个平台下载任务，返回结果数据或错误。
func (c *APIClient) createDownloadTaskV1Single(body CreateDownloadTaskV1Body) (gin.H, error) {
	c.logger.Debug().Str("platform", body.Platform).Msg("开始处理单个下载任务创建请求")

	// 数据库未初始化
	if c.db == nil {
		c.logger.Error().Msg("数据库未初始化，无法创建下载任务")
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	if body.Platform == "" {
		return nil, fmt.Errorf("platform 不能为空")
	}

	// 根据平台获取对应的处理器
	h := registry.Get(body.Platform)
	if h == nil {
		c.logger.Warn().Str("platform", body.Platform).Msg("不支持的平台")
		return nil, fmt.Errorf("不支持的平台: %s", body.Platform)
	}

	saveDir, err := c.resolveDownloadSaveDir(body.Config.SavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}

	// suffix ".mp3" 等同于 convert_mp3: true
	convertMP3 := body.Config.ConvertMP3 || strings.EqualFold(body.Config.Suffix, ".mp3")

	// 调用平台处理器构建下载模型
	info, content, account, err := h.BuildDownloadTask(body.Content, registry.DownloadConfig{
		SavePath:      saveDir,
		Filename:      body.Config.Filename,
		Spec:          specFromJSON(body.Config.Spec),
		Suffix:        body.Config.Suffix,
		DownloadCover: body.Config.DownloadCover,
		Overwrite:     body.Config.Overwrite,
		Duplicate:     body.Config.Duplicate,
		ConvertMP3:    convertMP3,
		UploadCloud:   body.Config.UploadCloud,
	})
	if err != nil {
		c.logger.Error().Str("platform", body.Platform).Err(err).Msg("平台构建下载任务失败")
		return nil, fmt.Errorf("构建下载任务失败: %w", err)
	}
	if info == nil {
		c.logger.Warn().Str("platform", body.Platform).Msg("平台未返回下载任务信息")
		return nil, fmt.Errorf("构建下载任务失败: 平台未返回下载任务")
	}

	resourceInfos := info.Resources
	c.logger.Debug().Str("platform", body.Platform).Str("task_name", info.Task.Name).Int("resource_count", len(resourceInfos)).Msg("平台下载任务构建成功")
	if len(resourceInfos) == 0 {
		resourceInfos = []registry.DownloadResourceInfo{{
			Resource:  info.Resource,
			Endpoints: []model.DownloadEndpoint{info.Endpoint},
		}}
	}
	for _, resourceInfo := range resourceInfos {
		if len(resourceInfo.Endpoints) == 0 {
			return nil, fmt.Errorf("资源 %s 没有下载端点", resourceInfo.Resource.Name)
		}
	}

	// 保存路径由 API 统一生成，Task 的 SavePath 始终为目录。
	info.Task.SavePath = downloadTaskSavePath(saveDir)

	// 检查重复：资源级（unique_id）和文件级（同名文件）
	resourceKeys := make([]string, 0, len(resourceInfos))
	resourceNames := make([]string, 0, len(resourceInfos))
	for _, ri := range resourceInfos {
		resourceKeys = append(resourceKeys, ri.Resource.UniqueID)
		resourceNames = append(resourceNames, ri.Resource.Name)
	}
	if handled, resp, err := c.checkDuplicateV1(info.Task.SavePath, resourceKeys, resourceNames, body.Config.Duplicate, body.Config.Overwrite); err != nil {
		return nil, err
	} else if handled {
		return resp, nil
	}

	// onTaskCreate hook: 用户可以在写入 DB 前修改资源名、过滤资源等
	if c.hookManager != nil && c.hookManager.HasCreateHook() {
		taskInput := buildTaskInput(info, body.Config)
		modified, err := c.hookManager.InvokeCreateHook(taskInput)
		if err != nil {
			return nil, fmt.Errorf("onTaskCreate hook 执行失败: %w", err)
		}
		applyTaskInputModifications(info, modified)
	}

	// 写入数据库
	now := time.Now().UnixMilli()
	if info.Task.CreatedAt == 0 {
		info.Task.CreatedAt = now
	}
	info.Task.UpdatedAt = now
	if err := c.db.Create(&info.Task).Error; err != nil {
		c.logger.Error().Str("platform", body.Platform).Err(err).Msg("下载任务写入数据库失败")
		return nil, fmt.Errorf("创建下载任务失败: %w", err)
	}
	c.logger.Info().Int("task_id", info.Task.Id).Str("task_name", info.Task.Name).Str("platform", body.Platform).Str("save_path", info.Task.SavePath).Msg("下载任务已写入数据库")

	// 将 Content 关联到下载任务，供后处理管道使用
	if content != nil {
		taskID := info.Task.Id
		content.DownloadTaskId = &taskID
		content.UpdatedAt = now
		if err := c.db.Save(content).Error; err != nil {
			return nil, fmt.Errorf("保存 Content 关联失败: %w", err)
		}
	}

	resources := make([]model.DownloadResource, 0, len(resourceInfos))
	endpoints := make([]model.DownloadEndpoint, 0, len(resourceInfos))
	for i := range resourceInfos {
		resource := resourceInfos[i].Resource
		resource.TaskId = info.Task.Id
		if resource.CreatedAt == 0 {
			resource.CreatedAt = now
		}
		resource.UpdatedAt = now
		if err := c.db.Create(&resource).Error; err != nil {
			return nil, fmt.Errorf("创建资源失败: %w", err)
		}
		resources = append(resources, resource)
		for _, endpointInfo := range resourceInfos[i].Endpoints {
			endpoint := endpointInfo
			endpoint.ResourceId = resource.Id
			if endpoint.CreatedAt == 0 {
				endpoint.CreatedAt = now
			}
			endpoint.UpdatedAt = now
			if err := c.db.Create(&endpoint).Error; err != nil {
				return nil, fmt.Errorf("创建端点失败: %w", err)
			}
			endpoints = append(endpoints, endpoint)
		}
	}
	if len(resources) == 0 || len(endpoints) == 0 {
		return nil, fmt.Errorf("平台未返回可下载资源或端点")
	}
	info.Resource = resources[0]
	info.Endpoint = endpoints[0]

	if err := c.startCreatedDownloadTask(info.Task.Id); err != nil {
		return nil, fmt.Errorf("启动下载任务失败: %w", err)
	}
	info.Task.Status = model.TaskStatusPreparing // Hermes 已写入 DB，此处仅更新内存变量供响应

	return gin.H{
		"task":      info.Task,
		"resource":  info.Resource,
		"endpoint":  info.Endpoint,
		"resources": resources,
		"endpoints": endpoints,
		"content":   content,
		"account":   account,
	}, nil
}

// handleCreateDownloadTaskV1 批量创建平台下载任务
// POST /api/v1/download_task/create
func (c *APIClient) handleCreateDownloadTaskV1(ctx *gin.Context) {
	var req CreateDownloadTaskV1Request
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.Warn().Err(err).Msg("POST /api/v1/download_task/create 请求参数解析失败")
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(req.Objects) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	c.logger.Info().Int("object_count", len(req.Objects)).Msg("POST /api/v1/download_task/create 收到批量创建下载任务请求")

	var duplicateErr *DuplicateTaskError
	tasks := make([]gin.H, 0, len(req.Objects))
	successCount := 0
	failCount := 0
	for _, body := range req.Objects {
		data, err := c.createDownloadTaskV1Single(body)
		if err != nil {
			if errors.As(err, &duplicateErr) {
				// 单个任务冲突：如果是单个请求则返回 409；批量请求中标记失败
				if len(req.Objects) == 1 {
					c.logger.Warn().Int("existing_task_id", duplicateErr.ExistingTaskID).Msg("POST /api/v1/download_task/create 任务冲突，已存在")
					result.Err(ctx, duplicateErr.StatusCode(), duplicateErr.Error())
					return
				}
				tasks = append(tasks, gin.H{"success": false, "error": err.Error(), "duplicate": true, "existing_task_id": duplicateErr.ExistingTaskID})
				failCount++
				continue
			}
			c.logger.Warn().Str("platform", body.Platform).Err(err).Msg("创建下载任务失败")
			tasks = append(tasks, gin.H{"success": false, "error": err.Error()})
			failCount++
		} else {
			tasks = append(tasks, gin.H{"success": true, "data": data})
			successCount++
		}
	}

	c.logger.Info().
		Int("total", len(tasks)).
		Int("success", successCount).
		Int("failed", failCount).
		Msg("POST /api/v1/download_task/create 批量创建下载任务完成")

	result.Ok(ctx, gin.H{"tasks": tasks})
}

// createDownloadTaskByURLV1Single 通过资源地址创建单个下载任务。
func (c *APIClient) createDownloadTaskByURLV1Single(body CreateDownloadTaskByURLBody) (gin.H, error) {
	if body.URL == "" {
		return nil, fmt.Errorf("url 不能为空")
	}

	parsedURL, err := url.Parse(body.URL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, fmt.Errorf("无效的下载地址")
	}

	protocol := strings.ToUpper(parsedURL.Scheme)

	requestedSavePath := body.SavePath
	if requestedSavePath == "" {
		requestedSavePath = body.Config.SavePath
	}
	saveDir, err := c.resolveDownloadSaveDir(requestedSavePath)
	if err != nil {
		return nil, fmt.Errorf("准备保存目录失败: %w", err)
	}
	filename := body.Filename
	if filename == "" {
		filename = body.Config.Filename
	}
	if filename == "" {
		// 从 URL 路径提取文件名
		base := filepath.Base(parsedURL.Path)
		if base != "" && base != "." && base != "/" {
			if decoded, err := url.QueryUnescape(base); err == nil {
				filename = decoded
			} else {
				filename = base
			}
		}
	}
	// 如果仍然无法提取文件名，使用 URL 作为名称
	if filename == "" {
		filename = body.URL
	}
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || filename == ".." || filename == string(filepath.Separator) {
		return nil, fmt.Errorf("无法确定下载文件名")
	}

	savePath := downloadTaskSavePath(saveDir)

	taskName := filename

	// 存储原始下载地址到 config_json
	configJSON, _ := json.Marshal(map[string]string{
		"url": body.URL,
	})

	// 数据库未初始化
	if c.db == nil {
		return nil, fmt.Errorf("应用未初始化，数据库不可用")
	}

	now := time.Now().UnixMilli()

	// 创建任务
	task := model.DownloadTaskV1{
		Name:       taskName,
		Status:     model.TaskStatusWaiting,
		SavePath:   savePath,
		ConfigJSON: string(configJSON),
	}
	task.CreatedAt = now
	task.UpdatedAt = now

	if err := c.db.Create(&task).Error; err != nil {
		c.logger.Error().Str("url", body.URL).Err(err).Msg("URL 下载任务写入数据库失败")
		return nil, fmt.Errorf("创建下载任务失败: %w", err)
	}

	c.logger.Info().Int("task_id", task.Id).Str("url", body.URL).Str("save_path", savePath).Msg("URL 下载任务已写入数据库")
	// 创建资源
	resource := model.DownloadResource{
		TaskId:     task.Id,
		Name:       filename,
		Kind:       "file",
		Status:     0,
		MergeOrder: 0,
	}
	resource.CreatedAt = now
	resource.UpdatedAt = now

	if err := c.db.Create(&resource).Error; err != nil {
		return nil, fmt.Errorf("创建资源失败: %w", err)
	}

	// 创建端点
	endpoint := model.DownloadEndpoint{
		ResourceId: resource.Id,
		Protocol:   protocol,
		URL:        body.URL,
		Priority:   0,
		Enabled:    1,
		Status:     0,
	}
	endpoint.CreatedAt = now
	endpoint.UpdatedAt = now

	if err := c.db.Create(&endpoint).Error; err != nil {
		return nil, fmt.Errorf("创建端点失败: %w", err)
	}

	// 交给调度器；任务先进入 PREPARING，获得并发槽位后再转为 DOWNLOADING。
	if err := c.startCreatedDownloadTask(task.Id); err != nil {
		return nil, fmt.Errorf("启动下载任务失败: %w", err)
	}
	task.Status = model.TaskStatusPreparing // Hermes 已写入 DB，此处仅更新内存变量供响应

	return gin.H{
		"task":     task,
		"resource": resource,
		"endpoint": endpoint,
	}, nil
}

// handleCreateDownloadTaskByURLV1 批量通过资源地址创建下载任务
// POST /api/v1/download_task/create_by_url
func (c *APIClient) handleCreateDownloadTaskByURLV1(ctx *gin.Context) {
	var req CreateDownloadTaskByURLRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		c.logger.Warn().Err(err).Msg("POST /api/v1/download_task/create_by_url 请求参数解析失败")
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if len(req.Objects) == 0 {
		result.Err(ctx, 400, "请求体不能为空数组")
		return
	}

	c.logger.Info().Int("object_count", len(req.Objects)).Msg("POST /api/v1/download_task/create_by_url 收到批量创建 URL 下载任务请求")

	tasks := make([]gin.H, 0, len(req.Objects))
	successCount := 0
	failCount := 0
	for _, body := range req.Objects {
		data, err := c.createDownloadTaskByURLV1Single(body)
		if err != nil {
			c.logger.Warn().Str("url", body.URL).Err(err).Msg("创建 URL 下载任务失败")
			tasks = append(tasks, gin.H{"success": false, "error": err.Error()})
			failCount++
		} else {
			tasks = append(tasks, gin.H{"success": true, "data": data})
			successCount++
		}
	}

	c.logger.Info().
		Int("total", len(tasks)).
		Int("success", successCount).
		Int("failed", failCount).
		Msg("POST /api/v1/download_task/create_by_url 批量创建 URL 下载任务完成")

	result.Ok(ctx, gin.H{"tasks": tasks})
}

// handleStartDownloadTaskV1 启动下载任务
// POST /api/v1/download_task/start
func (c *APIClient) handleStartDownloadTaskV1(ctx *gin.Context) {
	var body taskV1IDBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		c.logger.Warn().Err(err).Msg("POST /api/v1/download_task/start 请求参数解析失败")
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if body.TaskID <= 0 {
		result.Err(ctx, 400, "task_id 无效")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var task model.DownloadTaskV1
	if err := c.db.Where("id = ?", body.TaskID).First(&task).Error; err != nil {
		c.logger.Warn().Int("task_id", body.TaskID).Msg("POST /api/v1/download_task/start 任务不存在")
		result.Err(ctx, 404, "下载任务不存在")
		return
	}

	// 只有 Waiting / Paused / Failed 状态可以启动
	if task.Status != model.TaskStatusWaiting &&
		task.Status != model.TaskStatusPaused &&
		task.Status != model.TaskStatusFailed {
		c.logger.Warn().Int("task_id", body.TaskID).Int("current_status", task.Status).Msg("POST /api/v1/download_task/start 当前状态不允许启动")
		result.Err(ctx, 400, "当前状态不允许启动")
		return
	}

	c.logger.Info().Int("task_id", body.TaskID).Str("task_name", task.Name).Int("previous_status", task.Status).Msg("POST /api/v1/download_task/start 收到启动下载任务请求")

	// Hermes 负责状态持久化、日志写入和事件广播。
	if err := c.downloader.Start(task.Id); err != nil {
		c.logger.Error().Int("task_id", body.TaskID).Err(err).Msg("启动下载任务失败")
		result.Err(ctx, 500, "启动下载任务失败: "+err.Error())
		return
	}
	c.logger.Info().Int("task_id", body.TaskID).Str("status", "preparing").Msg("下载任务已启动")

	task.Status = model.TaskStatusPreparing

	result.Ok(ctx, gin.H{"task": task, "status_text": "preparing"})
}

// handlePauseDownloadTaskV1 暂停下载任务
// POST /api/v1/download_task/pause
func (c *APIClient) handlePauseDownloadTaskV1(ctx *gin.Context) {
	var body taskV1IDBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if body.TaskID <= 0 {
		result.Err(ctx, 400, "task_id 无效")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var task model.DownloadTaskV1
	if err := c.db.Where("id = ?", body.TaskID).First(&task).Error; err != nil {
		result.Err(ctx, 404, "下载任务不存在")
		return
	}

	if task.Status != model.TaskStatusPreparing && task.Status != model.TaskStatusDownloading {
		result.Err(ctx, 400, "当前状态不允许暂停")
		return
	}

	// Hermes 负责所有状态持久化（task/resource/segment/connection）、日志写入和事件广播。
	c.downloader.Pause(task.Id)

	// 直播流（STREAM）暂停应标记为完成，因为流无法断点续传
	if c.hasStreamResources(task.Id) {
		now := time.Now().UnixMilli()
		c.db.Model(&task).Updates(map[string]any{"status": model.TaskStatusFinished, "updated_at": now})
		task.Status = model.TaskStatusFinished
		if c.bus != nil {
			go c.bus.Publish(events.DownloadTaskFinished{TaskID: task.Id})
		}
		result.Ok(ctx, gin.H{"task": task, "status_text": "finished"})
		return
	}

	task.Status = model.TaskStatusPaused
	result.Ok(ctx, gin.H{"task": task, "status_text": "paused"})
}

// handleResumeDownloadTaskV1 恢复下载任务
// POST /api/v1/download_task/resume
func (c *APIClient) handleResumeDownloadTaskV1(ctx *gin.Context) {
	var body taskV1IDBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if body.TaskID <= 0 {
		result.Err(ctx, 400, "task_id 无效")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var task model.DownloadTaskV1
	if err := c.db.Where("id = ?", body.TaskID).First(&task).Error; err != nil {
		result.Err(ctx, 404, "下载任务不存在")
		return
	}

	if task.Status != model.TaskStatusPaused {
		result.Err(ctx, 400, "当前状态不允许恢复")
		return
	}

	// Hermes 负责状态持久化、日志写入和事件广播。
	if err := c.downloader.Start(task.Id); err != nil {
		result.Err(ctx, 500, "恢复下载任务失败: "+err.Error())
		return
	}
	task.Status = model.TaskStatusPreparing

	result.Ok(ctx, gin.H{"task": task, "status_text": "preparing"})
}

// handleDeleteDownloadTaskV1 删除下载任务
// POST /api/v1/download_task/delete
func (c *APIClient) handleDeleteDownloadTaskV1(ctx *gin.Context) {
	var body taskV1IDBody
	if err := ctx.ShouldBindJSON(&body); err != nil {
		result.Err(ctx, 400, "不合法的请求参数: "+err.Error())
		return
	}
	if body.TaskID <= 0 {
		result.Err(ctx, 400, "task_id 无效")
		return
	}
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var task model.DownloadTaskV1
	if err := c.db.Where("id = ?", body.TaskID).First(&task).Error; err != nil {
		result.Err(ctx, 404, "下载任务不存在")
		return
	}

	now := time.Now().UnixMilli()

	// Hermes 停止下载作业、设置 Cancelled 状态、写入日志。
	c.downloader.Delete(task.Id)
	deletedRecord, _ := c.buildDownloadTaskRecord(task.Id)

	// 软删除 task（Hermes 已写入 Cancelled 状态）
	c.db.Model(&task).Update("deleted_at", now)

	// 级联软删除关联数据
	c.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Update("deleted_at", now)

	var resourceIDs []int
	c.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Pluck("id", &resourceIDs)
	if len(resourceIDs) > 0 {
		c.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)
		c.db.Model(&model.DownloadSegment{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)

		var endpointIDs []int
		c.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Pluck("id", &endpointIDs)
		if len(endpointIDs) > 0 {
			c.db.Model(&model.DownloadConnection{}).Where("endpoint_id IN ?", endpointIDs).Update("deleted_at", now)
		}
	}

	if deletedRecord != nil {
		c.broadcastDownloadTaskDelete([]DownloadTaskRecord{*deletedRecord})
	}

	result.Ok(ctx, gin.H{"task_id": task.Id, "status_text": "cancelled"})
}

// handleListDownloadTaskV1 查询下载任务列表
// GET /api/v1/download_task/list
func (c *APIClient) handleListDownloadTaskV1(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}
	if taskID, err := strconv.Atoi(ctx.Query("task_id")); err == nil && taskID > 0 {
		record, err := c.buildDownloadTaskRecord(taskID)
		if err != nil {
			result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
			return
		}
		if record == nil {
			result.Err(ctx, 404, "下载任务不存在")
			return
		}
		result.Ok(ctx, record)
		return
	}

	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(ctx.DefaultQuery("page_size", "20"))
	statusFilter := ctx.Query("status")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	var tasks []model.DownloadTaskV1
	var total int64

	query := c.db.Model(&model.DownloadTaskV1{}).Where("deleted_at IS NULL")
	if statusFilter != "" {
		parts := strings.Split(statusFilter, ",")
		ints := make([]int, 0, len(parts))
		for _, p := range parts {
			if v, err := strconv.Atoi(strings.TrimSpace(p)); err == nil {
				ints = append(ints, v)
			}
		}
		if len(ints) == 1 {
			query = query.Where("status = ?", ints[0])
		} else if len(ints) > 1 {
			query = query.Where("status IN ?", ints)
		}
	}
	if err := query.Count(&total).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务总数失败: "+err.Error())
		return
	}
	if err := query.Order("id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tasks).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
		return
	}

	list, err := c.buildDownloadTaskRecords(tasks)
	if err != nil {
		result.Err(ctx, 500, "构建下载任务记录失败: "+err.Error())
		return
	}

	result.Ok(ctx, gin.H{
		"list":      list,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ResourceTreeNode 资源树节点，用于前端渲染文件目录结构。
type ResourceTreeNode struct {
	Name     string              `json:"name"`
	Type     string              `json:"type"` // "file" | "directory"
	Kind     string              `json:"kind,omitempty"`
	Endpoints []gin.H            `json:"endpoints,omitempty"`
	Children []*ResourceTreeNode `json:"children,omitempty"`
}

// buildResourceTree 将扁平的资源列表按路径拆分为目录树。
// 资源 name 如 "chapters/0001.html" 会被归入 "chapters" 目录。
func buildResourceTree(resources []gin.H) *ResourceTreeNode {
	root := &ResourceTreeNode{Name: "", Type: "directory", Children: []*ResourceTreeNode{}}
	for _, r := range resources {
		name, _ := r["name"].(string)
		kind, _ := r["kind"].(string)
		eps, _ := r["endpoints"].([]gin.H)
		parts := strings.Split(name, "/")

		node := root
		for i, part := range parts {
			if i == len(parts)-1 {
				// 文件节点
				node.Children = append(node.Children, &ResourceTreeNode{
					Name:      part,
					Type:      "file",
					Kind:      kind,
					Endpoints: eps,
				})
			} else {
				// 目录节点，查找或创建
				var dir *ResourceTreeNode
				for _, child := range node.Children {
					if child.Type == "directory" && child.Name == part {
						dir = child
						break
					}
				}
				if dir == nil {
					dir = &ResourceTreeNode{
						Name:     part,
						Type:     "directory",
						Children: []*ResourceTreeNode{},
					}
					node.Children = append(node.Children, dir)
				}
				node = dir
			}
		}
	}
	return root
}

// handleStartAllDownloadTaskV1 批量启动下载任务
// POST /api/v1/download_task/start_all
func (c *APIClient) handleStartAllDownloadTaskV1(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	ctx.ShouldBindJSON(&body)

	query := c.db.Where("deleted_at IS NULL")
	switch body.Status {
	case "waiting":
		query = query.Where("status = ?", model.TaskStatusWaiting)
	case "paused":
		query = query.Where("status = ?", model.TaskStatusPaused)
	case "failed":
		query = query.Where("status = ?", model.TaskStatusFailed)
	default:
		// 启动所有可启动的
		query = query.Where("status IN (?, ?, ?)",
			model.TaskStatusWaiting, model.TaskStatusPaused, model.TaskStatusFailed)
	}

	var tasks []model.DownloadTaskV1
	if err := query.Find(&tasks).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
		return
	}

	var started int
	for _, task := range tasks {
		if err := c.downloader.Start(task.Id); err != nil {
			continue
		}
		started++
	}

	result.Ok(ctx, gin.H{"started": started, "total": len(tasks)})
}

// handlePauseAllDownloadTaskV1 批量暂停下载任务
// POST /api/v1/download_task/pause_all
func (c *APIClient) handlePauseAllDownloadTaskV1(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var body struct {
		Status string `json:"status"`
	}
	ctx.ShouldBindJSON(&body)

	query := c.db.Where("deleted_at IS NULL")
	switch body.Status {
	case "preparing":
		query = query.Where("status = ?", model.TaskStatusPreparing)
	case "downloading":
		query = query.Where("status = ?", model.TaskStatusDownloading)
	case "running":
		query = query.Where("status IN (?, ?)",
			model.TaskStatusPreparing, model.TaskStatusDownloading)
	default:
		query = query.Where("status IN (?, ?)",
			model.TaskStatusPreparing, model.TaskStatusDownloading)
	}

	var tasks []model.DownloadTaskV1
	if err := query.Find(&tasks).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
		return
	}

	var paused int
	for _, task := range tasks {
		c.downloader.Pause(task.Id)
		// 直播流暂停应标记为完成
		if c.hasStreamResources(task.Id) {
			now := time.Now().UnixMilli()
			c.db.Model(&model.DownloadTaskV1{}).Where("id = ?", task.Id).
				Updates(map[string]any{"status": model.TaskStatusFinished, "updated_at": now})
			if c.bus != nil {
				go c.bus.Publish(events.DownloadTaskFinished{TaskID: task.Id})
			}
		}
		paused++
	}

	result.Ok(ctx, gin.H{"paused": paused, "total": len(tasks)})
}

// handleClearDownloadTaskV1 清理已完成/失败/已取消的下载任务
// POST /api/v1/download_task/clear
func (c *APIClient) handleClearDownloadTaskV1(ctx *gin.Context) {
	if c.db == nil {
		result.Err(ctx, 500, "应用未初始化，数据库不可用")
		return
	}

	var body struct {
		DeleteFiles bool `json:"delete_files"`
	}
	ctx.ShouldBindJSON(&body)

	var tasks []model.DownloadTaskV1
	if err := c.db.Where("deleted_at IS NULL").
		Where("status IN (?, ?, ?)",
			model.TaskStatusFinished, model.TaskStatusFailed, model.TaskStatusCancelled).
		Find(&tasks).Error; err != nil {
		result.Err(ctx, 500, "查询下载任务失败: "+err.Error())
		return
	}

	now := time.Now().UnixMilli()
	var cleared int

	for _, task := range tasks {
		c.downloader.Delete(task.Id)

		// 软删除 task
		c.db.Model(&task).Update("deleted_at", now)

		// 级联软删除关联数据
		c.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Update("deleted_at", now)

		var resourceIDs []int
		c.db.Model(&model.DownloadResource{}).Where("task_id = ?", task.Id).Pluck("id", &resourceIDs)
		if len(resourceIDs) > 0 {
			c.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)
			c.db.Model(&model.DownloadSegment{}).Where("resource_id IN ?", resourceIDs).Update("deleted_at", now)

			var endpointIDs []int
			c.db.Model(&model.DownloadEndpoint{}).Where("resource_id IN ?", resourceIDs).Pluck("id", &endpointIDs)
			if len(endpointIDs) > 0 {
				c.db.Model(&model.DownloadConnection{}).Where("endpoint_id IN ?", endpointIDs).Update("deleted_at", now)
			}
		}

		if body.DeleteFiles && task.SavePath != "" {
			os.RemoveAll(task.SavePath)
		}

		cleared++
	}

	result.Ok(ctx, gin.H{"cleared": cleared})
}

// buildTaskInput 将 DownloadInfo 和 DownloadConfig 转换为 hook 所需的 TaskInput。
func buildTaskInput(info *registry.DownloadInfo, bodyCfg DownloadConfig) *hermes.TaskInput {
	taskInfo := hermes.TaskInfo{
		Name:     info.Task.Name,
		SavePath: info.Task.SavePath,
	}

	resources := make([]hermes.ResourceInfo, 0, len(info.Resources))
	for _, ri := range info.Resources {
		endpoints := make([]hermes.EndpointInfo, 0, len(ri.Endpoints))
		for _, ep := range ri.Endpoints {
			endpoints = append(endpoints, hermes.EndpointInfo{
				Protocol: ep.Protocol,
				URL:      ep.URL,
			})
		}
		resources = append(resources, hermes.ResourceInfo{
			ID:        ri.Resource.Id,
			Name:      ri.Resource.Name,
			Kind:      ri.Resource.Kind,
			Size:      ri.Resource.Size,
			UniqueID:  ri.Resource.UniqueID,
			Endpoints: endpoints,
		})
	}

	config := map[string]any{
		"save_path":      bodyCfg.SavePath,
		"filename":       bodyCfg.Filename,
		"spec":           specFromJSON(bodyCfg.Spec),
		"download_cover": bodyCfg.DownloadCover,
		"overwrite":      bodyCfg.Overwrite,
		"duplicate":      bodyCfg.Duplicate,
	}

	// 合并 ConfigJSON 中的下载配置
	if info.Task.ConfigJSON != "" {
		var taskCfg map[string]any
		if json.Unmarshal([]byte(info.Task.ConfigJSON), &taskCfg) == nil {
			for k, v := range taskCfg {
				if _, exists := config[k]; !exists {
					config[k] = v
				}
			}
		}
	}
	// 合并 MetadataJSON 中的内容元数据，供 hooks 使用
	if info.Task.MetadataJSON != "" {
		var meta map[string]any
		if json.Unmarshal([]byte(info.Task.MetadataJSON), &meta) == nil {
			for k, v := range meta {
				if _, exists := config[k]; !exists {
					config[k] = v
				}
			}
		}
	}

	// 解析内容元数据，供 hooks 单独访问
	metadata := make(map[string]any)
	if info.Task.MetadataJSON != "" {
		json.Unmarshal([]byte(info.Task.MetadataJSON), &metadata)
	}

	return &hermes.TaskInput{
		Task:      taskInfo,
		Config:    config,
		Metadata:  metadata,
		Resources: resources,
	}
}

// applyTaskInputModifications 将 hook 返回的修改应用到 DownloadInfo。
func applyTaskInputModifications(info *registry.DownloadInfo, modified *hermes.TaskInput) {
	if modified == nil {
		return
	}

	if modified.Task.Name != "" {
		info.Task.Name = modified.Task.Name
	}
	if modified.Task.SavePath != "" {
		info.Task.SavePath = modified.Task.SavePath
	}

	for i, modRes := range modified.Resources {
		if i >= len(info.Resources) {
			break
		}
		if modRes.Name != "" {
			info.Resources[i].Resource.Name = modRes.Name
		}
	}
}

// specFromJSON 将请求中的 spec 字段转换为 *string，区分三种情况：
//   - 字段未传入（len==0）→ nil
//   - 字段为 null（"null"）→ 指向空字符串的指针
//   - 字段为字符串值 → 指向该字符串的指针
func specFromJSON(raw json.RawMessage) *string {
	if len(raw) == 0 {
		return nil
	}
	if string(raw) == "null" {
		s := ""
		return &s
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	return &s
}

// hasStreamResources 检查任务是否包含 STREAM 类型资源（直播流）。
func (c *APIClient) hasStreamResources(taskID int) bool {
	if c.db == nil {
		return false
	}
	var count int64
	c.db.Model(&model.DownloadResource{}).
		Where("task_id = ? AND resource_type = ?", taskID, model.ResourceTypeStream).
		Count(&count)
	return count > 0
}
