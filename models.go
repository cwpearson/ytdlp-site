package main

import (
	"sync"
	"time"

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
	Status        string // "pending", "downloading", "completed", "failed", "cancelled"
}

type User struct {
	gorm.Model
	Username string `gorm:"unique"`
	Password string
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
