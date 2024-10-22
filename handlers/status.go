package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"

	"github.com/labstack/echo/v4"
	"golang.org/x/sys/unix"
	"gorm.io/gorm"

	"ytdlp-site/config"
	"ytdlp-site/database"
	"ytdlp-site/ffmpeg"
	"ytdlp-site/media"
	"ytdlp-site/ytdlp"
)

// GetFreeSpace returns the free space in bytes for the filesystem containing the given directory
func getFreeSpace(dir string) (int64, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(dir, &stat)
	if err != nil {
		return 0, fmt.Errorf("error getting filesystem stats: %v", err)
	}

	// Calculate free space
	freeSpace := int64(stat.Bavail) * int64(stat.Bsize)
	return freeSpace, nil
}

type SizeEntry struct {
	Name string
	Size int64
}

// getDirectoryFiles calculates the total size of a directory in bytes
func getDirectoryFiles(dir string) ([]SizeEntry, error) {

	ret := []SizeEntry{}
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			ret = append(ret, SizeEntry{info.Name(), info.Size()})
		}
		return nil
	})
	if err != nil {
		return ret, fmt.Errorf("error walking directory: %v", err)
	}
	return ret, nil
}

// original ID, error
func getOriginalId(db *gorm.DB, filename string) (uint, error) {
	var video media.Video
	result := db.Where("filename = ?", filename).First(&video)
	if result.Error == nil && result.RowsAffected == 1 {
		return video.OriginalID, nil
	}
	var audio media.Audio
	result = db.Where("filename = ?", filename).First(&audio)
	if result.Error == nil && result.RowsAffected == 1 {
		return audio.OriginalID, nil
	}

	return 0, fmt.Errorf("no media found")
}

func StatusGet(c echo.Context) error {

	ytdlpStdout, _, err := ytdlp.Run("--version")
	if err != nil {
		log.Errorln(err)
	}
	ffmpegStdout, _, err := ffmpeg.Ffmpeg("-version")
	if err != nil {
		log.Errorln(err)
	}

	free, err := getFreeSpace(config.GetDataDir())
	if err != nil {
		log.Errorln(err)
	}
	entries, err := getDirectoryFiles(config.GetDataDir())
	if err != nil {
		log.Errorln(err)
	}

	var used, maxSize int64
	for _, entry := range entries {
		used += entry.Size
		if entry.Size > maxSize {
			maxSize = entry.Size
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Size > entries[j].Size
	})

	freeMiB := float64(free) / 1024 / 1024
	usedMiB := float64(used) / 1024 / 1024
	totalMib := float64(free+used) / 1024 / 1024

	fileSizes := make([]map[string]interface{}, 0)
	for _, entry := range entries {
		m := map[string]interface{}{
			"name":  entry.Name,
			"size":  fmt.Sprintf("%.2f", float64(entry.Size)/1024/1024),
			"value": int(100*float64(entry.Size)/float64(maxSize) + 0.5),
			"max":   100,
		}
		fileSizes = append(fileSizes, m)

		m["original_id"] = ""
		m["playlist_id"] = ""
		originalId, err := getOriginalId(database.Get(), entry.Name)
		if err == nil {
			m["original_id"] = fmt.Sprintf("%d", originalId)
		}

	}

	return c.Render(http.StatusOK, "status.html", map[string]interface{}{
		"ytdlp":  string(ytdlpStdout),
		"ffmpeg": string(ffmpegStdout),
		"free":   fmt.Sprintf("%.2f", freeMiB),
		"used":   fmt.Sprintf("%.2f", usedMiB),
		"total":  fmt.Sprintf("%.2f", totalMib),
		"files":  fileSizes,
		"Footer": MakeFooter(),
	})
}
