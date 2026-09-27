package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/jmoiron/sqlx"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Item struct {
	Code string `json:"code" db:"code"`
	Name string `json:"name" db:"name"`
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	db := openDB()
	seed := []Item{{Code: "c-1", Name: "chi"}, {Code: "c-2", Name: "sqlx"}}

	r.Get("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if db == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{"service": "chi-api", "store": "memory", "items": seed})
			return
		}
		var rows []Item
		if err := db.Select(&rows, "SELECT code, name FROM items LIMIT 100"); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"service": "chi-api", "store": "sql", "items": rows})
	})

	addr := "0.0.0.0:" + port
	log.Printf("chi-api listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, r))
}

func openDB() *sqlx.DB {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return nil
	}
	db, err := sqlx.Open("pgx", dsn)
	if err != nil {
		log.Printf("database unavailable (set DATABASE_URL): %v", err)
		return nil
	}
	if err := db.Ping(); err != nil {
		log.Printf("database ping failed: %v", err)
		_ = db.Close()
		return nil
	}
	return db
}
