package media

import "gorm.io/gorm"

type Status string

const (
	Pending     Status = "pending"
	Transcoding Status = "transcoding"
	Completed   Status = "completed"
	Failed      Status = "failed"
)

type MediaFile struct {
	Size   int64
	Length float64
	Type   string
	Codec  string
}

type Audio struct {
	gorm.Model
	MediaFile
	OriginalID uint   // Original.ID
	Source     string // "original", "transcode"
	Bps        uint
	Filename   string
	Status     Status
}

type Video struct {
	gorm.Model
	MediaFile
	OriginalID uint   // Original.ID
	Source     string // "original", "transcode"
	Filename   string
	Width      uint
	Height     uint
	FPS        float64
	Status     Status
}
