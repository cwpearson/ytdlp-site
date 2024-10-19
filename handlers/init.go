package handlers

import (
	"ytdlp-site/config"

	"github.com/gorilla/sessions"
	"github.com/sirupsen/logrus"
)

var log *logrus.Logger
var store *sessions.CookieStore

func Init(logger *logrus.Logger) error {
	log = logger.WithFields(logrus.Fields{
		"component": "handlers",
	}).Logger

	// create the cookie store
	key, err := config.GetSessionAuthKey()
	if err != nil {
		return err
	}
	store = sessions.NewCookieStore(key)
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // seconds
		HttpOnly: true,
		Secure:   config.GetSecure(),
	}

	return nil
}

func Fini() {}
