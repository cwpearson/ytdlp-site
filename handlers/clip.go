package handlers

import (
	"path/filepath"
	"strconv"
	"ytdlp-site/config"
	"ytdlp-site/database"
	"ytdlp-site/ffmpeg"
	"ytdlp-site/media"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func ClipPost(c echo.Context) error {

	videoID := c.FormValue("video_id")

	fromSecs, err := strconv.ParseFloat(c.FormValue("from_secs"), 64)
	if err != nil {
		return err
	}
	if fromSecs < 0 {
		fromSecs = 0
	}
	toSecs, err := strconv.ParseFloat(c.FormValue("to_secs"), 64)
	if err != nil {
		return err
	}

	var video media.Video
	err = database.Get().Where("id = ?", videoID).First(&video).Error
	if err != nil {
		return err
	}

	dstBase := uuid.Must(uuid.NewV7()).String()
	dstName := dstBase + filepath.Ext(video.Filename)
	dstPath := filepath.Join(config.GetDataDir(), dstName)

	srcPath := filepath.Join(config.GetDataDir(), video.Filename)
	log.Debugf("Clip from %s [%f-%f]", srcPath, fromSecs, toSecs)
	err = ffmpeg.Clip(srcPath, dstPath, fromSecs, toSecs)
	if err != nil {
		return err
	}

	clip := media.VideoClip{
		VideoFile:  video.VideoFile,
		OriginalID: video.OriginalID,
		VideoID:    video.ID,
		StartMS:    uint(fromSecs*1000 + 0.5),
		StopMS:     uint(toSecs*1000 + 0.5),
	}
	clip.Filename = dstName

	return database.Get().Create(&clip).Error
}
