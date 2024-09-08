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

type Video struct {
	gorm.Model
	URL           string
	Title         string
	VideoFilename string
	AudioFilename string
	UserID        uint
	Length        string
	AudioSize     string
	VideoSize     string
	Status        string // "pending", "downloading", "completed", "failed", "cancelled"
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
	result := db.Where("expires_at < ?", time.Now()).Delete(&TempURL{})
	if result.Error != nil {
		fmt.Printf("Error cleaning up expired URLs: %v\n", result.Error)
	} else {
		fmt.Printf("Cleaned up %d expired temporary URLs\n", result.RowsAffected)
	}
}

func vacuumDatabase() {
	if err := db.Exec("VACUUM").Error; err != nil {
		fmt.Println(err)
	}
}

func PeriodicCleanup() {
	ticker := time.NewTicker(12 * time.Hour)
	for range ticker.C {
		fmt.Println("PeriodicCleanup...")
		cleanupExpiredURLs()
		vacuumDatabase()
	}
}
