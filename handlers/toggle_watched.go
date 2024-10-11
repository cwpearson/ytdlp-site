package handlers

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"ytdlp-site/database"
	"ytdlp-site/originals"
)

func ToggleWatched(c echo.Context) error {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	db := database.Get()

	result := db.Model(&originals.Original{}).
		Where("id = ?", id).
		Update("watched", gorm.Expr("NOT watched"))

	if result.Error != nil {
		log.Errorln(result.Error)
	}

	if result.RowsAffected == 0 {
		log.Errorln(gorm.ErrRecordNotFound)
	}

	referrer := c.Request().Referer()
	if referrer == "" {
		referrer = "/videos"
	}
	return c.Redirect(http.StatusSeeOther, referrer)
}
