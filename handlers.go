package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

func registerHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "register.html", nil)
}

func registerPostHandler(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	err := CreateUser(db, username, password)

	if err != nil {
		return c.String(http.StatusInternalServerError, "Error creating user")
	}

	return c.Redirect(http.StatusSeeOther, "/login")
}

func loginHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "login.html", nil)
}

func homeHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "home.html", nil)
}

func loginPostHandler(c echo.Context) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	var user User
	if err := db.Where("username = ?", username).First(&user).Error; err != nil {
		return c.String(http.StatusUnauthorized, "Invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return c.String(http.StatusUnauthorized, "Invalid credentials")
	}

	session, err := store.Get(c.Request(), "session")
	if err != nil {
		return c.String(http.StatusInternalServerError, "Unable to retrieve session")
	}
	session.Values["user_id"] = user.ID
	err = session.Save(c.Request(), c.Response().Writer)

	if err != nil {
		return c.String(http.StatusInternalServerError, "Unable to save session")
	}

	session, _ = store.Get(c.Request(), "session")
	_, ok := session.Values["user_id"]
	if !ok {
		return c.String(http.StatusInternalServerError, "user_id was not saved as expected")
	}

	fmt.Println("loginPostHandler: redirect to /download")
	return c.Redirect(http.StatusSeeOther, "/download")
}

func logoutHandler(c echo.Context) error {
	session, _ := store.Get(c.Request(), "session")
	delete(session.Values, "user_id")
	session.Save(c.Request(), c.Response().Writer)
	return c.Redirect(http.StatusSeeOther, "/login")
}

func downloadHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "download.html", nil)
}

func downloadPostHandler(c echo.Context) error {
	url := c.FormValue("url")
	userID := c.Get("user_id").(uint)

	video := Video{URL: url, UserID: userID, Status: "pending"}
	db.Create(&video)

	go startDownload(video.ID, url)

	return c.Redirect(http.StatusSeeOther, "/videos")
}

type Meta struct {
	title string
	ext   string
}

func getMeta(url string) (Meta, error) {
	cmd := exec.Command("yt-dlp", "--simulate", "--print", "%(title)s.%(ext)s", url)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("getTitle error:", err)
		return Meta{}, err
	} else {

		isDot := func(r rune) bool {
			return r == '.'
		}

		fields := strings.FieldsFunc(strings.TrimSpace(stdout.String()), isDot)
		if len(fields) < 2 {
			return Meta{}, errors.New("couldn't parse ytdlp output")
		}

		return Meta{
			title: strings.Join(fields[:len(fields)-1], "."),
			ext:   fields[len(fields)-1],
		}, nil
	}
}

func startDownload(videoID uint, videoURL string) {
	db.Model(&Video{}).Where("id = ?", videoID).Update("status", "downloading")

	meta, err := getMeta(videoURL)
	if err != nil {
		db.Model(&Video{}).Where("id = ?", videoID).Update("status", "failed")
		return
	}
	fmt.Println("set video title:", meta.title)
	db.Model(&Video{}).Where("id = ?", videoID).Update("title", meta.title)

	videoFilename := fmt.Sprintf("%d-%s.%s", videoID, meta.title, meta.ext)
	videoFilepath := filepath.Join(getDownloadDir(), "video", videoFilename)
	cmd := exec.Command("yt-dlp", "-o", videoFilepath, videoURL)
	err = cmd.Run()
	if err != nil {
		db.Model(&Video{}).Where("id = ?", videoID).Update("status", "failed")
		return
	}

	audioFilename := fmt.Sprintf("%d-%s.mp3", videoID, meta.title)
	audioFilepath := filepath.Join(getDownloadDir(), "audio", audioFilename)
	audioDir := filepath.Dir(audioFilepath)
	fmt.Println("Create", audioDir)
	err = os.MkdirAll(audioDir, 0700)
	if err != nil {
		fmt.Println("Error: couldn't create", audioDir)
		db.Model(&Video{}).Where("id = ?", videoID).Update("status", "failed")
		return
	}
	ffmpeg := "ffmpeg"
	ffmpegArgs := []string{"-i", videoFilepath, "-vn", "-acodec",
		"mp3", "-b:a", "192k", audioFilepath}
	fmt.Println(ffmpeg, ffmpegArgs)
	cmd = exec.Command(ffmpeg, ffmpegArgs...)
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error: convert to audio file", videoFilepath, "->", audioFilepath)
		db.Model(&Video{}).Where("id = ?", videoID).Update("status", "failed")
		return
	}

	// FIXME: ensure expected files exist
	db.Model(&Video{}).Where("id = ?", videoID).Updates(map[string]interface{}{
		"video_filename": videoFilename,
		"audio_filename": audioFilename,
		"status":         "completed",
	})
}

func videosHandler(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	var videos []Video
	db.Where("user_id = ?", userID).Find(&videos)
	return c.Render(http.StatusOK, "videos.html", map[string]interface{}{"videos": videos})
}

func videoCancelHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var video Video
	if err := db.First(&video, id).Error; err != nil {
		return c.Redirect(http.StatusSeeOther, "/videos")
	}

	// Cancel the download (this is a simplified version, you might need to implement a more robust cancellation mechanism)
	video.Status = "cancelled"
	db.Save(&video)

	return c.Redirect(http.StatusSeeOther, "/videos")
}

func videoRestartHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var video Video
	if err := db.First(&video, id).Error; err != nil {
		return c.Redirect(http.StatusSeeOther, "/videos")
	}

	video.Status = "pending"
	db.Save(&video)
	go startDownload(uint(id), video.URL)

	return c.Redirect(http.StatusSeeOther, "/videos")
}

func videoDeleteHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var video Video
	if err := db.First(&video, id).Error; err != nil {
		return c.Redirect(http.StatusSeeOther, "/videos")
	}

	// Delete the file
	if video.VideoFilename != "" {
		os.Remove(filepath.Join(getDownloadDir(), "video", video.VideoFilename))
	}
	if video.AudioFilename != "" {
		os.Remove(filepath.Join(getDownloadDir(), "audio", video.AudioFilename))
	}

	// Delete from database
	db.Delete(&video)

	return c.Redirect(http.StatusSeeOther, "/videos")
}
