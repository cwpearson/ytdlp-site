package main

import (
	"fmt"
	"os"
	"strings"
)

var GitSHA string

func getDataDir() string {
	value, exists := os.LookupEnv("YTDLP_SITE_DATA_DIR")
	if exists {
		return value
	}
	return "data"
}

func getConfigDir() string {
	value, exists := os.LookupEnv("YTDLP_SITE_CONFIG_DIR")
	if exists {
		return value
	}
	return "config"
}

func getAdminInitialPassword() (string, error) {
	key := "YTDLP_SITE_ADMIN_INITIAL_PASSWORD"
	value, exists := os.LookupEnv(key)
	if exists {
		return value, nil
	}
	return "", fmt.Errorf("please set %s", key)
}

func getSessionAuthKey() ([]byte, error) {
	key := "YTDLP_SITE_SESSION_AUTH_KEY"
	value, exists := os.LookupEnv(key)
	if exists {
		return []byte(value), nil
	}
	return []byte{}, fmt.Errorf("please set %s", key)
}

func getSecure() bool {
	key := "YTDLP_SITE_SECURE"
	if value, exists := os.LookupEnv(key); exists {
		lower := strings.ToLower(value)
		if lower == "on" || lower == "1" || lower == "true" || lower == "yes" {
			return true
		}
	}
	return false
}

func getGitSHA() string {

	if GitSHA == "" {
		return "<not provided>"
	} else {
		return GitSHA
	}

}
