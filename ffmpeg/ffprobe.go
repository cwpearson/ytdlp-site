package ffmpeg

import (
	"bytes"
	"os/exec"
	"strings"
)

// runs ffprobe with the provided args and returns (stdout, stderr, error)
func Ffprobe(args ...string) ([]byte, []byte, error) {
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
