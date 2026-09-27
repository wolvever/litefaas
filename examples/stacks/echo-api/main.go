package main

import (
	"log"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Item struct {
	ID   uint   `json:"id" gorm:"primaryKey"`
	Code string `json:"code" gorm:"uniqueIndex;size:64"`
	Name string `json:"name"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	e := echo.New()
	e.HideBanner = true
	e.GET("/healthz", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	db := openDB()
	seed := []Item{{Code: "e-1", Name: "echo"}, {Code: "e-2", Name: "gorm"}}

	e.GET("/", func(c echo.Context) error {
		if db == nil {
			return c.JSON(http.StatusOK, map[string]any{"service": "echo-api", "store": "memory", "items": seed})
		}
		var rows []Item
		if err := db.Find(&rows).Error; err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		}
		return c.JSON(http.StatusOK, map[string]any{"service": "echo-api", "store": "sql", "items": rows})
	})

	addr := "0.0.0.0:" + port
	log.Printf("echo-api listening on %s", addr)
	log.Fatal(e.Start(addr))
}

func openDB() *gorm.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("database unavailable (set DATABASE_URL): %v", err)
		return nil
	}
	if err := db.AutoMigrate(&Item{}); err != nil {
		log.Printf("automigrate: %v", err)
		return nil
	}
	return db
}
