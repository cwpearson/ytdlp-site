package main

import (
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var db *gorm.DB

func ensureAdminAccount(db *gorm.DB) error {

	var user User
	if err := db.Where("username = ?", "admin").First(&user).Error; err != nil {
		// no such user

		password, err := getAdminInitialPassword()
		if err != nil {
			return err
		}

		err = CreateUser(db, "admin", password)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {

	// Create config database
	err := os.MkdirAll(getConfigDir(), 0700)
	if err != nil {
		panic("failed to create config dir")
	}

	// Initialize database
	dbPath := filepath.Join(getConfigDir(), "videos.db")
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&Video{}, &User{})

	// create a user
	// FIXME: only if this user doesn't exist
	err = ensureAdminAccount(db)
	if err != nil {
		panic(fmt.Sprintf("failed to create admin user: %v", err))
	}

	// create the cookie store
	key, err := getSessionAuthKey()
	if err != nil {
		panic(fmt.Sprintf("%v", err))
	}
	store = sessions.NewCookieStore(key)

	// Initialize Echo
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Templates
	t := &Template{
		templates: template.Must(template.ParseGlob("templates/*.html")),
	}
	e.Renderer = t

	// Routes
	e.GET("/", homeHandler)
	e.GET("/login", loginHandler)
	e.POST("/login", loginPostHandler)
	// e.GET("/register", registerHandler)
	// e.POST("/register", registerPostHandler)
	e.GET("/logout", logoutHandler)
	e.GET("/download", downloadHandler, authMiddleware)
	e.POST("/download", downloadPostHandler, authMiddleware)
	e.GET("/videos", videosHandler, authMiddleware)
	e.GET("/video/:id", videoHandler, authMiddleware)
	e.POST("/video/:id/cancel", videoCancelHandler, authMiddleware)
	e.POST("/video/:id/restart", videoRestartHandler, authMiddleware)
	e.POST("/video/:id/delete", videoDeleteHandler, authMiddleware)

	staticGroup := e.Group("/downloads")
	staticGroup.Use(authMiddleware)
	staticGroup.Static("/", getDownloadDir())
	// e.Static("/downloads", getDownloadDir())

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // seconds
		HttpOnly: true,
		Secure:   false, // needed for session to work over http
	}

	// Start server
	e.Logger.Fatal(e.Start(":8080"))
}

// Template renderer
type Template struct {
	templates *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.templates.ExecuteTemplate(w, name, data)
}
