package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"ytdlp-site/ffmpeg"
	"ytdlp-site/media"
	"ytdlp-site/originals"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func ensureDirFor(path string) error {
	dir := filepath.Dir(path)
	log.Debugln("Create", dir)
	return os.MkdirAll(dir, 0700)
}

func videoToVideo(transID uint, height uint, srcFilepath string) {

	// determine destination path
	dstFilename := uuid.Must(uuid.NewV7()).String()
	dstFilename = fmt.Sprintf("%s.mp4", dstFilename)
	dstFilepath := filepath.Join(getDataDir(), dstFilename)

	err := ensureDirFor(dstFilepath)
	if err != nil {
		fmt.Println("Error: couldn't create dir for ", dstFilepath, err)
		db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	// FIXME: ignoring any requested audio bitrate

	// determine audio bitrate
	var audioBitrate uint = 160
	if height <= 144 {
		audioBitrate = 64
	} else if height <= 480 {
		audioBitrate = 96
	} else if height < 720 {
		audioBitrate = 128
	}

	// start ffmpeg
	db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "running")
	stdout, stderr, err := ffmpeg.Ffmpeg("-i", srcFilepath,
		"-vf", fmt.Sprintf("scale=-2:%d", height), "-c:v", "libx264",
		"-crf", "23", "-preset", "veryfast", "-c:a", "aac", "-b:a", fmt.Sprintf("%dk", audioBitrate),
		dstFilepath)
	if err != nil {
		fmt.Println("Error: convert to video file", srcFilepath, "->", dstFilepath, string(stdout), string(stderr))
		db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	// look up original
	var trans Transcode
	db.First(&trans, transID)
	var orig originals.Original
	db.First(&orig, "id = ?", trans.OriginalID)

	// create video record
	video := media.Video{OriginalID: orig.ID, Source: "transcode", Filename: dstFilename}

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
}

func videoToAudio(transID uint, kbps uint, videoFilepath string) {

	// determine destination path
	audioFilename := uuid.Must(uuid.NewV7()).String()
	audioFilename = fmt.Sprintf("%s.mp3", audioFilename)
	audioFilepath := filepath.Join(getDataDir(), audioFilename)

	// ensure destination directory
	err := ensureDirFor(audioFilepath)
	if err != nil {
		fmt.Println("Error: couldn't create dir for ", audioFilepath, err)
		db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "running")
	_, _, err = ffmpeg.Ffmpeg("-i", videoFilepath, "-vn", "-acodec",
		"mp3", "-b:a",
		fmt.Sprintf("%dk", kbps),
		audioFilepath)
	if err != nil {
		fmt.Println("Error: convert to audio file", videoFilepath, "->", audioFilepath)
		db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	// look up original
	var trans Transcode
	db.First(&trans, "id = ?", transID)
	var orig originals.Original
	db.First(&orig, "id = ?", trans.OriginalID)

	// create audio record
	audio := media.Audio{OriginalID: orig.ID,
		Filename: audioFilename,
		Bps:      kbps * 1000,
		Source:   "transcode",
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
}

func audioToAudio(transID uint, kbps uint, srcFilepath string) {

	// determine destination path
	dstFilename := uuid.Must(uuid.NewV7()).String()
	dstFilename = fmt.Sprintf("%s.mp3", dstFilename)
	dstFilepath := filepath.Join(getDataDir(), dstFilename)

	// ensure destination directory
	err := ensureDirFor(dstFilepath)
	if err != nil {
		fmt.Println("Error: couldn't create dir for ", dstFilepath, err)
		db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "running")
	_, _, err = ffmpeg.Ffmpeg("-i", srcFilepath, "-vn", "-acodec",
		"mp3", "-b:a",
		fmt.Sprintf("%dk", kbps),
		dstFilepath)
	if err != nil {
		fmt.Println("Error: convert to audio file", srcFilepath, "->", dstFilepath)
		db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	// look up original
	var trans Transcode
	db.First(&trans, "id = ?", transID)
	var orig originals.Original
	db.First(&orig, "id = ?", trans.OriginalID)

	// create audio record
	audio := media.Audio{
		OriginalID: orig.ID,
		Filename:   dstFilename,
		Bps:        kbps * 1000,
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
}

func transcodePending() {
	log.Traceln("transcodePending...")

	// any running jobs here got stuck or dead in the midde, so reset them
	db.Model(&Transcode{}).Where("status = ?", "running").Update("status", "pending")

	// loop until no more pending jobs
	for {

		var originalsToUpdate []uint

		// find any originals with a transcode job and mark them as transcoding
		db.Model(&originals.Original{}).
			Select("id").
			Where("id IN (?)",
				db.Model(&Transcode{}).
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
				db.Model(&Transcode{}).
					Select("original_id"),
			).
			Find(&originalsToUpdate)
		db.Model(&originals.Original{}).
			Where("id IN ? AND status = ?", originalsToUpdate, originals.StatusTranscoding).
			Update("status", originals.StatusCompleted)

		var trans Transcode
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
			srcFilepath := filepath.Join(getDataDir(), srcVideo.Filename)

			if trans.DstKind == "video" {
				videoToVideo(trans.ID, trans.Height, srcFilepath)
			} else if trans.DstKind == "audio" {
				videoToAudio(trans.ID, trans.Rate, srcFilepath)
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
			srcFilepath := filepath.Join(getDataDir(), srcAudio.Filename)
			audioToAudio(trans.ID, trans.Rate, srcFilepath)
		} else {
			fmt.Println("unexpected src kind for Transcode", trans)
			db.Delete(&trans)
		}
	}

}

func transcodeWorker() {
	transcodePending()
	ticker := time.NewTicker(10 * time.Second)
	for range ticker.C {
		transcodePending()
	}
}
