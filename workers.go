package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func ensureDirFor(path string) error {
	dir := filepath.Dir(path)
	fmt.Println("Create", dir)
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
	} else if height <= 240 {
		audioBitrate = 96
	} else if height < 720 {
		audioBitrate = 128
	}

	// start ffmpeg
	ffmpeg := "ffmpeg"
	ffmpegArgs := []string{"-i", srcFilepath,
		"-vf", fmt.Sprintf("scale=-2:%d", height), "-c:v", "libx264",
		"-crf", "23", "-preset", "veryfast", "-c:a", "aac", "-b:a", fmt.Sprintf("%dk", audioBitrate),
		dstFilepath}
	fmt.Println(ffmpeg, strings.Join(ffmpegArgs, " "))
	cmd := exec.Command(ffmpeg, ffmpegArgs...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "running")
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error: convert to video file", srcFilepath, "->", dstFilepath, stdout.String(), stderr.String())
		return
	}

	// look up original
	var trans Transcode
	db.First(&trans, transID)
	var orig Original
	db.First(&orig, "id = ?", trans.OriginalID)

	// create video record
	video := Video{OriginalID: orig.ID, Source: "transcode", Filename: dstFilename}

	fileSize, err := getSize(dstFilepath)
	if err == nil {
		video.Size = humanSize(fileSize)
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

func videoToAudio(transID uint, bitrate uint, videoFilepath string) {

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

	ffmpeg := "ffmpeg"
	ffmpegArgs := []string{"-i", videoFilepath, "-vn", "-acodec",
		"mp3", "-b:a",
		fmt.Sprintf("%dk", bitrate),
		audioFilepath}
	fmt.Println(ffmpeg, strings.Join(ffmpegArgs, " "))
	cmd := exec.Command(ffmpeg, ffmpegArgs...)
	db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "running")
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error: convert to audio file", videoFilepath, "->", audioFilepath)
		db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	// look up original
	var trans Transcode
	db.First(&trans, "id = ?", transID)
	var orig Original
	db.First(&orig, "id = ?", trans.OriginalID)

	// create audio record
	audio := Audio{OriginalID: orig.ID, Filename: audioFilename, Kbps: fmt.Sprintf("%dk", bitrate)}

	fileSize, err := getSize(audioFilepath)
	if err == nil {
		audio.Size = humanSize(fileSize)
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

	ffmpeg := "ffmpeg"
	ffmpegArgs := []string{"-i", srcFilepath, "-vn", "-acodec",
		"mp3", "-b:a",
		fmt.Sprintf("%dk", kbps),
		dstFilepath}
	fmt.Println(ffmpeg, strings.Join(ffmpegArgs, " "))
	cmd := exec.Command(ffmpeg, ffmpegArgs...)
	db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "running")
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error: convert to audio file", srcFilepath, "->", dstFilepath)
		db.Model(&Transcode{}).Where("id = ?", transID).Update("status", "failed")
		return
	}

	// look up original
	var trans Transcode
	db.First(&trans, "id = ?", transID)
	var orig Original
	db.First(&orig, "id = ?", trans.OriginalID)

	// create audio record
	audio := Audio{
		OriginalID: orig.ID,
		Filename:   dstFilename,
		Kbps:       fmt.Sprintf("%dk", kbps),
	}

	fileSize, err := getSize(dstFilepath)
	if err == nil {
		audio.Size = humanSize(fileSize)
	}

	db.Create(&audio)

	// complete transcode
	db.Delete(&trans)
}

func transcodePending() {
	fmt.Println("transcodePending...")

	// any running jobs here got stuck or dead in the midde, so reset them
	db.Model(&Transcode{}).Where("status = ?", "running").Update("status", "pending")

	// loop until no more pending jobs
	for {
		var trans Transcode
		err := db.First(&trans, "status = ?", "pending").Error
		if err == gorm.ErrRecordNotFound {
			fmt.Println("no pending transcode jobs")
			break // no more pending jobs
		}

		if trans.SrcKind == "video" {

			var srcVideo Video
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

			var srcAudio Audio
			err = db.First(&srcAudio, "id = ?", trans.SrcID).Error
			if err != nil {
				fmt.Println("no such source audio for audio Transcode", trans)
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