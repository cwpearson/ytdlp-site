package ytdlp

import "github.com/sirupsen/logrus"

var log *logrus.Logger

func Init(logger *logrus.Logger) error {
	log = logger.WithFields(logrus.Fields{
		"component": "ytdlp",
	}).Logger
	return nil
}
