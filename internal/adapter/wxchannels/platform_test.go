package wxchannels

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"wx_channel/internal/database/model"
	"wx_channel/internal/download/registry"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

func TestBuildDownloadTaskWithCoverCreatesMultipleResources(t *testing.T) {
	obj := scraper.ChannelsObject{
		ID:            "feed123",
		ObjectNonceId: "nonce123",
		Type:          "media",
		Contact: scraper.ChannelsContact{
			Username: "test_user",
			Nickname: "测试用户",
		},
		ObjectDesc: scraper.ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   scraper.MediaTypeVideo,
			Media: []scraper.ChannelsMediaItem{{
				URL:      "https://video.example.com/video.mp4?token=video",
				CoverUrl: "https://image.example.com/cover.jpg?token=cover",
				ThumbUrl: "https://image.example.com/thumb.jpg",
				FileSize: 1024,
			}},
		},
	}
	raw, err := json.Marshal(obj)
	require.NoError(t, err)

	info, _, _, err := (&handler{}).BuildDownloadTask(raw, registry.DownloadConfig{
		SavePath:      "/downloads",
		Filename:      "自定义名称",
		DownloadCover: true,
	})
	require.NoError(t, err)
	require.NotNil(t, info)
	require.Len(t, info.Resources, 2)
	assert.Equal(t, "video", info.Resources[0].Resource.Kind)
	assert.Equal(t, "自定义名称.mp4", info.Resources[0].Resource.Name)
	assert.Equal(t, "https://video.example.com/video.mp4?token=video", info.Resources[0].Endpoints[0].URL)
	assert.Equal(t, "cover", info.Resources[1].Resource.Kind)
	assert.Equal(t, "自定义名称.jpg", info.Resources[1].Resource.Name)
	assert.Equal(t, "https://image.example.com/cover.jpg?token=cover", info.Resources[1].Endpoints[0].URL)
}

// mergedLiveContentJSON simulates what the frontend sends after
// Object.assign({}, joinLiveData, profileData).  The profile (ChannelsObject)
// contributes anchorContact, contact, nickname, username; the joinLive
// response contributes liveSdkInfo, liveInfo, liveDescription.
const mergedLiveContentJSON = `{
	"liveSdkInfo": {
		"liveCdnUrl": "http://pull-m1.wxlivecdn.com/trtc_1400419933/orig_live_stream.flv?token=abc123"
	},
	"liveInfo": {
		"liveId": "2078967496773105135",
		"startTime": 1785075244
	},
	"liveDescription": "谁可以无缘无故给我刷个岛",
	"nickname": "小玉来了哦",
	"username": "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder",
	"contact": {
		"username": "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder",
		"nickname": "小玉来了哦",
		"headUrl": "https://example.com/contact_avatar.jpg"
	},
	"anchorContact": {
		"username": "anchor_user",
		"nickname": "主播",
		"headUrl": "https://example.com/anchor_avatar.jpg",
		"coverImgUrl": "https://example.com/live_cover.jpg"
	}
}`

func TestBuildDownloadTask_LiveStream_DetectsJoinLiveContent(t *testing.T) {
	raw := json.RawMessage(mergedLiveContentJSON)
	info, content, account, err := (&handler{}).BuildDownloadTask(raw, registry.DownloadConfig{})
	require.NoError(t, err)
	require.NotNil(t, info)
	require.NotNil(t, content)
	require.NotNil(t, account)

	// ---- Verify Content ----
	t.Run("Content", func(t *testing.T) {
		assert.Equal(t, "wx_channels:2078967496773105135", content.Id)
		assert.Equal(t, "wx_channels", content.PlatformId)
		assert.Equal(t, "2078967496773105135", content.ExternalId)
		assert.Equal(t, "live", content.ContentType)
		assert.Equal(t, "谁可以无缘无故给我刷个岛", content.Title)
		require.NotNil(t, content.PublishTime)
		assert.Equal(t, int64(1785075244), *content.PublishTime)
	})

	// ---- Verify Account (anchorContact preferred for live) ----
	t.Run("Account", func(t *testing.T) {
		assert.Equal(t, "wx_channels:anchor_user", account.Id)
		assert.Equal(t, "anchor_user", account.Username)
		assert.Equal(t, "anchor_user", account.ExternalId)
		assert.Equal(t, "主播", account.Nickname)
		assert.Equal(t, "https://example.com/anchor_avatar.jpg", account.AvatarURL)
	})

	// ---- Verify DownloadTaskV1 ----
	t.Run("DownloadTaskV1", func(t *testing.T) {
		assert.Equal(t, "谁可以无缘无故给我刷个岛", info.Task.Name)
		assert.Equal(t, model.TaskStatusWaiting, info.Task.Status)
		assert.Equal(t, "/downloads/wx_channels", info.Task.SavePath)

		var meta map[string]any
		require.NoError(t, json.Unmarshal([]byte(info.Task.MetadataJSON), &meta))
		assert.Equal(t, "wx_channels", meta["platform"])
		assert.Equal(t, "2078967496773105135", meta["id"])
		assert.Equal(t, "live", meta["content_type"])
		assert.Equal(t, "主播", meta["author"])
	})

	// ---- Verify DownloadResource (STREAM type) ----
	t.Run("DownloadResource", func(t *testing.T) {
		r := info.Resource
		assert.Equal(t, "谁可以无缘无故给我刷个岛.mkv", r.Name)
		assert.Equal(t, "stream", r.Kind)
		assert.Equal(t, model.ResourceTypeStream, r.ResourceType)
		assert.Equal(t, 1, r.IsLive)
		assert.Equal(t, 10, r.RotateMinutes)
		assert.Equal(t, "2078967496773105135_1785075244", r.UniqueID)
		assert.Equal(t, "http://pull-m1.wxlivecdn.com/trtc_1400419933/orig_live_stream.flv?token=abc123", r.StreamURL)
	})

	// ---- Verify DownloadEndpoint ----
	t.Run("DownloadEndpoint", func(t *testing.T) {
		assert.Equal(t, "livestream", info.Endpoint.Protocol)
		assert.Equal(t, "http://pull-m1.wxlivecdn.com/trtc_1400419933/orig_live_stream.flv?token=abc123", info.Endpoint.URL)
		assert.Equal(t, 1, info.Endpoint.Enabled)
	})

	// ---- Verify Resources list ----
	require.Len(t, info.Resources, 1)
	assert.Equal(t, info.Resource, info.Resources[0].Resource)
	assert.Len(t, info.Resources[0].Endpoints, 1)
	assert.Equal(t, info.Endpoint, info.Resources[0].Endpoints[0])
}

func TestBuildDownloadTask_LiveStream_ContactFallback(t *testing.T) {
	// Payload with contact but NO anchorContact — should fall back to contact
	raw := json.RawMessage(`{
		"liveSdkInfo": { "liveCdnUrl": "http://live.example.com/stream.flv" },
		"liveInfo": { "liveId": "abc123", "startTime": 1700000000 },
		"liveDescription": "测试直播",
		"contact": {
			"username": "contact_user",
			"nickname": "联系人昵称",
			"headUrl": "https://example.com/contact.jpg"
		}
	}`)

	info, _, account, err := (&handler{}).BuildDownloadTask(raw, registry.DownloadConfig{})
	require.NoError(t, err)
	require.NotNil(t, info)
	require.NotNil(t, account)

	assert.Equal(t, "wx_channels:contact_user", account.Id)
	assert.Equal(t, "contact_user", account.Username)
	assert.Equal(t, "联系人昵称", account.Nickname)
	assert.Equal(t, "https://example.com/contact.jpg", account.AvatarURL)
	assert.Equal(t, "测试直播", info.Task.Name)

	var meta map[string]any
	require.NoError(t, json.Unmarshal([]byte(info.Task.MetadataJSON), &meta))
	assert.Equal(t, "联系人昵称", meta["author"])
}

func TestBuildDownloadTask_LiveStream_NicknameFallback(t *testing.T) {
	// Payload with NO contact and NO anchorContact — should fall back to top-level fields
	raw := json.RawMessage(`{
		"liveSdkInfo": { "liveCdnUrl": "http://live.example.com/stream.flv" },
		"liveInfo": { "liveId": "simple_live", "startTime": 1700000000 },
		"liveDescription": "极简直播",
		"nickname": "顶层昵称",
		"username": "top_level_user"
	}`)

	info, _, account, err := (&handler{}).BuildDownloadTask(raw, registry.DownloadConfig{})
	require.NoError(t, err)
	require.NotNil(t, info)
	require.NotNil(t, account)

	assert.Equal(t, "wx_channels:top_level_user", account.Id)
	assert.Equal(t, "top_level_user", account.Username)
	assert.Equal(t, "顶层昵称", account.Nickname)
	assert.Equal(t, "", account.AvatarURL)
	assert.Equal(t, "极简直播", info.Task.Name)

	var meta map[string]any
	require.NoError(t, json.Unmarshal([]byte(info.Task.MetadataJSON), &meta))
	assert.Equal(t, "顶层昵称", meta["author"])
}

func TestBuildDownloadTask_LiveStream_NoLiveDescription(t *testing.T) {
	// Payload without liveDescription — title should default to "直播"
	raw := json.RawMessage(`{
		"liveSdkInfo": { "liveCdnUrl": "http://live.example.com/stream.flv" },
		"liveInfo": { "liveId": "no_desc_live" },
		"nickname": "阿强",
		"username": "aqiang"
	}`)

	info, content, _, err := (&handler{}).BuildDownloadTask(raw, registry.DownloadConfig{})
	require.NoError(t, err)
	require.NotNil(t, info)
	require.NotNil(t, content)

	assert.Equal(t, "直播", content.Title)
	assert.Equal(t, "直播.mkv", info.Resource.Name)
}

func TestBuildDownloadTask_LiveStream_CustomFilename(t *testing.T) {
	raw := json.RawMessage(mergedLiveContentJSON)
	info, _, _, err := (&handler{}).BuildDownloadTask(raw, registry.DownloadConfig{
		Filename: "我的直播录制",
		SavePath: "/downloads/live",
	})
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "我的直播录制", info.Task.Name)
	assert.Equal(t, "我的直播录制.mkv", info.Resource.Name)
	assert.Equal(t, "/downloads/live", info.Task.SavePath)
}

func TestBuildDownloadTask_LiveStream_NoLiveId(t *testing.T) {
	// Payload without liveInfo — liveId should be empty string
	raw := json.RawMessage(`{
		"liveSdkInfo": { "liveCdnUrl": "http://live.example.com/stream.flv" },
		"liveDescription": "无ID直播",
		"nickname": "测试用户",
		"username": "test_user"
	}`)

	info, content, account, err := (&handler{}).BuildDownloadTask(raw, registry.DownloadConfig{})
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "wx_channels:", content.Id)
	assert.Equal(t, "", content.ExternalId)
	assert.NotEmpty(t, info.Resource.UniqueID)
	assert.Contains(t, info.Resource.UniqueID, "_")
	assert.NotNil(t, account)
}

func TestJoinLivePayload_Parse(t *testing.T) {
	var jl joinLivePayload
	require.NoError(t, json.Unmarshal([]byte(mergedLiveContentJSON), &jl))

	// liveSdkInfo
	require.NotNil(t, jl.LiveSdkInfo)
	assert.Equal(t, "http://pull-m1.wxlivecdn.com/trtc_1400419933/orig_live_stream.flv?token=abc123", jl.LiveSdkInfo.LiveCdnUrl)

	// liveInfo
	require.NotNil(t, jl.LiveInfo)
	assert.Equal(t, "2078967496773105135", jl.LiveInfo.LiveId)
	assert.Equal(t, 1785075244, jl.LiveInfo.StartTime)

	// liveDescription
	assert.Equal(t, "谁可以无缘无故给我刷个岛", jl.LiveDescription)

	// Top-level fields
	assert.Equal(t, "小玉来了哦", jl.Nickname)
	assert.Equal(t, "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder", jl.Username)

	// anchorContact (from profile)
	require.NotNil(t, jl.AnchorContact)
	assert.Equal(t, "anchor_user", jl.AnchorContact.Username)
	assert.Equal(t, "主播", jl.AnchorContact.Nickname)
	assert.Equal(t, "https://example.com/anchor_avatar.jpg", jl.AnchorContact.HeadUrl)
	assert.Equal(t, "https://example.com/live_cover.jpg", jl.AnchorContact.CoverImgUrl)

	// contact (from profile)
	require.NotNil(t, jl.Contact)
	assert.Equal(t, "v2_060000231003b20faec8c7e68a1ccad0c70cef35b077ac3113d4169c015905408166537ba68d@finder", jl.Contact.Username)
	assert.Equal(t, "小玉来了哦", jl.Contact.Nickname)
	assert.Equal(t, "https://example.com/contact_avatar.jpg", jl.Contact.HeadUrl)
}

func TestBuildDownloadTask_NotLive_NotJoinLive(t *testing.T) {
	// A regular video feed should NOT be detected as joinLive
	obj := scraper.ChannelsObject{
		ID:   "video_feed_123",
		Type: "media",
		Contact: scraper.ChannelsContact{
			Username: "test_user",
			Nickname: "测试用户",
		},
		ObjectDesc: scraper.ChannelsObjectDesc{
			Description: "测试视频",
			MediaType:   scraper.MediaTypeVideo,
			Media: []scraper.ChannelsMediaItem{{
				URL:      "https://video.example.com/video.mp4",
				CoverUrl: "https://image.example.com/cover.jpg",
				FileSize: 1024,
			}},
		},
	}
	raw, err := json.Marshal(obj)
	require.NoError(t, err)

	info, content, _, err := (&handler{}).BuildDownloadTask(raw, registry.DownloadConfig{})
	require.NoError(t, err)
	require.NotNil(t, info)
	require.NotNil(t, content)

	// Should be video type, NOT live/stream
	assert.Equal(t, "video", content.ContentType)
	assert.NotEqual(t, "stream", info.Resource.Kind)
}

func TestJoinLivePayload_Detection_NoLiveSdkInfo(t *testing.T) {
	// JSON that has liveDescription but NO liveSdkInfo — should NOT trigger joinLive path
	raw := json.RawMessage(`{
		"liveDescription": "some text",
		"liveInfo": { "liveId": "123" },
		"nickname": "test",
		"username": "test_user"
	}`)

	// This should fail since there's no liveSdkInfo (falls through to ChannelsObject,
	// which will fail because it's not valid ChannelsObject format)
	_, _, _, err := (&handler{}).BuildDownloadTask(raw, registry.DownloadConfig{})
	assert.Error(t, err)
}
