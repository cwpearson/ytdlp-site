package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"ytdlp-site/ffmpeg"
)

type FFProbeOutput struct {
	Streams []struct {
		CodecName string `json:"codec_name"`
	} `json:"streams"`
}

func getAudioFormat(filename string) (string, error) {
	output, _, err := ffmpeg.Ffprobe("-v", "quiet", "-print_format", "json", "-show_streams", filename)
	if err != nil {
		log.Errorln("ffprobe error:", err)
		return "", err
	}

	var ffprobeOutput FFProbeOutput
	err = json.Unmarshal(output, &ffprobeOutput)
	if err != nil {
		log.Errorln("failed to parse ffprobe output:", err)
		return "", err
	}

	numStreams := len(ffprobeOutput.Streams)
	if numStreams > 1 || numStreams <= 0 {
		log.Error(numStreams, "streams in ffprobe output", numStreams)
		return "", err
	}

	return ffprobeOutput.Streams[0].CodecName, nil
}

func getStreamBitrate(path string, stream int) (uint, error) {
	ffprobeArgs := []string{
		"-v", "quiet",
		"-select_streams", fmt.Sprintf("a:%d", stream),
		"-show_entries", "stream=bit_rate",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path}

	stdout, _, err := ffmpeg.Ffprobe(ffprobeArgs...)
	if err != nil {
		fmt.Println("ffprobe error:", err, string(stdout))
		return 0, err
	}
	bitrateStr := strings.TrimSpace(string(stdout))

	bitrate, err := strconv.ParseUint(bitrateStr, 10, 32)
	if err != nil {
		fmt.Println("parse bitrate error:", err)
		return 0, err
	}
	return uint(bitrate), nil
}

func getFormatBitrate(path string) (uint, error) {
	ffprobeArgs := []string{
		"-v", "quiet",
		"-show_entries", "format=bit_rate",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path}

	stdout, _, err := ffmpeg.Ffprobe(ffprobeArgs...)
	if err != nil {
		fmt.Println("ffprobe error:", err, string(stdout))
		return 0, err
	}
	bitrateStr := strings.TrimSpace(string(stdout))

	bitrate, err := strconv.ParseUint(bitrateStr, 10, 32)
	if err != nil {
		fmt.Println("parse bitrate error:", err)
		return 0, err
	}
	return uint(bitrate), nil
}
