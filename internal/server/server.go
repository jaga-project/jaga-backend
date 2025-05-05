package server

import (
    "github.com/labstack/echo/v4"
    "gorm.io/gorm"
)

func NewRouter(db *gorm.DB) *echo.Echo {
    e := echo.New()

    // Daftarkan rute di sini
    e.GET("/", func(c echo.Context) error {
        return c.JSON(200, map[string]string{
            "message": "🚀 JAGA Backend is running!",
        })
    })

    return e
}
