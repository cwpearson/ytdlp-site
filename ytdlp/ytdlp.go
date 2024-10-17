package ytdlp

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
)

// runs ffprobe with the provided args and returns (stdout, stderr, error)
func Run(args ...string) ([]byte, []byte, error) {
	cmd, cancel, err := Start(args...)
	defer cancel()
	if err != nil {
		return nil, nil, err
	}
	return cmd.Wait()
}

type Cmd struct {
	ctx context.Context
	cmd *exec.Cmd

	stdout *bytes.Buffer
	stderr *bytes.Buffer
}

func Start(args ...string) (*Cmd, context.CancelFunc, error) {

	ytdlp := "yt-dlp"
	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, ytdlp, args...)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	log.Infoln(ytdlp, strings.Join(args, " "))
	err := cmd.Start()
	if err != nil {
		return nil, cancel, err // FIXME: okay to just return this cancel thing?
	}

	return &Cmd{
		ctx:    ctx,
		cmd:    cmd,
		stdout: stdout,
		stderr: stderr,
	}, cancel, nil
}

func (c *Cmd) Wait() ([]byte, []byte, error) {
	err := c.cmd.Wait()
	if err != nil {
		if c.ctx.Err() == context.Canceled {
			log.Debugln("command canceled")
		} else {
			log.Errorln("yt-dlp error", err)
		}
	} else {
		log.Infoln("stdout:", c.stdout.String())
		log.Infoln("stderr:", c.stderr.String())
	}

	return c.stdout.Bytes(), c.stderr.Bytes(), err
}
