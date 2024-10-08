package main

import (
	"bytes"
	"os/exec"
	"strings"
)

// runs ffprobe with the provided args and returns (stdout, stderr, error)
func runYtdlp(args ...string) ([]byte, []byte, error) {
	ytdlp := "yt-dlp"
	log.Infoln(ytdlp, strings.Join(args, " "))
	cmd := exec.Command(ytdlp, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		log.Errorf("yt-dlp error: %v", err)
	}
	log.Infoln("stdout:", stdout.String())
	log.Infoln("stderr:", stderr.String())
	return stdout.Bytes(), stderr.Bytes(), err
}
