package main

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type OriginalStatus string

const (
	StatusNotStarted  OriginalStatus = "not started"
	Pending           OriginalStatus = "pending"
	Metadata          OriginalStatus = "metadata"
	Downloading       OriginalStatus = "downloading"
	DownloadCompleted OriginalStatus = "download completed"
	Transcoding       OriginalStatus = "transcoding"
	Completed         OriginalStatus = "completed"
	Failed            OriginalStatus = "failed"
)

type Original struct {
	gorm.Model
	UserID  uint
	URL     string
	Title   string
	Artist  string
	Status  OriginalStatus
	Audio   bool // video download requested
	Video   bool // audio download requested
	Watched bool

	Playlist   bool // part of a playlist
	PlaylistID uint // Playlist.ID (if part of a playlist)
}

type Transcode struct {
	gorm.Model
	Status     string // "pending", "running", "failed"
	SrcID      uint   // Video.ID or Audio.ID of the source file
	OriginalID uint   // Original.ID
	SrcKind    string // "video", "audio"
	DstKind    string // "video", "audio"
	TimeSubmit time.Time
	TimeStart  time.Time

	// video fields
	Height uint // target height
	Width  uint // target width
	FPS    uint // target FPS

	// audio & video fields
	Rate uint
}

type Playlist struct {
	gorm.Model
	UserID uint
	URL    string
	Title  string
	Status OriginalStatus
	Audio  bool
	Video  bool
}

type User struct {
	gorm.Model
	Username string `gorm:"unique"`
	Password string
}

type TempURL struct {
	Token     string `gorm:"uniqueIndex"`
	FilePath  string
	ExpiresAt time.Time
}

type DownloadStatus struct {
	ID        uint
	Progress  float64
	Status    string
	Error     string
	StartTime time.Time
}

type DownloadManager struct {
	downloads map[uint]*DownloadStatus
	mutex     sync.RWMutex
}

func CreateUser(db *gorm.DB, username, password string) error {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := User{Username: username, Password: string(hashedPassword)}
	if err := db.Create(&user).Error; err != nil {
		return err
	}
	return nil
}

func SetOriginalStatus(id uint, status OriginalStatus) error {
	return db.Model(&Original{}).Where("id = ?", id).Update("status", status).Error
}

func SetPlaylistStatus(id uint, status OriginalStatus) error {
	return db.Model(&Playlist{}).Where("id = ?", id).Update("status", status).Error
}

func NewDownloadManager() *DownloadManager {
	return &DownloadManager{
		downloads: make(map[uint]*DownloadStatus),
	}
}

func (dm *DownloadManager) UpdateStatus(id uint, progress float64, status string, err string) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	if _, exists := dm.downloads[id]; !exists {
		dm.downloads[id] = &DownloadStatus{ID: id, StartTime: time.Now()}
	}
	dm.downloads[id].Progress = progress
	dm.downloads[id].Status = status
	dm.downloads[id].Error = err
}

func (dm *DownloadManager) GetStatus(id uint) (DownloadStatus, bool) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	status, exists := dm.downloads[id]
	if !exists {
		return DownloadStatus{}, false
	}
	return *status, true
}

func (dm *DownloadManager) RemoveStatus(id uint) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	delete(dm.downloads, id)
}

func generateToken() string {
	uuidObj := uuid.Must(uuid.NewV7())
	return uuidObj.String()
}

func CreateTempURL(filePath string) (TempURL, error) {

	token := generateToken()
	expiration := time.Now().Add(24 * time.Hour)

	tempURL := TempURL{
		Token:     token,
		FilePath:  filePath,
		ExpiresAt: expiration,
	}

	if err := db.Create(&tempURL).Error; err != nil {
		return TempURL{}, errors.New("failed to create temporary URL")
	}

	return tempURL, nil
}

func cleanupExpiredURLs() {
	log.Debugln("cleanupExpiredURLs...")
	result := db.Unscoped().Where("expires_at < ?", time.Now()).Delete(&TempURL{})
	if result.Error != nil {
		fmt.Printf("Error cleaning up expired URLs: %v\n", result.Error)
	} else {
		fmt.Printf("Cleaned up %d expired temporary URLs\n", result.RowsAffected)
	}
}

func vacuumDatabase() {
	if err := db.Exec("VACUUM").Error; err != nil {
		log.Errorln(err)
	}
}

func PeriodicCleanup() {
	cleanupExpiredURLs()
	vacuumDatabase()
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		cleanupExpiredURLs()
		vacuumDatabase()
	}
}
