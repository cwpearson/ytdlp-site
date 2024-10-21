package originals

import "github.com/sirupsen/logrus"

var log *logrus.Logger

func Init(logger *logrus.Logger) error {
	log = logger.WithFields(logrus.Fields{
		"component": "originals",
	}).Logger

	listeners = make(map[uint][]*Queue)

	return nil
}

func Fini() {}
