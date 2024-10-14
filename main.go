package main

import (
	"fmt"
	"html/template"
	"io"
	golog "log"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ytdlp-site/database"
	"ytdlp-site/ffmpeg"
	"ytdlp-site/handlers"
	"ytdlp-site/media"
	"ytdlp-site/originals"
	"ytdlp-site/playlists"
	"ytdlp-site/ytdlp"
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

	initLogger()

	log.Infof("GitSHA: %s", getGitSHA())
	log.Infof("BuildDate: %s", getBuildDate())

	ffmpeg.Init(log)
	handlers.Init(log)
	ytdlp.Init(log)

	gormLogger := logger.New(
		golog.New(os.Stdout, "\r\n", golog.LstdFlags), // io writer
		logger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logger.Warn, // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			ParameterizedQueries:      true,        // Don't include params in the SQL log
			Colorful:                  false,       // Disable color
		},
	)

	// Create config database
	err := os.MkdirAll(getConfigDir(), 0700)
	if err != nil {
		log.Panicf("failed to create config dir %s", getConfigDir())
	}

	// Initialize database
	dbPath := filepath.Join(getConfigDir(), "videos.db")
	db, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		log.Panicf("failed to connect to database %s", dbPath)
	}

	// set only a single connection so we don't actually have concurrent writes
	sqlDB, err := db.DB()
	if err != nil {
		log.Panicln("failed to retrieve database")
	}
	sqlDB.SetMaxOpenConns(1)

	// Migrate the schema
	db.AutoMigrate(&originals.Original{}, &playlists.Playlist{}, &media.Video{},
		&media.Audio{}, &User{}, &TempURL{}, &Transcode{})

	database.Init(db, log)
	defer database.Fini()

	go PeriodicCleanup()

	// create a user
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
	e.POST("/video/:id/restart", videoRestartHandler, authMiddleware)
	e.POST("/video/:id/delete", deleteOriginalHandler, authMiddleware)
	e.GET("/temp/:token", tempHandler)
	e.POST("/video/:id/process", processHandler, authMiddleware)
	e.POST("/video/:id/toggle_watched", handlers.ToggleWatched, authMiddleware)
	e.POST("/delete_video/:id", deleteVideoHandler, authMiddleware)
	e.POST("/delete_audio/:id", deleteAudioHandler, authMiddleware)
	e.POST("/transcode_to_video/:id", transcodeToVideoHandler, authMiddleware)
	e.POST("/transcode_to_audio/:id", transcodeToAudioHandler, authMiddleware)

	e.GET("/p/:id", playlistHandler, authMiddleware)
	e.POST("/p/:id/delete", deletePlaylistHandler, authMiddleware)

	dataGroup := e.Group("/data")
	dataGroup.Use(authMiddleware)
	dataGroup.Static("/", getDataDir())

	staticGroup := e.Group("/static")
	staticGroup.Use(authMiddleware)
	staticGroup.Static("/", "static")

	secure := getSecure()

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   30 * 24 * 60 * 60, // seconds
		HttpOnly: true,
		Secure:   secure,
	}

	// start the transcode worker
	go transcodeWorker()

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
