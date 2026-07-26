package wxchannels

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gorm.io/gorm"

	"wx_channel/internal/database/model"
	"wx_channel/internal/pipeline"
	scraper "wx_channel/pkg/scraper/wxchannels"
)

// PostProcessPipeline 根据用户提交的下载配置构建后处理管道。
// pc.Values["config"] 即创建任务时提交的完整配置（包含 convert_mp3、upload_cloud、suffix 等字段）。
//
// 管道链: decrypt → [convert_mp3] → [upload_cloud] → finalize
// decrypt 和 finalize 必然执行，中间节点按配置决定。
func (h *handler) PostProcessPipeline(pc *pipeline.Context) *pipeline.Pipeline {
	builder := pipeline.NewBuilder("wxchannels_postprocess").
		Add("decrypt", decryptNode).
		Add("finalize", finalizeNode)

	cfg, _ := pc.Values["config"].(map[string]any)
	enableConvertMP3, _ := cfg["convert_mp3"].(bool)
	enableUploadCloud, _ := cfg["upload_cloud"].(bool)

	prevNode := "decrypt"
	if enableConvertMP3 {
		builder.Add("convert_mp3", convertMP3Node)
		builder.Chain("decrypt", "convert_mp3")
		prevNode = "convert_mp3"
	}

	if enableUploadCloud {
		builder.Add("upload_cloud", uploadCloudNode)
		builder.Chain(prevNode, "upload_cloud")
		prevNode = "upload_cloud"
	}

	builder.Chain(prevNode, "finalize")
	return builder.Build()
}

// decryptNode decrypts the downloaded file using the decode key.
// Context values:
//   - "input_file": path to the downloaded file
//   - "decode_key": decrypt key as uint64
//   - "enc_limit": encryption limit as uint64
//   - "save_path": output directory
//
// Output:
//   - "decrypted_file": path to the decrypted file
var decryptNode = pipeline.NewFuncNode("decrypt", "decrypt", func(ctx context.Context, pc *pipeline.Context) error {
	inputFile, _ := pc.Values["input_file"].(string)
	if inputFile == "" {
		return fmt.Errorf("缺少 input_file")
	}

	decodeKeyStr, _ := pc.Values["decode_key"].(string)
	if decodeKeyStr == "" {
		if key, ok := pc.Values["decode_key"].(uint64); ok {
			decodeKeyStr = strconv.FormatUint(key, 10)
		}
	}
	key, err := strconv.ParseUint(decodeKeyStr, 10, 64)
	if err != nil {
		return fmt.Errorf("解析 decode_key 失败: %w", err)
	}

	encLimit, _ := pc.Values["enc_limit"].(uint64)

	tmpFile := inputFile + ".tmp"
	if err := scraper.DecryptFile(inputFile, tmpFile, key, encLimit); err != nil {
		_ = os.Remove(tmpFile)
		return err
	}
	if err := os.Rename(tmpFile, inputFile); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("原地替换解密文件失败: %w", err)
	}

	pc.Values["decrypted_file"] = inputFile
	return nil
})

// convertMP3Node converts the decrypted file to MP3 using ffmpeg.
// Context values:
//   - "decrypted_file": path to the decrypted file
//   - "save_path": output directory
//
// Output:
//   - "mp3_file": path to the MP3 file
var convertMP3Node = pipeline.NewFuncNode("convert_mp3", "convert_mp3", func(ctx context.Context, pc *pipeline.Context) error {
	decryptedFile, _ := pc.Values["decrypted_file"].(string)
	if decryptedFile == "" {
		return fmt.Errorf("缺少 decrypted_file")
	}

	baseName := filepath.Base(decryptedFile)
	ext := filepath.Ext(baseName)
	mp3File := filepath.Join(filepath.Dir(decryptedFile), strings.TrimSuffix(baseName, ext)+".mp3")

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", decryptedFile,
		"-vn",
		"-acodec", "libmp3lame",
		"-ab", "192k",
		"-f", "mp3",
		"-y",
		mp3File,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg 转换失败: %w\n%s", err, string(output))
	}

	pc.Values["mp3_file"] = mp3File
	return nil
})

// uploadCloudNode uploads the MP3 file to cloud storage.
// Context values:
//   - "mp3_file": path to the MP3 file (如果未转 MP3 则为 decrypted_file)
//   - "decrypted_file": fallback input if mp3_file is not set
//
// Output:
//   - "upload_url": URL of the uploaded file (placeholder)
var uploadCloudNode = pipeline.NewFuncNode("upload_cloud", "upload_cloud", func(ctx context.Context, pc *pipeline.Context) error {
	uploadFile, _ := pc.Values["mp3_file"].(string)
	if uploadFile == "" {
		uploadFile, _ = pc.Values["decrypted_file"].(string)
	}
	if uploadFile == "" {
		return fmt.Errorf("缺少待上传文件路径")
	}

	if _, err := os.Stat(uploadFile); err != nil {
		return fmt.Errorf("待上传文件不存在: %w", err)
	}

	// TODO: implement actual cloud upload
	pc.Values["upload_url"] = ""
	return nil
})

// finalizeNode 管道收尾：根据最终产出文件更新 task / resource 名称并清理临时文件。
// Context values:
//   - "input_file":     原始加密文件（解密已原地修改，与 decrypted_file 相同）
//   - "decrypted_file": 解密文件（如果转了 mp3 将被清理）
//   - "mp3_file":       最终 mp3 文件（如果转了 mp3）
//   - "db":             *gorm.DB
//   - "task_id":        int
var finalizeNode = pipeline.NewFuncNode("finalize", "finalize", func(ctx context.Context, pc *pipeline.Context) error {
	db, _ := pc.Values["db"].(*gorm.DB)
	if db == nil {
		return fmt.Errorf("缺少 db")
	}
	taskID, ok := pc.Values["task_id"].(int)
	if !ok {
		return fmt.Errorf("缺少 task_id")
	}

	mp3File, hasMP3 := pc.Values["mp3_file"].(string)
	decryptedFile, _ := pc.Values["decrypted_file"].(string)

	// 确定最终产物
	var finalFile string
	if hasMP3 && mp3File != "" {
		finalFile = mp3File
	} else if decryptedFile != "" {
		finalFile = decryptedFile
	}

	if finalFile == "" {
		return fmt.Errorf("缺少最终产物文件")
	}

	// 更新 task 名称（任务名扁平化，取基础文件名即可）
	taskName := filepath.Base(finalFile)
	var task model.DownloadTaskV1
	if err := db.Where("id = ?", taskID).First(&task).Error; err == nil {
		oldName := task.Name
		if oldName != taskName {
			db.Model(&task).Update("name", taskName)
		}
	}

	// 更新第一个 resource 的名称（保留相对目录结构）
	resourceName := finalFile
	if len(finalFile) > len(task.SavePath) && strings.HasPrefix(finalFile, task.SavePath) {
		resourceName = strings.TrimPrefix(finalFile[len(task.SavePath):], string(filepath.Separator))
	}
	var resource model.DownloadResource
	if err := db.Where("task_id = ?", taskID).Order("merge_order ASC, id ASC").First(&resource).Error; err == nil {
		oldName := resource.Name
		if oldName != resourceName {
			db.Model(&resource).Updates(map[string]any{
				"name": resourceName,
				"kind": pickFinalKind(resourceName, resource.Kind),
			})
		}
	}

	// 如果转了 mp3，清理中间解密文件（解密已原地修改 inputFile，即 decryptedFile == inputFile）
	if hasMP3 && mp3File != "" && decryptedFile != "" && decryptedFile != finalFile {
		_ = os.Remove(decryptedFile)
	}

	return nil
})

// pickFinalKind 根据最终文件扩展名和原类型决定 kind。
func pickFinalKind(name, currentKind string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".mp3", ".wav", ".aac", ".flac", ".ogg":
		return "audio"
	case ".mp4", ".mkv", ".avi", ".mov", ".webm":
		return "video"
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp":
		return "picture"
	default:
		if currentKind != "" {
			return currentKind
		}
		return "file"
	}
}

// StreamPostProcessPipeline 为直播流（STREAM）构建后处理管道。
// 下载阶段产出 MKV（Matroska 支持 pipe 输出），后处理阶段将 MKV remux 为 MP4
// 以获取更好的播放兼容性。
//
// 管道链: stream_convert → finalize_stream
func StreamPostProcessPipeline(pc *pipeline.Context) *pipeline.Pipeline {
	builder := pipeline.NewBuilder("wxchannels_stream_postprocess").
		Add("stream_convert", streamConvertNode).
		Add("finalize_stream", finalizeStreamNode)

	builder.Chain("stream_convert", "finalize_stream")
	return builder.Build()
}

// streamConvertNode 对下载的流文件进行容器格式转换。
// MKV 文件可直接播放（Matroska 原生支持 HEVC），跳过转换。
// FLV/TS 等格式需 remux 为 MP4 以改善兼容性。
//
// Context values:
//   - "input_file": 原始下载文件路径
//
// Output:
//   - "mp4_file": 最终播放文件路径（可能是原 MKV 或转换后的 MP4）
var streamConvertNode = pipeline.NewFuncNode("stream_convert", "stream_convert", func(ctx context.Context, pc *pipeline.Context) error {
	inputFile, _ := pc.Values["input_file"].(string)
	if inputFile == "" {
		return fmt.Errorf("缺少 input_file")
	}

	ext := strings.ToLower(filepath.Ext(inputFile))
	// MKV 和 MP4 可直接播放，跳过转换
	if ext == ".mp4" || ext == ".mkv" {
		pc.Values["mp4_file"] = inputFile
		return nil
	}

	baseName := filepath.Base(inputFile)
	mp4File := filepath.Join(filepath.Dir(inputFile), strings.TrimSuffix(baseName, ext)+".mp4")

	// ffmpeg remux，不重新编码（File output, not pipe — +faststart is safe here）
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputFile,
		"-c", "copy",
		"-bsf:a", "aac_adtstoasc",
		"-movflags", "+faststart",
		"-y",
		mp4File,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		_ = os.Remove(mp4File)
		return fmt.Errorf("ffmpeg stream remux 失败: %w\n%s", err, string(output))
	}

	// 删除原始文件，用 MP4 替换
	if mp4File != inputFile {
		_ = os.Remove(inputFile)
	}

	pc.Values["mp4_file"] = mp4File
	return nil
})

// finalizeStreamNode 更新 task 和 resource 的名称为 MP4 后缀。
// Context values:
//   - "mp4_file":      最终 MP4 文件路径
//   - "db":            *gorm.DB
//   - "task_id":       int
//   - "save_path":     保存目录
var finalizeStreamNode = pipeline.NewFuncNode("finalize_stream", "finalize_stream", func(ctx context.Context, pc *pipeline.Context) error {
	db, _ := pc.Values["db"].(*gorm.DB)
	if db == nil {
		return fmt.Errorf("缺少 db")
	}
	taskID, ok := pc.Values["task_id"].(int)
	if !ok {
		return fmt.Errorf("缺少 task_id")
	}

	mp4File, _ := pc.Values["mp4_file"].(string)
	if mp4File == "" {
		return fmt.Errorf("缺少 mp4_file")
	}

	// 更新 task 名称
	var task model.DownloadTaskV1
	if err := db.Where("id = ?", taskID).First(&task).Error; err == nil {
		mp4Name := filepath.Base(mp4File)
		if task.Name != mp4Name {
			db.Model(&task).Update("name", mp4Name)
		}
	}

	// 更新 resource 名称
	var resource model.DownloadResource
	if err := db.Where("task_id = ?", taskID).Order("merge_order ASC, id ASC").First(&resource).Error; err == nil {
		resourceName := mp4File
		if len(mp4File) > len(task.SavePath) && strings.HasPrefix(mp4File, task.SavePath) {
			resourceName = strings.TrimPrefix(mp4File[len(task.SavePath):], string(filepath.Separator))
		}
		if resource.Name != resourceName {
			db.Model(&resource).Updates(map[string]any{
				"name": resourceName,
				"kind": "video",
			})
		}
	}

	return nil
})
