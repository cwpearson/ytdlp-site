package handlers

import (
	"encoding/json"
	"fmt"
	"ytdlp-site/originals"

	"github.com/labstack/echo/v4"
)

func VideosEvents(c echo.Context) error {

	user, err := GetUser(c)
	if err != nil {
		return err
	}

	req := c.Request()
	res := c.Response()

	// Set headers for SSE
	res.Header().Set(echo.HeaderContentType, "text/event-stream")
	res.Header().Set("Cache-Control", "no-cache")
	res.Header().Set("Connection", "keep-alive")

	// Create a channel to signal client disconnect
	done := req.Context().Done()

	q := originals.Subscribe(user.Id)
	defer originals.Unsubscribe(user.Id, q)

	// Send SSE messages
	for {
		select {
		case <-done:
			return nil
		default:
			event := <-q.Ch

			jsonData, err := json.Marshal(event)
			if err != nil {
				return err
			}

			msg := fmt.Sprintf("data: %s\n\n", jsonData)
			_, err = res.Write([]byte(msg))
			if err != nil {
				return err
			}
			res.Flush()
		}
	}
}
