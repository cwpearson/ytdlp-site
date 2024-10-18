package playlists

import (
	"ytdlp-site/database"

	"gorm.io/gorm"
)

type Status string

type Playlist struct {
	gorm.Model
	UserID uint
	URL    string
	Title  string
	Status Status
	Audio  bool
	Video  bool
}

const (
	StatusNotStarted  Status = "not started"
	StatusDownloading Status = "downloading"
	StatusCompleted   Status = "completed"
	StatusFailed      Status = "failed"
)

func SetStatus(id uint, status Status) error {
	db := database.Get()
	return db.Model(&Playlist{}).Where("id = ?", id).Update("status", status).Error
}
