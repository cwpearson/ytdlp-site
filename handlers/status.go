package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"ytdlp-site/ffmpeg"
	"ytdlp-site/ytdlp"
)

func StatusGet(c echo.Context) error {

	ytdlpStdout, _, err := ytdlp.Run("--version")
	if err != nil {
		log.Errorln(err)
	}
	ffmpegStdout, _, err := ffmpeg.Ffmpeg("-version")
	if err != nil {
		log.Errorln(err)
	}

	return c.Render(http.StatusOK, "status.html", map[string]interface{}{
		"ytdlp":  string(ytdlpStdout),
		"ffmpeg": string(ffmpegStdout),
		"Footer": MakeFooter(),
	})
}
