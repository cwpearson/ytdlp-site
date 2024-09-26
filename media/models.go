package media

import "gorm.io/gorm"

type Status string

const (
	Transcoding Status = "transcoding"
)

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
}
