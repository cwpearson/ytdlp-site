package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"golang.org/x/sys/unix"

	"ytdlp-site/config"
	"ytdlp-site/ffmpeg"
	"ytdlp-site/ytdlp"
)

// GetFreeSpace returns the free space in bytes for the filesystem containing the given directory
func getFreeSpace(dir string) (uint64, error) {
	var stat unix.Statfs_t
	err := unix.Statfs(dir, &stat)
	if err != nil {
		return 0, fmt.Errorf("error getting filesystem stats: %v", err)
	}

	// Calculate free space
	freeSpace := stat.Bavail * uint64(stat.Bsize)
	return freeSpace, nil
}

// GetDirectorySize calculates the total size of a directory in bytes
func getDirectorySize(dir string) (int64, error) {
	var size int64
	err := filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	if err != nil {
		return 0, fmt.Errorf("error walking directory: %v", err)
	}
	return size, nil
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
	used, err := getDirectorySize(config.GetDataDir())
	if err != nil {
		log.Errorln(err)
	}

	freeMiB := float64(free) / 1024 / 1024
	usedMiB := float64(used) / 1024 / 1024

	return c.Render(http.StatusOK, "status.html", map[string]interface{}{
		"ytdlp":  string(ytdlpStdout),
		"ffmpeg": string(ffmpegStdout),
		"free":   fmt.Sprintf("%.2f", freeMiB),
		"used":   fmt.Sprintf("%.2f", usedMiB),
		"Footer": MakeFooter(),
	})
}
