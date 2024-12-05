package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var gitSHA string
var buildDate string

func GetDataDir() string {
	value, exists := os.LookupEnv("YTDLP_SITE_DATA_DIR")
	if exists {
		return value
	}
	return "data"
}

// defaults to GetDataDir() / config
func GetConfigDir() string {
	value, exists := os.LookupEnv("YTDLP_SITE_CONFIG_DIR")
	if exists {
		return value
	}
	return filepath.Join(GetDataDir(), "config")
}

func GetAdminInitialPassword() (string, error) {
	key := "YTDLP_SITE_ADMIN_INITIAL_PASSWORD"
	value, exists := os.LookupEnv(key)
	if exists {
		return value, nil
	}
	return "", fmt.Errorf("please set %s", key)
}

func GetSessionAuthKey() ([]byte, error) {
	key := "YTDLP_SITE_SESSION_AUTH_KEY"
	value, exists := os.LookupEnv(key)
	if exists {
		return []byte(value), nil
	}
	return []byte{}, fmt.Errorf("please set %s", key)
}

func GetSecure() bool {
	key := "YTDLP_SITE_SECURE"
	if value, exists := os.LookupEnv(key); exists {
		lower := strings.ToLower(value)
		if lower == "on" || lower == "1" || lower == "true" || lower == "yes" {
			return true
		}
	}
	return false
}

func GetGitSHA() string {
	if gitSHA == "" {
		return "<not provided>"
	} else {
		return gitSHA
	}
}

func GetBuildDate() string {
	if buildDate == "" {
		return "<not provided>"
	} else {
		return buildDate
	}
}
