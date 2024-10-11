package database

import (
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

var db *gorm.DB
var log *logrus.Logger

func Init(d *gorm.DB, logger *logrus.Logger) error {
	db = d
	log = logger.WithFields(logrus.Fields{
		"component": "database",
	}).Logger
	return nil
}

func Fini() {}

func Get() *gorm.DB {
	if db == nil {
		panic("didn't call database.Init(...)")
	}
	return db
}
