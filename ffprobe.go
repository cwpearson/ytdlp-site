package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type FFProbeOutput struct {
	Streams []struct {
		CodecName string `json:"codec_name"`
	} `json:"streams"`
}

// runs ffprobe with the provided args and returns (stdout, stderr, error)
func runFfprobe(args ...string) ([]byte, []byte, error) {
	ffprobe := "ffprobe"
	log.Infoln(ffprobe, strings.Join(args, " "))
	cmd := exec.Command(ffprobe, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		log.Errorf("ffprobe error: %v", err)
	}
	log.Infoln("stdout:", stdout.String())
	log.Infoln("stderr:", stderr.String())
	return stdout.Bytes(), stderr.Bytes(), err
}

func getAudioFormat(filename string) (string, error) {
	output, _, err := runFfprobe("-v", "quiet", "-print_format", "json", "-show_streams", filename)
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

	stdout, _, err := runFfprobe(ffprobeArgs...)
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

	stdout, _, err := runFfprobe(ffprobeArgs...)
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
