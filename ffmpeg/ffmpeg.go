package ffmpeg

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func Clip(src, dst string, from, to float64) error {
	_, _, err := Ffmpeg("-i", src,
		"-ss", fmt.Sprintf("%f", from),
		"-to", fmt.Sprintf("%f", to),
		"-c", "copy",
		dst)
	return err
}

// runs ffprobe with the provided args and returns (stdout, stderr, error)
func Ffmpeg(args ...string) ([]byte, []byte, error) {
	ffmpeg := "ffmpeg"
	log.Infoln(ffmpeg, strings.Join(args, " "))
	cmd := exec.Command(ffmpeg, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	if err != nil {
		log.Errorf("ffmpeg error: %v", err)
	}
	log.Infoln("stdout:", stdout.String())
	log.Infoln("stderr:", stderr.String())
	return stdout.Bytes(), stderr.Bytes(), err
}
