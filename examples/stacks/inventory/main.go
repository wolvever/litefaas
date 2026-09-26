package main

import (
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type SKU struct {
	ID    uint   `json:"id" gorm:"primaryKey"`
	Code  string `json:"code" gorm:"uniqueIndex;size:64"`
	OnHand int   `json:"on_hand"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := gin.New()
	r.Use(gin.Recovery())
	r.GET("/healthz", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	db := openDB()
	seed := []SKU{{Code: "mug-1", OnHand: 12}, {Code: "tee-1", OnHand: 4}}

	r.GET("/", func(c *gin.Context) {
		if db == nil {
			c.JSON(http.StatusOK, gin.H{"service": "inventory", "store": "memory", "items": seed})
			return
		}
		var rows []SKU
		if err := db.Find(&rows).Error; err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"service": "inventory", "store": "sql", "items": rows})
	})

	addr := ":" + port
	log.Printf("inventory listening on %s", addr)
	log.Fatal(r.Run(addr))
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
	if err := db.AutoMigrate(&SKU{}); err != nil {
		log.Printf("automigrate: %v", err)
		return nil
	}
	return db
}
