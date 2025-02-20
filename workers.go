package main

import (
	"fmt"
	"os"
	"path/filepath"
	"ytdlp-site/config"
	"ytdlp-site/ffmpeg"
	"ytdlp-site/media"
	"ytdlp-site/originals"
	"ytdlp-site/transcodes"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	maxConcurrent = 2
)

var sem = make(chan struct{}, maxConcurrent)

func ensureDirFor(path string) error {
	dir := filepath.Dir(path)
	log.Debugln("Create", dir)
	return os.MkdirAll(dir, 0700)
}

func videoToVideo(sem chan struct{}, transID uint, srcFilepath string) {
	sem <- struct{}{}        // Acquire semaphore
	defer func() { <-sem }() // release semaphore

	var trans transcodes.Transcode
	db.First(&trans, "id = ?", transID)
	originals.SetStatus(trans.OriginalID, originals.StatusTranscoding)

	// determine destination path
	dstFilename := uuid.Must(uuid.NewV7()).String()
	dstFilename = fmt.Sprintf("%s.mp4", dstFilename)
	dstFilepath := filepath.Join(config.GetDataDir(), dstFilename)

	err := ensureDirFor(dstFilepath)
	if err != nil {
		fmt.Println("Error: couldn't create dir for ", dstFilepath, err)
		db.Model(&transcodes.Transcode{}).Where("id = ?", trans.ID).Update("status", "failed")
		return
	}

	// FIXME: ignoring any requested audio bitrate
	// determine audio bitrate
	var audioBitrate uint = 160
	if trans.Height <= 144 {
		audioBitrate = 64
	} else if trans.Height <= 480 {
		audioBitrate = 96
	} else if trans.Height < 720 {
		audioBitrate = 128
	}

	// start ffmpeg
	db.Model(&transcodes.Transcode{}).Where("id = ?", trans.ID).Update("status", "running")
	var vf string
	if trans.FPS > 0 {
		vf = fmt.Sprintf("scale=-2:%d,fps=%f", trans.Height, trans.FPS)
	} else {
		vf = fmt.Sprintf("scale=-2:%d", trans.Height)
	}
	stdout, stderr, err := ffmpeg.Ffmpeg("-i", srcFilepath,
		"-vf", vf, "-c:v", "libx264",
		"-crf", "23", "-preset", "fast", "-c:a", "aac", "-b:a", fmt.Sprintf("%dk", audioBitrate),
		dstFilepath)
	if err != nil {
		fmt.Println("Error: convert to video file", srcFilepath, "->", dstFilepath, string(stdout), string(stderr))
		db.Model(&transcodes.Transcode{}).Where("id = ?", trans.ID).Update("status", "failed")
		return
	}

	// look up original
	var orig originals.Original
	db.First(&orig, "id = ?", trans.OriginalID)

	// create video record
	video := media.Video{
		VideoFile: media.VideoFile{
			MediaFile: media.MediaFile{
				Filename: dstFilename,
			},
		},
		OriginalID: orig.ID, Source: "transcode",
	}

	fileSize, err := getSize(dstFilepath)
	if err == nil {
		video.Size = fileSize
	}
	length, err := getLength(dstFilepath)
	if err == nil {
		video.Length = length
	}

	meta, err := getVideoMeta(dstFilepath)
	fmt.Println("meta for", dstFilepath, meta)
	if err == nil {
		video.Width = meta.width
		video.Height = meta.height
		video.FPS = meta.fps
	}

	db.Create(&video)

	// complete transcode
	db.Delete(&trans)
	originals.SetStatusTranscodingOrCompleted(trans.OriginalID)
}

func videoToAudio(sem chan struct{}, transID uint, videoFilepath string) {
	sem <- struct{}{}        // Acquire semaphore
	defer func() { <-sem }() // release semaphore

	var trans transcodes.Transcode
	db.First(&trans, "id = ?", transID)
	originals.SetStatus(trans.OriginalID, originals.StatusTranscoding)

	// determine destination path
	audioFilename := uuid.Must(uuid.NewV7()).String()
	audioFilename = fmt.Sprintf("%s.mp3", audioFilename)
	audioFilepath := filepath.Join(config.GetDataDir(), audioFilename)

	// ensure destination directory
	err := ensureDirFor(audioFilepath)
	if err != nil {
		fmt.Println("Error: couldn't create dir for ", audioFilepath, err)
		db.Model(&transcodes.Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	db.Model(&transcodes.Transcode{}).Where("id = ?", transID).Update("status", "running")
	_, _, err = ffmpeg.Ffmpeg("-i", videoFilepath, "-vn", "-acodec",
		"mp3", "-b:a",
		fmt.Sprintf("%dk", trans.Kbps),
		audioFilepath)
	if err != nil {
		fmt.Println("Error: convert to audio file", videoFilepath, "->", audioFilepath)
		db.Model(&transcodes.Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	// look up original

	var orig originals.Original
	db.First(&orig, "id = ?", trans.OriginalID)

	// create audio record
	audio := media.Audio{
		MediaFile: media.MediaFile{
			Filename: audioFilename,
		},
		OriginalID: orig.ID,
		Bps:        trans.Kbps * 1000,
		Source:     "transcode",
	}

	fileSize, err := getSize(audioFilepath)
	if err == nil {
		audio.Size = fileSize
	}
	length, err := getLength(audioFilepath)
	if err == nil {
		audio.Length = length
	}

	db.Create(&audio)

	// complete transcode
	db.Delete(&trans)
	originals.SetStatusTranscodingOrCompleted(trans.OriginalID)
}

func audioToAudio(sem chan struct{}, transID uint, srcFilepath string) {
	sem <- struct{}{}        // Acquire semaphore
	defer func() { <-sem }() // release semaphore

	var trans transcodes.Transcode
	db.First(&trans, "id = ?", transID)

	originals.SetStatus(trans.OriginalID, originals.StatusTranscoding)

	// determine destination path
	dstFilename := uuid.Must(uuid.NewV7()).String()
	dstFilename = fmt.Sprintf("%s.mp3", dstFilename)
	dstFilepath := filepath.Join(config.GetDataDir(), dstFilename)

	// ensure destination directory
	err := ensureDirFor(dstFilepath)
	if err != nil {
		fmt.Println("Error: couldn't create dir for ", dstFilepath, err)
		db.Model(&transcodes.Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	db.Model(&transcodes.Transcode{}).Where("id = ?", transID).Update("status", "running")
	_, _, err = ffmpeg.Ffmpeg("-i", srcFilepath, "-vn", "-acodec",
		"mp3", "-b:a",
		fmt.Sprintf("%dk", trans.Kbps),
		dstFilepath)
	if err != nil {
		fmt.Println("Error: convert to audio file", srcFilepath, "->", dstFilepath)
		db.Model(&transcodes.Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	// look up original
	var orig originals.Original
	db.First(&orig, "id = ?", trans.OriginalID)

	// create audio record
	audio := media.Audio{
		MediaFile: media.MediaFile{
			Filename: dstFilename,
		},
		OriginalID: orig.ID,
		Bps:        trans.Kbps * 1000,
		Source:     "transcode",
	}

	fileSize, err := getSize(dstFilepath)
	if err == nil {
		audio.Size = fileSize
	}
	length, err := getLength(dstFilepath)
	if err == nil {
		audio.Length = length
	}

	db.Create(&audio)

	// complete transcode
	db.Delete(&trans)
	originals.SetStatusTranscodingOrCompleted(trans.OriginalID)
}

func cleanupTranscodes() {
	log.Traceln("cleanupTranscode")

	// any running jobs here got stuck or dead in the midde, so reset them
	db.Model(&transcodes.Transcode{}).Where("status = ?", "running").Update("status", "pending")

	// find any originals with a transcode job -> transcoding
	var originalsToUpdate []uint
	db.Model(&originals.Original{}).
		Select("id").
		Where("id IN (?)",
			db.Model(&transcodes.Transcode{}).
				Select("original_id"),
		).
		Find(&originalsToUpdate)
	db.Model(&originals.Original{}).
		Where("id IN ?", originalsToUpdate).
		Update("status", originals.StatusTranscoding)

	// originals marked transcoding that don't have a transcode job -> complete
	db.Model(&originals.Original{}).
		Select("id").
		Where("status = ? AND id NOT IN (?)",
			originals.StatusTranscoding,
			db.Model(&transcodes.Transcode{}).
				Select("original_id"),
		).
		Find(&originalsToUpdate)
	db.Model(&originals.Original{}).
		Where("id IN ? AND status = ?", originalsToUpdate, originals.StatusTranscoding).
		Update("status", originals.StatusCompleted)

	// start any existing transcode jobs
	for {
		var trans transcodes.Transcode
		err := db.Where("status = ?", "pending").
			Order("CASE " +
				"WHEN dst_kind = 'video' AND height = 480 THEN 0 " +
				"WHEN dst_kind = 'audio' AND rate = 96 THEN 0 " +
				"ELSE 1 END").First(&trans).Error
		// err := db.First(&trans, "status = ?", "pending").Error
		if err == gorm.ErrRecordNotFound {
			log.Traceln("no pending transcode jobs")
			break // no more pending jobs
		}

		if trans.SrcKind == "video" {

			var srcVideo media.Video
			err = db.First(&srcVideo, "id = ?", trans.SrcID).Error
			if err != nil {
				fmt.Println("no such source video for video Transcode", trans)
				db.Delete(&trans)
				continue
			}
			srcFilepath := filepath.Join(config.GetDataDir(), srcVideo.Filename)

			if trans.DstKind == "video" {
				go videoToVideo(sem, trans.ID, srcFilepath)
			} else if trans.DstKind == "audio" {
				go videoToAudio(sem, trans.ID, srcFilepath)
			} else {
				fmt.Println("unexpected src/dst kinds for Transcode", trans)
				db.Delete(&trans)
			}
		} else if trans.SrcKind == "audio" {
			var srcAudio media.Audio
			err = db.First(&srcAudio, "id = ?", trans.SrcID).Error
			if err != nil {
				log.Errorln("no such source audio for audio Transcode", trans)
				db.Delete(&trans)
				continue
			}
			srcFilepath := filepath.Join(config.GetDataDir(), srcAudio.Filename)
			go audioToAudio(sem, trans.ID, srcFilepath)
		} else {
			fmt.Println("unexpected src kind for Transcode", trans)
			db.Delete(&trans)
		}
	}

}
