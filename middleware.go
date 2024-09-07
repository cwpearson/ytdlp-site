package main

import (
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
)

var store *sessions.CookieStore

func authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		session, err := store.Get(c.Request(), "session")
		if err != nil {
			return c.String(http.StatusInternalServerError, "Error: Unable to retrieve session")
		}
		userID, ok := session.Values["user_id"]
		if !ok {
			fmt.Println("authMiddleware: session does not contain user_id. Redirect to /login")
			// return c.String(http.StatusForbidden, "not logged in")
			return c.Redirect(http.StatusSeeOther, "/login")
		}
		fmt.Println("set user_id", userID, "in context")
		c.Set("user_id", userID)
		return next(c)
	}
}
