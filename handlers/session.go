package handlers

import (
	"fmt"

	"github.com/labstack/echo/v4"
)

type User struct {
	Id uint
}

func GetUser(c echo.Context) (User, error) {
	session, err := store.Get(c.Request(), "session")
	if err == nil {
		val, ok := session.Values["user_id"]
		if ok {
			return User{Id: val.(uint)}, nil
		} else {
			return User{}, fmt.Errorf("user_id not in session")
		}
	} else {
		return User{}, fmt.Errorf("couldn't retureve session from store")
	}

}
