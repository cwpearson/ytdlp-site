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
	"time"

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

	original := Original{URL: url, UserID: userID, Status: "pending"}
	db.Create(&original)
	go startDownload(original.ID, url)

	return c.Redirect(http.StatusSeeOther, "/videos")
}

type Meta struct {
	title string
	ext   string
}

func getMeta(url string) (Meta, error) {
	ytdlp := "yt-dlp"
	args := []string{"--simulate", "--print", "%(title)s.%(ext)s", url}
	fmt.Println(ytdlp, strings.Join(args, " "))
	cmd := exec.Command(ytdlp, args...)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("getTitle error:", err, stdout.String())
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

// return the length in seconds of a video file at `path`
func getLength(path string) (float64, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("getLength cmd error:", err)
		return -1, err
	}

	result, err := strconv.ParseFloat(strings.TrimSpace(stdout.String()), 64)
	if err != nil {
		fmt.Println("getLength parse error:", err, stdout.String())
	}
	return result, nil
}

func getVideoWidth(path string) (uint, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams",
		"v:0", "-count_packets", "-show_entries",
		"stream=width", "-of", "csv=p=0", path)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("getVideoWidth cmd error:", err)
		return 0, err
	}

	result, err := strconv.ParseUint(strings.TrimSpace(stdout.String()), 10, 32)
	if err != nil {
		fmt.Println("getVideoWidth parse error:", err, stdout.String())
	}
	return uint(result), nil
}

func getVideoHeight(path string) (uint, error) {
	cmd := exec.Command("ffprobe", "-v", "error", "-select_streams",
		"v:0", "-count_packets", "-show_entries",
		"stream=height", "-of", "csv=p=0", path)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("getVideoHeight cmd error:", err)
		return 0, err
	}

	result, err := strconv.ParseUint(strings.TrimSpace(stdout.String()), 10, 32)
	if err != nil {
		fmt.Println("getVideoHeight parse error:", err, stdout.String())
	}
	return uint(result), nil
}

func getVideoFPS(path string) (float64, error) {

	ffprobe := "ffprobe"
	args := []string{"-v", "error", "-select_streams",
		"v:0", "-count_packets", "-show_entries",
		"stream=r_frame_rate", "-of", "csv=p=0", path}

	fmt.Println(ffprobe, strings.Join(args, " "))

	cmd := exec.Command(ffprobe, args...)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("getVideoFPS cmd error:", err)
		return -1, err
	}

	// TODO: this produces a string like "num/denom", do the division
	parts := strings.Split(strings.TrimSpace(stdout.String()), "/")
	if len(parts) != 2 {
		fmt.Println("getVideoFPS split error:", err, stdout.String())
	}

	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		fmt.Println("getVideoFPS numerator parse error:", err, stdout.String())
	}

	denom, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		fmt.Println("getVideoFPS denominator parse error:", err, stdout.String())
	}
	if denom == 0 {
		fmt.Println("getVideoFPS denominator is zero error:", stdout.String())
	}

	return num / denom, nil
}

func humanLength(s float64) string {
	ss := int64(s)
	mm, ss := ss/60, ss%60
	hh, mm := mm/60, mm%60

	return fmt.Sprintf("%d:%02d:%02d", hh, mm, ss)
}

func getSize(path string) (int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return -1, err
	}
	return fi.Size(), nil
}

func humanSize(bytes int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
		GiB = 1024 * MiB
	)

	if bytes >= GiB {
		return fmt.Sprintf("%.1f GiB", float64(bytes)/float64(GiB))
	} else if bytes >= MiB {
		return fmt.Sprintf("%.1f MiB", float64(bytes)/float64(MiB))
	} else if bytes >= KiB {
		return fmt.Sprintf("%.1f KiB", float64(bytes)/float64(KiB))
	}
	return fmt.Sprintf("%d bytes", bytes)
}

type VideoMeta struct {
	width  uint
	height uint
	fps    float64
}

func getVideoMeta(path string) (VideoMeta, error) {
	w, err := getVideoWidth(path)
	if err != nil {
		return VideoMeta{}, err
	}
	h, err := getVideoHeight(path)
	if err != nil {
		return VideoMeta{}, err
	}
	fps, err := getVideoFPS(path)
	if err != nil {
		return VideoMeta{}, err
	}
	return VideoMeta{
		width:  w,
		height: h,
		fps:    fps,
	}, nil
}

func processOriginal(originalID uint, videoFilename string, origMeta Meta) {

	videoFilepath := filepath.Join(getDataDir(), videoFilename)
	_, err := os.Stat(videoFilepath)
	if os.IsNotExist(err) {
		fmt.Println("Skipping non-existant file for processOriginal")
		return
	}

	// create video entry for original
	video := Video{
		OriginalID: originalID,
		Filename:   videoFilename,
		Source:     "original",
		Type:       origMeta.ext,
	}
	fmt.Println("create Video", video)
	if err := db.Create(&video).Error; err != nil {
		fmt.Println(err)
	}

	videoMeta, err := getVideoMeta(videoFilepath)
	if err != nil {
		fmt.Println(err)
	} else {
		fmt.Println(videoMeta)
		db.Model(&Video{}).Where("id = ?", video.ID).Update("fps", videoMeta.fps)
		db.Model(&Video{}).Where("id = ?", video.ID).Update("width", videoMeta.width)
		db.Model(&Video{}).Where("id = ?", video.ID).Update("height", videoMeta.height)
	}

	videoSize, err := getSize(videoFilepath)
	if err == nil {
		db.Model(&Video{}).Where("id = ?", video.ID).Update("size", humanSize(videoSize))
	}

	// create audio transcodes
	for _, bitrate := range []uint{64, 96, 128, 160, 192} {
		t := Transcode{
			SrcID:      video.ID,
			OriginalID: originalID,
			SrcKind:    "video",
			DstKind:    "audio",
			Rate:       bitrate,
			TimeSubmit: time.Now(),
			Status:     "pending",
		}
		db.Create(&t)
	}

	// create video transcodes
	for _, targetHeight := range []uint{144, 240, 360, 480, 720, 1080} {
		if targetHeight <= videoMeta.height {
			t := Transcode{
				SrcID:      video.ID,
				OriginalID: originalID,
				SrcKind:    "video",
				DstKind:    "video",
				Height:     targetHeight,
				TimeSubmit: time.Now(),
				Status:     "pending",
			}
			db.Create(&t)
		}
	}
}

func startDownload(originalID uint, videoURL string) {

	// metadata phase
	db.Model(&Original{}).Where("id = ?", originalID).Update("status", "metadata")
	origMeta, err := getMeta(videoURL)
	if err != nil {
		db.Model(&Original{}).Where("id = ?", originalID).Update("status", "failed")
		return
	}
	fmt.Printf("original metadata %v\n", origMeta)
	db.Model(&Original{}).Where("id = ?", originalID).Update("title", origMeta.title)

	// download original
	db.Model(&Original{}).Where("id = ?", originalID).Update("status", "downloading")
	videoFilename := fmt.Sprintf("%d-%s.%s", originalID, origMeta.title, origMeta.ext)
	videoFilepath := filepath.Join(getDataDir(), videoFilename)
	cmd := exec.Command("yt-dlp",
		"-f", "bestvideo+bestaudio/best",
		"-o", videoFilepath,
		videoURL)
	err = cmd.Run()
	if err != nil {
		db.Model(&Original{}).Where("id = ?", originalID).Update("status", "failed")
		return
	}
	db.Model(&Original{}).Where("id = ?", originalID).Update("status", "completed")

	processOriginal(originalID, videoFilename, origMeta)
}

func videosHandler(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	var origs []Original
	db.Where("user_id = ?", userID).Find(&origs)
	return c.Render(http.StatusOK, "videos.html", map[string]interface{}{"videos": origs})
}

type VideoTemplate struct {
	Video
	TempURL
}

type AudioTemplate struct {
	Audio
	TempURL
}

func videoHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var orig Original
	if err := db.First(&orig, id).Error; err != nil {
		return c.Redirect(http.StatusSeeOther, "/videos")
	}

	var videos []Video
	db.Where("original_id = ?", id).Find(&videos)

	var audios []Audio
	db.Where("original_id = ?", id).Find(&audios)

	dataDir := getDataDir()

	// create remporary URLs
	var videoURLs []VideoTemplate
	var audioURLs []AudioTemplate
	for _, video := range videos {
		tempURL, err := CreateTempURL(filepath.Join(dataDir, video.Filename))
		if err != nil {
			continue
		}
		videoURLs = append(videoURLs, VideoTemplate{video, tempURL})
	}
	for _, audio := range audios {
		tempURL, err := CreateTempURL(filepath.Join(dataDir, audio.Filename))
		if err != nil {
			continue
		}
		audioURLs = append(audioURLs, AudioTemplate{audio, tempURL})
	}

	return c.Render(http.StatusOK, "video.html",
		map[string]interface{}{
			"original": orig,
			"videos":   videoURLs,
			"audios":   audioURLs,
			"dataDir":  dataDir,
		})
}

func videoCancelHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var video Video
	if err := db.First(&video, id).Error; err != nil {
		return c.Redirect(http.StatusSeeOther, "/videos")
	}

	// Cancel the download (this is a simplified version, you might need to implement a more robust cancellation mechanism)

	return c.Redirect(http.StatusSeeOther, "/videos")
}

func videoRestartHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var orig Original
	if err := db.First(&orig, id).Error; err != nil {
		return c.Redirect(http.StatusSeeOther, "/videos")
	}

	orig.Status = "pending"
	db.Save(&orig)
	go startDownload(uint(id), orig.URL)

	return c.Redirect(http.StatusSeeOther, "/videos")
}

func deleteTranscodes(originalID int) {
	fmt.Println("Delete Transcode entries for Original", originalID)
	db.Delete(&Transcode{}, "original_id = ?", originalID)
}

func deleteTranscodedVideos(originalID int) {
	var videos []Video
	db.Where("original_id = ?", originalID).Where("source = ?", "transcoded").Find(&videos)
	for _, video := range videos {
		path := filepath.Join(getDataDir(), video.Filename)
		fmt.Println("remove", path)
		err := os.Remove(path)
		if err != nil {
			fmt.Println("error removing", path, err)
		}
	}
	db.Delete(&Video{}, "original_id = ?", originalID)
}

func deleteOriginalVideos(originalID int) {
	var videos []Video
	db.Where("original_id = ?", originalID).Where("source = ?", "original").Find(&videos)
	for _, video := range videos {
		path := filepath.Join(getDataDir(), video.Filename)
		fmt.Println("remove", path)
		err := os.Remove(path)
		if err != nil {
			fmt.Println("error removing", path, err)
		}
	}
	db.Delete(&Video{}, "original_id = ?", originalID)
}

func deleteAudios(originalID int) {
	// delete audios
	var audios []Audio
	db.Where("original_id = ?", originalID).Find(&audios)
	for _, audio := range audios {
		path := filepath.Join(getDataDir(), audio.Filename)
		fmt.Println("remove", path)
		err := os.Remove(path)
		if err != nil {
			fmt.Println("error removing", path, err)
		}
	}
	db.Delete(&Video{}, "original_id = ?", originalID)
}

func videoDeleteHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var orig Original
	if err := db.First(&orig, id).Error; err != nil {
		return c.Redirect(http.StatusSeeOther, "/videos")
	}

	deleteTranscodes(id)
	deleteTranscodedVideos(id)
	deleteOriginalVideos(id)
	deleteAudios(id)

	db.Delete(&orig)
	return c.Redirect(http.StatusSeeOther, "/videos")
}

func tempHandler(c echo.Context) error {
	token := c.Param("token")

	var tempURL TempURL
	if err := db.Where("token = ? AND expires_at > ?", token, time.Now()).First(&tempURL).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Invalid or expired token"})
	}

	return c.File(tempURL.FilePath)
}

// func processHandler(c echo.Context) error {
// 	id, _ := strconv.Atoi(c.Param("id"))

// 	deleteTranscodes(id)
// 	deleteAudios(id)
// 	deleteTranscodedVideos(id)

// 	processOriginal(id)
// }
