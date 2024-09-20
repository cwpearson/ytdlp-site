package main

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type Footer struct {
	BuildDate    string
	BuildId      string
	BuildIdShort string
}

func makeFooter() Footer {
	return Footer{
		BuildDate:    getBuildDate(),
		BuildId:      getGitSHA(),
		BuildIdShort: getGitSHA()[0:7],
	}
}

var ytdlpAudioOptions = []string{"-f", "bestvideo[height<=1080]+bestaudio/best[height<=1080]"}
var ytdlpVideoOptions = []string{"-f", "bestaudio"}

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
	return c.Render(http.StatusOK, "home.html",
		map[string]interface{}{
			"Footer": makeFooter(),
		})
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
	return c.Render(http.StatusOK, "download.html",
		map[string]interface{}{
			"Footer": makeFooter(),
		})
}

func downloadPostHandler(c echo.Context) error {
	url := c.FormValue("url")
	userID := c.Get("user_id").(uint)
	vaStr := c.FormValue("color")

	audioOnly := false
	if vaStr == "audio" {
		audioOnly = true
	} else if vaStr == "audio-video" {
		audioOnly = false
	} else {
		return c.Redirect(http.StatusSeeOther, "/download")
	}

	original := Original{
		URL:    url,
		UserID: userID,
		Status: "pending",
		Audio:  audioOnly,
		Video:  !audioOnly,
	}
	db.Create(&original)
	go startDownload(original.ID, url, audioOnly)
	return c.Redirect(http.StatusSeeOther, "/videos")
}

type Meta struct {
	title  string
	artist string
	ext    string
}

func getYtdlpTitle(url string, args []string) (string, error) {
	ytdlp := "yt-dlp"
	args = append(args, "--simulate", "--print", "%(title)s", url)
	fmt.Println(ytdlp, strings.Join(args, " "))
	cmd := exec.Command(ytdlp, args...)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("getYtdlpTitle error:", err, stdout.String())
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func getYtdlpArtist(url string, args []string) (string, error) {
	ytdlp := "yt-dlp"
	args = append(args, "--simulate", "--print", "%(uploader)s", url)
	fmt.Println(ytdlp, strings.Join(args, " "))
	cmd := exec.Command(ytdlp, args...)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("getYtdlpArtist error:", err, stdout.String())
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func getYtdlpExt(url string, args []string) (string, error) {
	ytdlp := "yt-dlp"
	args = append(args, "--simulate", "--print", "%(ext)s", url)
	fmt.Println(ytdlp, strings.Join(args, " "))
	cmd := exec.Command(ytdlp, args...)

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("getYtdlpExt error:", err, stdout.String())
		return "", err
	}
	result := strings.TrimSpace(stdout.String())
	fmt.Println(result)
	return result, nil
}

func getYtdlpMeta(url string, args []string) (Meta, error) {

	meta := Meta{}
	var err error

	meta.title, err = getYtdlpTitle(url, args)
	if err != nil {

	}
	meta.artist, err = getYtdlpArtist(url, args)
	if err != nil {

	}
	meta.ext, err = getYtdlpExt(url, args)
	if err != nil {

	}

	return meta, nil
}

func getYtdlpAudioMeta(url string) (Meta, error) {
	return getYtdlpMeta(url, ytdlpVideoOptions)
}

func getYtdlpVideoMeta(url string) (Meta, error) {
	return getYtdlpMeta(url, ytdlpAudioOptions)
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
	length float64
}

type AudioMeta struct {
	rate uint
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
	length, err := getLength(path)
	if err != nil {
		return VideoMeta{}, err
	}
	return VideoMeta{
		width:  w,
		height: h,
		fps:    fps,
		length: length,
	}, nil
}

func getAudioDuration(path string) (float64, error) {

	ffprobe := "ffprobe"
	ffprobeArgs := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path}
	fmt.Println(ffprobe, strings.Join(ffprobeArgs, " "))
	cmd := exec.Command(ffprobe, ffprobeArgs...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		fmt.Println("getAudioDuration error:", err, stdout.String())
		return 0, err
	}
	durationStr := strings.TrimSpace(stdout.String())
	return strconv.ParseFloat(durationStr, 64)
}

func getAudioBitrate(path string) (uint, error) {
	codec, err := getAudioFormat(path)
	if err != nil {
		return 0, err
	}
	if codec == "opus" {
		return getFormatBitrate(path)
	} else {
		return getStreamBitrate(path, 0)
	}
}

func getAudioMeta(path string) (AudioMeta, error) {
	rate, err := getAudioBitrate(path)
	if err != nil {
		return AudioMeta{}, err
	}
	return AudioMeta{
		rate: rate,
	}, nil
}

func processOriginal(originalID uint) {

	// check if there is an original video
	hasOriginalVideo := true
	hasOriginalAudio := true
	var video Video
	var audio Audio
	err := db.Where("source = ?", "original").Where("original_id = ?", originalID).First(&video).Error
	if err == gorm.ErrRecordNotFound {
		hasOriginalVideo = false
	}
	err = db.Where("source = ?", "original").Where("original_id = ?", originalID).First(&audio).Error
	if err == gorm.ErrRecordNotFound {
		hasOriginalAudio = false
	}

	if hasOriginalVideo {

		videoFilepath := filepath.Join(getDataDir(), video.Filename)
		_, err := os.Stat(videoFilepath)
		if os.IsNotExist(err) {
			fmt.Println("Skipping non-existant file for processOriginal")
			return
		}
		videoMeta, err := getVideoMeta(videoFilepath)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(videoMeta)
			db.Model(&Video{}).Where("id = ?", video.ID).Update("fps", videoMeta.fps)
			db.Model(&Video{}).Where("id = ?", video.ID).Update("width", videoMeta.width)
			db.Model(&Video{}).Where("id = ?", video.ID).Update("height", videoMeta.height)
			db.Model(&Video{}).Where("id = ?", video.ID).Updates(map[string]interface{}{
				"fps":    videoMeta.fps,
				"width":  videoMeta.width,
				"height": videoMeta.height,
				"length": videoMeta.length,
			})
		}

		videoSize, err := getSize(videoFilepath)
		if err == nil {
			db.Model(&Video{}).Where("id = ?", video.ID).Update("size", videoSize)
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
		for _, targetHeight := range []uint{144, 480, 720, 1080} {
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

	} else if hasOriginalAudio {

		audioFilepath := filepath.Join(getDataDir(), audio.Filename)
		_, err := os.Stat(audioFilepath)
		if os.IsNotExist(err) {
			fmt.Println("Skipping non-existant audio file for processOriginal")
			return
		}
		audioMeta, err := getAudioMeta(audioFilepath)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(audioMeta)
			db.Model(&Audio{}).Where("id = ?", audio.ID).Update("bps", audioMeta.rate)
		}

		size, err := getSize(audioFilepath)
		if err == nil {
			db.Model(&Audio{}).Where("id = ?", audio.ID).Update("size", size)
		}

		// create audio transcodes
		for _, bitrate := range []uint{64, 96, 128, 160, 192} {
			t := Transcode{
				SrcID:      audio.ID,
				OriginalID: originalID,
				SrcKind:    "audio",
				DstKind:    "audio",
				Rate:       bitrate,
				TimeSubmit: time.Now(),
				Status:     "pending",
			}
			db.Create(&t)
		}

	} else {
		fmt.Println("No original video or audio found in processOriginal")
	}

}

func startDownload(originalID uint, videoURL string, audioOnly bool) {
	fmt.Println("startDownload audioOnly=", audioOnly)

	// metadata phase
	db.Model(&Original{}).Where("id = ?", originalID).Update("status", "metadata")
	var origMeta Meta
	var err error
	if audioOnly {
		origMeta, err = getYtdlpAudioMeta(videoURL)
	} else {
		origMeta, err = getYtdlpVideoMeta(videoURL)
	}
	if err != nil {
		db.Model(&Original{}).Where("id = ?", originalID).Update("status", "failed")
		return
	}
	fmt.Printf("original metadata %v\n", origMeta)
	err = db.Model(&Original{}).Where("id = ?", originalID).Updates(map[string]interface{}{
		"title":  origMeta.title,
		"artist": origMeta.artist,
	}).Error
	if err != nil {
		db.Model(&Original{}).Where("id = ?", originalID).Update("status", "failed")
		return
	}

	// download original
	db.Model(&Original{}).Where("id = ?", originalID).Update("status", "downloading")
	dlFilename := fmt.Sprintf("%d-%s.%s", originalID, origMeta.title, origMeta.ext)
	dlFilepath := filepath.Join(getDataDir(), dlFilename)

	var args []string
	if audioOnly {
		args = ytdlpVideoOptions
	} else {
		args = ytdlpAudioOptions
	}

	ytdlp := "yt-dlp"
	ytdlpArgs := append(args, "-o", dlFilepath, videoURL)

	fmt.Println(ytdlp, strings.Join(ytdlpArgs, " "))
	cmd := exec.Command(ytdlp, ytdlpArgs...)
	err = cmd.Run()
	if err != nil {
		db.Model(&Original{}).Where("id = ?", originalID).Update("status", "failed")
		return
	}

	length, _ := getLength(dlFilepath)
	size, _ := getSize(dlFilepath)

	if audioOnly {
		audio := Audio{
			OriginalID: originalID,
			Filename:   dlFilename,
			Source:     "original",
			Type:       origMeta.ext,
			Length:     length,
			Size:       size,
		}
		fmt.Println("create Audio", audio)
		if db.Create(&audio).Error != nil {
			fmt.Println("Couldn't create audio entry", err)
			db.Model(&Original{}).Where("id = ?", originalID).Update("status", "failed")
			return
		}
	} else {
		video := Video{
			OriginalID: originalID,
			Filename:   dlFilename,
			Source:     "original",
			Type:       origMeta.ext,
			Length:     length,
			Size:       size,
		}
		fmt.Println("create Video", video)
		if db.Create(&video).Error != nil {
			fmt.Println("Couldn't create video entry", err)
			db.Model(&Original{}).Where("id = ?", originalID).Update("status", "failed")
			return
		}
	}

	db.Model(&Original{}).Where("id = ?", originalID).Update("status", "completed")
	processOriginal(originalID)
}

func videosHandler(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	var origs []Original
	db.Where("user_id = ?", userID).Find(&origs)
	return c.Render(http.StatusOK, "videos.html",
		map[string]interface{}{
			"videos": origs,
			"Footer": makeFooter(),
		})
}

type VideoTemplate struct {
	Source           string
	Width            uint
	Height           uint
	FPS              string
	Size             string
	Filename         string
	DownloadFilename string
	StreamRate       string
	TempURL
}

type AudioTemplate struct {
	Kbps             string
	Size             string
	Filename         string
	DownloadFilename string
	StreamRate       string
	TempURL
}

func makeNiceFilename(input string) string {
	// Convert to lowercase
	input = strings.ToLower(input)

	// Replace spaces with underscores
	input = strings.ReplaceAll(input, " ", "_")

	// Replace common special characters
	replacements := map[string]string{
		"æ": "ae", "ø": "oe", "å": "aa", "ä": "ae", "ö": "oe", "ü": "ue",
		"ß": "ss", "ñ": "n", "ç": "c", "œ": "oe",
	}
	for old, new := range replacements {
		input = strings.ReplaceAll(input, old, new)
	}

	// Remove any remaining non-alphanumeric characters (except underscores)
	reg := regexp.MustCompile("[^a-z0-9_\\.]+")
	input = reg.ReplaceAllString(input, "")

	// Trim leading/trailing underscores
	input = strings.Trim(input, "_")

	// Collapse multiple underscores into a single one
	reg = regexp.MustCompile("_+")
	input = reg.ReplaceAllString(input, "_")

	// Ensure the filename is not empty
	if input == "" {
		input = "unnamed_file"
	}

	return input
}

func videoHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var orig Original
	if err := db.First(&orig, id).Error; err != nil {
		return c.Redirect(http.StatusSeeOther, "/videos")
	}

	var videos []Video
	db.Where("original_id = ?", id).
		Order("CASE WHEN source = 'original' THEN 0 ELSE 1 END, height ASC").
		Find(&videos)

	var audios []Audio
	db.Where("original_id = ?", id).
		Order("CASE WHEN source = 'original' THEN 0 ELSE 1 END, bps ASC").
		Find(&audios)

	dataDir := getDataDir()

	// create temporary URLs
	var videoURLs []VideoTemplate
	var audioURLs []AudioTemplate
	for _, video := range videos {
		tempURL, err := CreateTempURL(filepath.Join(dataDir, video.Filename))
		if err != nil {
			continue
		}

		rate := float64(video.Size) / video.Length

		videoURLs = append(videoURLs, VideoTemplate{
			Source:           video.Source,
			Width:            video.Width,
			Height:           video.Height,
			FPS:              fmt.Sprintf("%.1f", video.FPS),
			Size:             humanSize(video.Size),
			Filename:         video.Filename,
			DownloadFilename: makeNiceFilename(orig.Title),
			StreamRate:       fmt.Sprintf("%.1f KiB/s", rate/1024),
			TempURL:          tempURL,
		})
	}
	for _, audio := range audios {
		tempURL, err := CreateTempURL(filepath.Join(dataDir, audio.Filename))
		if err != nil {
			continue
		}

		kbps := float64(audio.Bps) / 1000
		rate := float64(audio.Size) / audio.Length

		audioURLs = append(audioURLs, AudioTemplate{
			Kbps:             fmt.Sprintf("%.1f kbps", kbps),
			Size:             humanSize(audio.Size),
			Filename:         audio.Filename,
			DownloadFilename: makeNiceFilename(orig.Title),
			StreamRate:       fmt.Sprintf("%.1f KiB/s", rate/1024),
			TempURL:          tempURL,
		})
	}

	return c.Render(http.StatusOK, "video.html",
		map[string]interface{}{
			"original": orig,
			"videos":   videoURLs,
			"audios":   audioURLs,
			"dataDir":  dataDir,
			"Footer":   makeFooter(),
		})
}

func videoRestartHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var orig Original
	if err := db.First(&orig, id).Error; err != nil {
		return c.Redirect(http.StatusSeeOther, "/videos")
	}

	orig.Status = "pending"
	db.Save(&orig)

	go startDownload(uint(id), orig.URL, orig.Audio)

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
