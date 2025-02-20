package main

import (
	"errors"
	"fmt"
	"time"
	"ytdlp-site/originals"

	"github.com/google/uuid"
)

type TempURL struct {
	Token     string `gorm:"uniqueIndex"`
	FilePath  string
	ExpiresAt time.Time
}

func SetOriginalStatus(id uint, status originals.Status) error {
	return db.Model(&originals.Original{}).Where("id = ?", id).Update("status", status).Error
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
