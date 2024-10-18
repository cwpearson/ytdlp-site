package transcodes

import (
	"time"

	"gorm.io/gorm"
)

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
	Height uint    // target height
	Width  uint    // target width
	FPS    float64 // target FPS

	// audio & video fields
	Kbps uint
}
