package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"ytdlp-site/media"

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

func homeHandler(c echo.Context) error {

	// redirect to /videos if logged in
	session, err := store.Get(c.Request(), "session")
	if err == nil {
		_, ok := session.Values["user_id"]
		if ok {
			fmt.Println("homeHandler: session contains user_id. Redirect to /video")
			return c.Redirect(http.StatusSeeOther, "/videos")
		}
	}

	return c.Render(http.StatusOK, "home.html",
		map[string]interface{}{
			"Footer": makeFooter(),
		})
}

func loginHandler(c echo.Context) error {
	return c.Render(http.StatusOK, "login.html", nil)
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
		Status: Pending,
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
}

func getYtdlpTitle(url string, args []string) (string, error) {
	args = append(args, "--simulate", "--print", "%(title)s", url)
	stdout, _, err := runYtdlp(args...)
	if err != nil {
		log.Errorln(err)
		return "", err
	}
	return strings.TrimSpace(string(stdout)), nil
}

func getYtdlpArtist(url string, args []string) (string, error) {
	args = append(args, "--simulate", "--print", "%(uploader)s", url)
	stdout, _, err := runYtdlp(args...)
	if err != nil {
		log.Errorln(err)
		return "", err
	}
	return strings.TrimSpace(string(stdout)), nil
}

func getYtdlpExt(url string, args []string) (string, error) {
	args = append(args, "--simulate", "--print", "%(ext)s", url)
	stdout, _, err := runYtdlp(args...)
	if err != nil {
		log.Errorln(err)
		return "", err
	}
	return strings.TrimSpace(string(stdout)), nil
}

func getYtdlpMeta(url string, args []string) (Meta, error) {

	meta := Meta{}
	var err error

	meta.title, err = getYtdlpTitle(url, args)
	if err != nil {
		return meta, err
	}
	meta.artist, err = getYtdlpArtist(url, args)
	if err != nil {
		return meta, err
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
	stdout, _, err := runFfprobe("-v", "error", "-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1", path)
	if err != nil {
		log.Errorln("ffprobe error:", err)
		return -1, err
	}

	result, err := strconv.ParseFloat(strings.TrimSpace(string(stdout)), 64)
	if err != nil {
		log.Errorln("parse error:", err, string(stdout))
	}
	return result, nil
}

func getVideoWidth(path string) (uint, error) {
	stdout, _, err := runFfprobe("-v", "error", "-select_streams",
		"v:0", "-count_packets", "-show_entries",
		"stream=width", "-of", "csv=p=0", path)

	if err != nil {
		log.Errorln("ffprobe error", err)
		return 0, err
	}

	result, err := strconv.ParseUint(strings.TrimSpace(string(stdout)), 10, 32)
	if err != nil {
		log.Errorln("parse width error:", err, string(stdout))
	}
	return uint(result), nil
}

func getVideoHeight(path string) (uint, error) {
	stdout, _, err := runFfprobe("-v", "error", "-select_streams",
		"v:0", "-count_packets", "-show_entries",
		"stream=height", "-of", "csv=p=0", path)

	if err != nil {
		log.Errorln("ffprobe error:", err)
		return 0, err
	}

	result, err := strconv.ParseUint(strings.TrimSpace(string(stdout)), 10, 32)
	if err != nil {
		log.Errorln("getVideoHeight parse error:", err, string(stdout))
	}
	return uint(result), nil
}

func getVideoFPS(path string) (float64, error) {

	stdout, _, err := runFfprobe("-v", "error", "-select_streams",
		"v:0", "-count_packets", "-show_entries",
		"stream=r_frame_rate", "-of", "csv=p=0", path)
	if err != nil {
		log.Errorln("ffprobe error:", err)
		return -1, err
	}

	stdoutStr := string(stdout)
	parts := strings.Split(strings.TrimSpace(stdoutStr), "/")
	if len(parts) != 2 {
		log.Errorln("output format error", err, stdoutStr)
		return 0, err
	}

	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		log.Errorln("numerator parse error:", err, stdoutStr)
		return 0, err
	}

	denom, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		log.Errorln("denominator parse error:", err, stdoutStr)
		return 0, err
	}
	if denom == 0 {
		log.Errorln("denominator is zero error:", stdoutStr)
		return 0, err
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
	size   int64 // file size
}

type AudioMeta struct {
	rate   uint
	length float64
	size   int64 // file size
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
	size, err := getSize(path)
	if err != nil {
		return VideoMeta{}, err
	}
	return VideoMeta{
		width:  w,
		height: h,
		fps:    fps,
		length: length,
		size:   size,
	}, nil
}

func getAudioDuration(path string) (float64, error) {

	stdout, _, err := runFfprobe("-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		path)
	if err != nil {
		log.Errorln("ffprobe error:", err)
		return 0, err
	}
	durationStr := strings.TrimSpace(string(stdout))
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
	length, err := getLength(path)
	if err != nil {
		return AudioMeta{}, err
	}
	size, err := getSize(path)
	if err != nil {
		return AudioMeta{}, err
	}
	return AudioMeta{
		rate:   rate,
		length: length,
		size:   size,
	}, nil
}

func addAudioTranscode(mediaId, originalId, bitrate uint, srcKind string) {
	t := Transcode{
		SrcID:      mediaId,
		OriginalID: originalId,
		SrcKind:    srcKind,
		DstKind:    "audio",
		Rate:       bitrate,
		TimeSubmit: time.Now(),
		Status:     "pending",
	}
	db.Create(&t)
}

func addVideoTranscode(videoId, originalId, targetHeight uint) {
	t := Transcode{
		SrcID:      videoId,
		OriginalID: originalId,
		SrcKind:    "video",
		DstKind:    "video",
		Height:     targetHeight,
		TimeSubmit: time.Now(),
		Status:     "pending",
	}
	db.Create(&t)
}

func processOriginal(originalID uint) {

	// check if there is an original video
	hasOriginalVideo := true
	hasOriginalAudio := true
	var video media.Video
	var audio media.Audio
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

		// create audio transcodes
		for _, bitrate := range []uint{64 /*, 96, 128, 160, 192*/} {
			addAudioTranscode(video.ID, originalID, bitrate, "video")
		}

		// create video transcodes
		for _, targetHeight := range []uint{480, 240, 144} {
			if targetHeight <= video.Height {
				addVideoTranscode(video.ID, originalID, targetHeight)
				break
			}
		}

	} else if hasOriginalAudio {

		audioFilepath := filepath.Join(getDataDir(), audio.Filename)
		_, err := os.Stat(audioFilepath)
		if os.IsNotExist(err) {
			fmt.Println("Skipping non-existant audio file for processOriginal")
			return
		}

		// create audio transcodes
		for _, bitrate := range []uint{64 /*, 96, 128, 160, 192*/} {
			addAudioTranscode(video.ID, originalID, bitrate, "audio")
		}

	} else {
		log.Errorf("No original video or audio for original %d found in processOriginal", originalID)
	}

}

func startDownload(originalID uint, videoURL string, audioOnly bool) {
	log.Debugf("startDownload audioOnly=%t", audioOnly)

	// metadata phase
	SetOriginalStatus(originalID, Metadata)
	var origMeta Meta
	var err error
	if audioOnly {
		origMeta, err = getYtdlpAudioMeta(videoURL)
	} else {
		origMeta, err = getYtdlpVideoMeta(videoURL)
	}
	if err != nil {
		log.Errorln("couldn't retrieve metadata:", err)
		SetOriginalStatus(originalID, Failed)
		return
	}
	log.Debugf("original metadata %v", origMeta)
	err = db.Model(&Original{}).Where("id = ?", originalID).Updates(map[string]interface{}{
		"title":  origMeta.title,
		"artist": origMeta.artist,
	}).Error
	if err != nil {
		log.Errorln("couldn't store metadata:", err)
		SetOriginalStatus(originalID, Failed)
		return
	}

	// download original
	SetOriginalStatus(originalID, Downloading)

	// create temporary directory
	tempDir, err := os.MkdirTemp("", "dl")
	if err != nil {
		log.Errorln("Error creating temporary directory:", err)
		SetOriginalStatus(originalID, Failed)
		return
	}
	defer os.RemoveAll(tempDir)
	log.Debugln("created", tempDir)

	// download into temporary directory
	var args []string
	if audioOnly {
		args = ytdlpVideoOptions
	} else {
		args = ytdlpAudioOptions
	}
	ytdlp := "yt-dlp"
	ytdlpArgs := append(args, videoURL)
	fmt.Println(ytdlp, strings.Join(ytdlpArgs, " "))
	cmd := exec.Command(ytdlp, ytdlpArgs...)
	cmd.Dir = tempDir
	err = cmd.Run()
	if err != nil {
		log.Errorln("yt-dlp failed")
		SetOriginalStatus(originalID, Failed)
		return
	}

	// discover name of downloaded file
	dirEnts, err := os.ReadDir(tempDir)
	if err != nil {
		log.Errorln("Error reading directory:", err)
		SetOriginalStatus(originalID, Failed)
		return
	}
	dlFilename := ""
	for _, dirEnt := range dirEnts {
		if !dirEnt.IsDir() {
			dlFilename = dirEnt.Name()
			break
		}
	}
	if dlFilename == "" {
		log.Errorln("couldn't find a downloaded file")
		SetOriginalStatus(originalID, Failed)
	}

	// move to data directory
	dlFilepath := filepath.Join(getDataDir(), dlFilename)
	err = os.Rename(filepath.Join(tempDir, dlFilename), dlFilepath)
	if err != nil {
		log.Errorln("couldn't move downloaded file")
		SetOriginalStatus(originalID, Failed)
	}

	if audioOnly {
		mediaMeta, err := getAudioMeta(dlFilepath)
		if err != nil {
			log.Errorln("couldn't get audio file metadata", err)
			SetOriginalStatus(originalID, Failed)
			return
		}

		audio := media.Audio{
			OriginalID: originalID,
			Filename:   dlFilename,
			Source:     "original",
			Length:     mediaMeta.length,
			Size:       mediaMeta.size,
		}
		fmt.Println("create Audio", audio)
		if db.Create(&audio).Error != nil {
			fmt.Println("Couldn't create audio entry", err)
			SetOriginalStatus(originalID, Failed)
			return
		}
	} else {
		mediaMeta, err := getVideoMeta(dlFilepath)
		if err != nil {
			log.Errorln("couldn't get video file metadata", err)
			SetOriginalStatus(originalID, Failed)
			return
		}

		video := media.Video{
			OriginalID: originalID,
			Filename:   dlFilename,
			Source:     "original",
			FPS:        mediaMeta.fps,
			Width:      mediaMeta.width,
			Height:     mediaMeta.height,
			Length:     mediaMeta.length,
			Size:       mediaMeta.size,
		}
		log.Debugln("create Video", video)
		if db.Create(&video).Error != nil {
			log.Errorln("Couldn't create video entry", err)
			SetOriginalStatus(originalID, Failed)
			return
		}
	}

	SetOriginalStatus(originalID, DownloadCompleted)
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
	ID               uint // Video.ID
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
	ID               uint // Audio.ID
	Source           string
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

	var videos []media.Video
	db.Where("original_id = ?", id).
		Order("CASE WHEN source = 'original' THEN 0 ELSE 1 END, height ASC").
		Find(&videos)

	var audios []media.Audio
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
			ID:               video.ID,
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
			ID:               audio.ID,
			Source:           audio.Source,
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

	// FIXME: rewrite this as an update
	var orig Original
	if err := db.First(&orig, id).Error; err != nil {
		return c.Redirect(http.StatusSeeOther, "/videos")
	}
	orig.Status = Pending
	db.Save(&orig)

	go startDownload(uint(id), orig.URL, orig.Audio)

	return c.Redirect(http.StatusSeeOther, "/videos")
}

func deleteTranscodes(originalID int) {
	log.Debugln("Delete Transcode entries for Original", originalID)
	db.Delete(&Transcode{}, "original_id = ?", originalID)
}

func deleteTranscodedVideos(originalID int) {
	var videos []media.Video
	db.Where("original_id = ?", originalID).Where("source = ?", "transcode").Find(&videos)
	for _, video := range videos {
		path := filepath.Join(getDataDir(), video.Filename)
		log.Debugln("remove video", path)
		err := os.Remove(path)
		if err != nil {
			log.Errorln("error removing", path, err)
		}
	}
	db.Delete(&media.Video{}, "original_id = ? AND source = ?", originalID, "transcode")
}

func deleteOriginalVideos(originalID int) {
	var videos []media.Video
	db.Where("original_id = ?", originalID).Where("source = ?", "original").Find(&videos)
	for _, video := range videos {
		path := filepath.Join(getDataDir(), video.Filename)
		fmt.Println("remove", path)
		err := os.Remove(path)
		if err != nil {
			fmt.Println("error removing", path, err)
		}
	}
	db.Delete(&media.Video{}, "original_id = ? AND source = ?", originalID, "original")
}

func deleteAudiosWithSource(originalID int, source string) {
	var audios []media.Audio
	db.Where("original_id = ?", originalID).Where("source = ?", source).Find(&audios)
	for _, audio := range audios {
		path := filepath.Join(getDataDir(), audio.Filename)
		log.Debugln("remove audio", path)
		err := os.Remove(path)
		if err != nil {
			log.Errorln("error removing", path, err)
		}
	}
	db.Delete(&media.Audio{}, "original_id = ? AND source = ?", originalID, source)
}

func deleteOriginalHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	var orig Original
	if err := db.First(&orig, id).Error; err != nil {
		return c.Redirect(http.StatusSeeOther, "/videos")
	}

	deleteTranscodes(id)
	deleteTranscodedVideos(id)
	deleteOriginalVideos(id)
	deleteAudiosWithSource(id, "original")
	deleteAudiosWithSource(id, "transcode")

	db.Delete(&orig)
	return c.Redirect(http.StatusSeeOther, "/videos")
}

func deleteVideoHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	referrer := c.Request().Referer()
	if referrer == "" {
		referrer = "/"
	}

	var video media.Video
	result := db.First(&video, id)
	if result.Error != nil {
		log.Errorln("error retrieving video", id, result.Error)
		return c.Redirect(http.StatusSeeOther, referrer)
	}

	videoPath := filepath.Join(getDataDir(), video.Filename)
	log.Debugln("remove", videoPath)
	err := os.Remove(videoPath)
	if err != nil {
		log.Errorln("coudn't remove", videoPath, err)
	}

	if err := db.Delete(&media.Video{}, id).Error; err != nil {
		log.Errorln("error deleting video record", id, err)
	}
	return c.Redirect(http.StatusSeeOther, referrer)
}

func deleteAudioHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id"))
	referrer := c.Request().Referer()
	if referrer == "" {
		referrer = "/"
	}

	var audio media.Audio
	result := db.First(&audio, id)
	if result.Error != nil {
		log.Errorln("error retrieving audio", id, result.Error)
		return c.Redirect(http.StatusSeeOther, referrer)
	}

	filePath := filepath.Join(getDataDir(), audio.Filename)
	log.Debugln("remove", filePath)
	err := os.Remove(filePath)
	if err != nil {
		log.Errorln("coudn't remove", filePath, err)
	}

	if err := db.Delete(&media.Audio{}, id).Error; err != nil {
		log.Errorln("error deleting audio record", id, err)
	}
	return c.Redirect(http.StatusSeeOther, referrer)
}

func transcodeToVideoHandler(c echo.Context) error {
	originalId, _ := strconv.ParseUint(c.FormValue("original_id"), 10, 32)
	height, _ := strconv.ParseUint(c.FormValue("height"), 10, 32)
	referrer := c.Request().Referer()
	if referrer == "" {
		referrer = "/"
	}

	var video media.Video
	err := db.Where("source = ?", "original").Where("original_id = ?", originalId).First(&video).Error
	if err == gorm.ErrRecordNotFound {
		log.Errorf("no video record for original %d: %v", originalId, err)
	} else {
		addVideoTranscode(video.ID, uint(originalId), uint(height))
	}

	return c.Redirect(http.StatusSeeOther, referrer)
}

func transcodeToAudioHandler(c echo.Context) error {
	originalId, _ := strconv.ParseUint(c.FormValue("original_id"), 10, 32)
	kbps, _ := strconv.ParseUint(c.FormValue("kbps"), 10, 32)
	referrer := c.Request().Referer()
	if referrer == "" {
		referrer = "/"
	}

	// check if there is an original video
	hasOriginalVideo := true
	hasOriginalAudio := true
	var video media.Video
	var audio media.Audio
	err := db.Where("source = ?", "original").Where("original_id = ?", originalId).First(&video).Error
	if err == gorm.ErrRecordNotFound {
		hasOriginalVideo = false
	}
	err = db.Where("source = ?", "original").Where("original_id = ?", originalId).First(&audio).Error
	if err == gorm.ErrRecordNotFound {
		hasOriginalAudio = false
	}

	if hasOriginalVideo {
		addAudioTranscode(video.ID, uint(originalId), uint(kbps), "video")
	} else if hasOriginalAudio {
		addAudioTranscode(audio.ID, uint(originalId), uint(kbps), "audio")
	} else {
		log.Errorln("no audio or video record for original", originalId)
	}

	return c.Redirect(http.StatusSeeOther, referrer)
}

func tempHandler(c echo.Context) error {
	token := c.Param("token")

	var tempURL TempURL
	if err := db.Where("token = ? AND expires_at > ?", token, time.Now()).First(&tempURL).Error; err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "Invalid or expired token"})
	}

	return c.File(tempURL.FilePath)
}

func processHandler(c echo.Context) error {
	id, _ := strconv.Atoi(c.Param("id")) // FIXME: strconv.ParseUint?

	deleteTranscodes(id)
	deleteAudiosWithSource(id, "transcode")
	deleteTranscodedVideos(id)

	err := SetOriginalStatus(uint(id), DownloadCompleted)
	if err != nil {
		log.Errorf("error while setting original %d status: %v", id, err)
	}

	processOriginal(uint(id))

	return c.Redirect(http.StatusSeeOther, "/videos")
}
