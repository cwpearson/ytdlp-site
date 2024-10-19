package users

import (
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type User struct {
	gorm.Model
	Username string `gorm:"unique"`
	Password string
}

func Create(db *gorm.DB, username, password string) error {
	hashedPassword, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	user := User{Username: username, Password: string(hashedPassword)}
	if err := db.Create(&user).Error; err != nil {
		return err
	}
	return nil
}
