package wxchannels

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

func init() {
	registry.Register(&handler{})
}

type handler struct{}

func (h *handler) PlatformID() string { return PlatformID }

func (h *handler) BuildDownloadTask(contentJSON json.RawMessage, config registry.DownloadConfig) (*registry.DownloadInfo, *model.Content, *model.Account, error) {
	var obj scraper.ChannelsObject
	if err := json.Unmarshal(contentJSON, &obj); err != nil {
		return nil, nil, nil, err
	}

	content, err := ToContent(&obj)
	if err != nil {
		return nil, nil, nil, err
	}
	account, err := ToAccount(&obj)
	if err != nil {
		return nil, nil, nil, err
	}

	title := config.Filename
	if title == "" {
		title = ObjectTitle(&obj)
	}
	var spec string
	if config.Spec == nil {
		// 前端未传入 spec 字段时，根据配置决定是否自动挑选清晰度
		if !GetChannelsConfig().DownloadDefaultHighest {
			spec = PickSpec(&obj)
		}
	} else {
		spec = *config.Spec
	}
	coverURL := strings.TrimSpace(content.CoverURL)
	if len(obj.ObjectDesc.Media) > 0 {
		if candidate := strings.TrimSpace(obj.ObjectDesc.Media[0].CoverUrl); candidate != "" {
			coverURL = candidate
		} else if candidate := strings.TrimSpace(obj.ObjectDesc.Media[0].ThumbUrl); candidate != "" {
			coverURL = candidate
		}
	}
	savePath := config.SavePath
	if savePath == "" {
		savePath = "/downloads/wx_channels"
	}

	contact, _ := pickAccountContact(&obj)
	extraJSON := buildExtraJSON(obj.ID, title, spec, int64(obj.CreateTime), contact.Nickname)

	// 封面下载：仅创建封面资源
	if config.Suffix == ".jpg" && coverURL != "" {
		configJSON, _ := json.Marshal(buildConfigJSON(config))
		metadataJSON, _ := json.Marshal(map[string]any{
			"platform":     PlatformID,
			"id":           content.ExternalId,
			"external_id":  content.ExternalId,
			"nonce_id":     content.ExternalId2,
			"created_at":   obj.CreateTime,
			"author":       contact.Nickname,
			"download_at":  time.Now().Unix(),
		})
		coverResource := model.DownloadResource{
			Name:       title + ".jpg",
			Kind:       "cover",
			Size:       content.Size,
			UniqueID:   content.ExternalId + "_cover",
			MergeOrder: 0,
			Extra:      extraJSON,
		}
		coverEndpoint := model.DownloadEndpoint{
			Protocol: "https",
			URL:      coverURL,
			Enabled:  1,
		}
		return &registry.DownloadInfo{
			Task: model.DownloadTaskV1{
				Name:         title,
				Status:       model.TaskStatusWaiting,
				SavePath:     savePath,
				ConfigJSON:   string(configJSON),
				MetadataJSON: string(metadataJSON),
			},
			Resource:  coverResource,
			Endpoint:  coverEndpoint,
			Resources: []registry.DownloadResourceInfo{{
				Resource:  coverResource,
				Endpoints: []model.DownloadEndpoint{coverEndpoint},
			}},
		}, content, account, nil
	}

	// 图片类型：每个 media 项创建一个下载资源，同时包含背景音乐
	if obj.ObjectDesc.MediaType == scraper.MediaTypePicture {
		files := obj.Files
		if len(files) == 0 {
			files = obj.ObjectDesc.Media
		}
		if len(files) == 0 {
			return nil, nil, nil, fmt.Errorf("图片类型缺少文件数据")
		}

		resources := make([]registry.DownloadResourceInfo, 0, len(files)+1)
		for i, file := range files {
			mediaURL := getMediaURL(file)
			if mediaURL == "" {
				return nil, nil, nil, fmt.Errorf("图片 %d 下载地址为空", i+1)
			}
			imageName := title
			if len(files) > 1 {
				imageName = fmt.Sprintf("%s_%d", title, i+1)
			}
			resources = append(resources, registry.DownloadResourceInfo{
				Resource: model.DownloadResource{
					Name:     sanitizeFilename(imageName, mediaURL),
					Kind:     "picture",
					Size:     int64(file.FileSize),
					UniqueID: content.ExternalId + "_" + strconv.Itoa(i),
					Extra:    extraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "https",
					URL:      mediaURL,
					Enabled:  1,
				}},
			})
		}

		// 背景音乐
		bgm := formatBGM(&obj)
		if bgm != nil {
			resources = append(resources, registry.DownloadResourceInfo{
				Resource: model.DownloadResource{
					Name:     bgm.Name,
					Kind:     "audio",
					UniqueID: content.ExternalId + "_bgm",
					Extra:    extraJSON,
				},
				Endpoints: []model.DownloadEndpoint{{
					Protocol: "http",
					URL:      bgm.URL,
					Enabled:  1,
				}},
			})
		}

		configJSON, _ := json.Marshal(buildConfigJSON(config))
		metadataJSON, _ := json.Marshal(map[string]any{
			"platform":     PlatformID,
			"id":           content.ExternalId,
			"external_id":  content.ExternalId,
			"nonce_id":     content.ExternalId2,
			"content_type": "picture",
			"created_at":   obj.CreateTime,
			"author":       contact.Nickname,
			"download_at":  time.Now().Unix(),
			"decode_key":   content.ExternalId3,
		})

		return &registry.DownloadInfo{
			Task: model.DownloadTaskV1{
				Name:         title,
				Status:       model.TaskStatusWaiting,
				SavePath:     savePath,
				ConfigJSON:   string(configJSON),
				MetadataJSON: string(metadataJSON),
			},
			Resource:  resources[0].Resource,
			Endpoint:  resources[0].Endpoints[0],
			Resources: resources,
		}, content, account, nil
	}

	// 视频类型
	downloadURL := BuildDownloadURLWithSpec(&obj, spec)
	if downloadURL == "" {
		return nil, nil, nil, fmt.Errorf("无法获取视频下载地址")
	}

	configJSON, _ := json.Marshal(buildConfigJSON(config))
	metadataJSON, _ := json.Marshal(map[string]any{
		"platform":    PlatformID,
		"id":          content.ExternalId,
		"external_id": content.ExternalId,
		"nonce_id":    content.ExternalId2,
		"spec":        spec,
		"created_at":  obj.CreateTime,
		"author":      contact.Nickname,
		"download_at": time.Now().Unix(),
		"decode_key":  content.ExternalId3,
	})

	videoResource := model.DownloadResource{
		Name:     title + ".mp4",
		Kind:     "video",
		Size:     content.Size,
		UniqueID: content.ExternalId + "_" + spec,
		Extra:    extraJSON,
	}
	videoEndpoint := model.DownloadEndpoint{
		Protocol: "https",
		URL:      downloadURL,
		Enabled:  1,
	}
	resources := []registry.DownloadResourceInfo{{
		Resource:  videoResource,
		Endpoints: []model.DownloadEndpoint{videoEndpoint},
	}}
	if config.DownloadCover && coverURL != "" {
		resources = append(resources, registry.DownloadResourceInfo{
			Resource: model.DownloadResource{
				Name:       title + ".jpg",
				Kind:       "cover",
				UniqueID:   content.ExternalId + "_cover",
				MergeOrder: 1,
				Extra:      extraJSON,
			},
			Endpoints: []model.DownloadEndpoint{{
				Protocol: "https",
				URL:      coverURL,
				Enabled:  1,
			}},
		})
	}

	return &registry.DownloadInfo{
		Task: model.DownloadTaskV1{
			Name:         title,
			Status:       model.TaskStatusWaiting,
			SavePath:     savePath,
			ConfigJSON:   string(configJSON),
			MetadataJSON: string(metadataJSON),
		},
		Resource:  videoResource,
		Endpoint:  videoEndpoint,
		Resources: resources,
	}, content, account, nil
}

// getMediaURL returns the combined download URL for a media item (url + urlToken).
// Mirrors JS get_media_url.
func getMediaURL(media scraper.ChannelsMediaItem) string {
	return media.URL + media.URLToken
}

// bgmInfo holds background music download info extracted from a picture feed.
type bgmInfo struct {
	URL  string
	Name string
}

// formatBGM extracts background music info from a picture feed's followPostInfo.
// Returns nil if no valid music URL is found. Mirrors JS format_bgm.
func formatBGM(obj *scraper.ChannelsObject) *bgmInfo {
	musicInfo := obj.ObjectDesc.FollowPostInfo.MusicInfo
	if musicInfo.MediaStreamingUrl == "" {
		return nil
	}
	name := "bgm.mp3"
	if musicInfo.Name != "" {
		name = sanitizeBGMName(musicInfo.Name) + ".mp3"
	}
	return &bgmInfo{URL: musicInfo.MediaStreamingUrl, Name: name}
}

// sanitizeBGMName removes characters unsafe for filenames.
func sanitizeBGMName(name string) string {
	r := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", "\"", "_", "<", "_", ">", "_", "|", "_",
	)
	return r.Replace(name)
}

// buildExtraJSON 构建 resource.Extra JSON 字符串，供 onFilename hook 的 meta 参数使用。
func buildExtraJSON(id, title, spec string, createdAt int64, author string) string {
	data, _ := json.Marshal(map[string]string{
		"id":         id,
		"title":      title,
		"spec":       spec,
		"created_at": strconv.FormatInt(createdAt, 10),
		"author":     author,
	})
	return string(data)
}

// buildConfigJSON returns a map containing only the config fields whose value is true.
// This keeps config_json compact by omitting empty/false fields.
func buildConfigJSON(config registry.DownloadConfig) map[string]any {
	m := make(map[string]any)
	if config.Suffix != "" {
		m["suffix"] = config.Suffix
	}
	if config.DownloadCover {
		m["download_cover"] = true
	}
	if config.Overwrite {
		m["overwrite"] = true
	}
	if config.Duplicate {
		m["duplicate"] = true
	}
	if config.ConvertMP3 {
		m["convert_mp3"] = true
	}
	if config.UploadCloud {
		m["upload_cloud"] = true
	}
	return m
}

// sanitizeFilename ensures the filename has an extension, extracting from the URL if needed.
func sanitizeFilename(name string, rawURL string) string {
	ext := filepath.Ext(name)
	if ext == "" {
		if base := filepath.Base(rawURL); base != "" {
			if idx := strings.Index(base, "?"); idx >= 0 {
				base = base[:idx]
			}
			ext = filepath.Ext(base)
		}
		if ext == "" {
			ext = ".jpg"
		}
	}
	nameWithoutExt := strings.TrimSuffix(name, filepath.Ext(name))
	return nameWithoutExt + ext
}
