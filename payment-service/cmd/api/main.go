package main

import (
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	e := echo.New()
	e.Use(middleware.RequestLogger())

	e.GET("/trip/preview", func(c *echo.Context) error {
		return c.String(http.StatusOK, "Trip preview")
	})

	if err := e.Start(":8083"); err != nil {
		e.Logger.Error("Failed to start server: ", err)
	}
}
