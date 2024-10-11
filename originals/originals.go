package originals

import "gorm.io/gorm"

type Status string

const (
	StatusNotStarted        Status = "not started"
	StatusMetadata          Status = "metadata"
	StatusDownloading       Status = "downloading"
	StatusDownloadCompleted Status = "download completed"
	StatusTranscoding       Status = "transcoding"
	StatusCompleted         Status = "completed"
	StatusFailed            Status = "failed"
)

type Original struct {
	gorm.Model
	UserID  uint
	URL     string
	Title   string
	Artist  string
	Status  Status
	Audio   bool // video download requested
	Video   bool // audio download requested
	Watched bool

	Playlist   bool // part of a playlist
	PlaylistID uint // Playlist.ID (if part of a playlist)
}

func SetStatus(db *gorm.DB, id uint, status Status) error {
	return db.Model(&Original{}).Where("id = ?", id).Update("status", status).Error
}
