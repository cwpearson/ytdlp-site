package main

import (
	"fmt"
	"html/template"
	"io"
	golog "log"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ytdlp-site/config"
	"ytdlp-site/database"
	"ytdlp-site/ffmpeg"
	"ytdlp-site/handlers"
	"ytdlp-site/media"
	"ytdlp-site/originals"
	"ytdlp-site/playlists"
	"ytdlp-site/transcodes"
	"ytdlp-site/users"
	"ytdlp-site/ytdlp"
)

var db *gorm.DB

func ensureAdminAccount(db *gorm.DB) error {

	var user users.User
	if err := db.Where("username = ?", "admin").First(&user).Error; err != nil {
		// no such user

		password, err := config.GetAdminInitialPassword()
		if err != nil {
			return err
		}

		err = users.Create(db, "admin", password)
		if err != nil {
			return err
		}
	}
	return nil
}

func main() {

	initLogger()

	log.Infof("GitSHA: %s", config.GetGitSHA())
	log.Infof("BuildDate: %s", config.GetBuildDate())

	ffmpeg.Init(log)
	handlers.Init(log)
	ytdlp.Init(log)
	originals.Init(log)
	defer originals.Fini()

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
	err := os.MkdirAll(config.GetConfigDir(), 0700)
	if err != nil {
		log.Panicf("failed to create config dir %s", config.GetConfigDir())
	}

	// Initialize database
	dbPath := filepath.Join(config.GetConfigDir(), "videos.db")
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
	db.AutoMigrate(&originals.Original{}, &playlists.Playlist{},
		&media.Video{}, &media.Audio{}, &media.VideoClip{},
		&users.User{}, &TempURL{}, &transcodes.Transcode{})

	database.Init(db, log)
	defer database.Fini()
	err = handlers.Init(log)
	if err != nil {
		panic(fmt.Sprintf("%v", err))
	}
	defer handlers.Fini()

	go PeriodicCleanup()

	// create a user
	err = ensureAdminAccount(db)
	if err != nil {
		panic(fmt.Sprintf("failed to create admin user: %v", err))
	}

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
	e.GET("/login", handlers.LoginGet)
	e.POST("/login", handlers.LoginPost)
	// e.GET("/register", registerHandler)
	// e.POST("/register", registerPostHandler)
	e.GET("/logout", handlers.LogoutGet)
	e.GET("/download", downloadHandler, handlers.AuthMiddleware)
	e.POST("/download", downloadPostHandler, handlers.AuthMiddleware)
	e.GET("/videos", videosHandler, handlers.AuthMiddleware)
	e.GET("/video/:id", videoHandler, handlers.AuthMiddleware)
	e.POST("/video/:id/restart", videoRestartHandler, handlers.AuthMiddleware)
	e.POST("/video/:id/delete", deleteOriginalHandler, handlers.AuthMiddleware)
	e.GET("/temp/:token", tempHandler)
	e.POST("/video/:id/process", processHandler, handlers.AuthMiddleware)
	e.POST("/video/:id/toggle_watched", handlers.ToggleWatched, handlers.AuthMiddleware)
	e.POST("/delete_video/:id", deleteVideoHandler, handlers.AuthMiddleware)
	e.POST("/delete_audio/:id", deleteAudioHandler, handlers.AuthMiddleware)
	e.POST("/transcode_to_video/:id", transcodeToVideoHandler, handlers.AuthMiddleware)
	e.POST("/transcode_to_audio/:id", transcodeToAudioHandler, handlers.AuthMiddleware)
	e.GET("/status", handlers.StatusGet, handlers.AuthMiddleware)
	e.GET("/videos/events", handlers.VideosEvents, handlers.AuthMiddleware)

	e.GET("/p/:id", playlistHandler, handlers.AuthMiddleware)
	e.POST("/p/:id/delete", deletePlaylistHandler, handlers.AuthMiddleware)

	dataGroup := e.Group("/data")
	dataGroup.Use(handlers.AuthMiddleware)
	dataGroup.Static("/", config.GetDataDir())

	staticGroup := e.Group("/static")
	staticGroup.Use(handlers.AuthMiddleware)
	staticGroup.Static("/", "static")

	// tidy up the transcodes database
	log.Debug("tidy transcodes database...")
	cleanupTranscodes()

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
