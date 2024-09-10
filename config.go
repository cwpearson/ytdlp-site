package main

import (
	"errors"
	"fmt"
	"os"
)

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
	return "", errors.New(fmt.Sprintf("please set %s", key))
}

func getSessionAuthKey() ([]byte, error) {
	key := "YTDLP_SITE_SESSION_AUTH_KEY"
	value, exists := os.LookupEnv(key)
	if exists {
		return []byte(value), nil
	}
	return []byte{}, errors.New(fmt.Sprintf("please set %s", key))
}
