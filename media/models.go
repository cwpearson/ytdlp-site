package media

import "gorm.io/gorm"

type Status string

const (
	Pending     Status = "pending"
	Transcoding Status = "transcoding"
	Completed   Status = "completed"
	Failed      Status = "failed"
)

type Audio struct {
	gorm.Model
	OriginalID uint   // Original.ID
	Source     string // "original", "transcode"
	Bps        uint
	Length     float64
	Size       int64
	Type       string
	Codec      string
	Filename   string
	Status     Status
}

type Video struct {
	gorm.Model
	OriginalID uint   // Original.ID
	Source     string // "original", "transcode"
	Filename   string
	Width      uint
	Height     uint
	FPS        float64
	Length     float64
	Size       int64
	Type       string
	Codec      string
	Status     Status
}
