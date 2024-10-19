package originals

import (
	"sync"
	"ytdlp-site/database"
	"ytdlp-site/transcodes"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

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

var listeners map[uint][]*Queue // map of userId to queues
var lMu sync.Mutex

func bcast(userId, origId uint, status Status) {
	lMu.Lock()
	defer lMu.Unlock()

	qs, ok := listeners[userId]
	if ok {
		for _, q := range qs {
			q.Ch <- Event{origId, status}
		}
	}
}

func SetStatus(id uint, status Status) error {
	db := database.Get()
	log.Debugln("original", id, "status -> ", status)
	err := db.Model(&Original{}).Where("id = ?", id).Update("status", status).Error
	if err != nil {
		return err
	}
	var orig Original
	err = db.Where("id = ?", id).First(&orig).Error
	if err != nil {
		return err
	}
	bcast(orig.UserID, id, status)
	return nil
}

// if there is an active transcode for this original,
// set the status to transcode. otherwise ,to completed
func SetStatusTranscodingOrCompleted(id uint) error {
	db := database.Get()

	var count int64
	err := db.Model(&transcodes.Transcode{}).Where("original_id = ?", id).Count(&count).Error
	if err != nil {
		return err
	}

	if count > 0 {
		log.Debugln("found transcodes for original", id)
		return SetStatus(id, StatusTranscoding)
	} else {
		log.Debugln("no transcodes for original", id)
		return SetStatus(id, StatusCompleted)
	}
}

type Event struct {
	VideoId uint
	Status  Status
}

type Queue struct {
	id uuid.UUID
	Ch chan Event
}

func newQueue() *Queue {
	return &Queue{
		id: uuid.Must(uuid.NewV7()),
		Ch: make(chan Event),
	}
}

func Subscribe(userId uint) *Queue {
	_, ok := listeners[userId]
	if !ok {
		listeners[userId] = make([]*Queue, 0)
	}
	q := newQueue()
	lMu.Lock()
	listeners[userId] = append(listeners[userId], q)
	lMu.Unlock()
	return q
}

func Unsubscribe(userId uint, q *Queue) {
	lMu.Lock()
	defer lMu.Unlock()

	qs, ok := listeners[userId]
	if !ok {
		return
	}

	newQs := []*Queue{}
	for _, oldQ := range qs {
		if oldQ != q {
			newQs = append(newQs, oldQ)
		}
	}
	listeners[userId] = newQs
}
