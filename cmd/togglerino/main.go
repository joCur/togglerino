package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/togglerino/togglerino/internal/config"
	"github.com/togglerino/togglerino/internal/store"
	"github.com/togglerino/togglerino/migrations"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	pool, err := store.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := store.RunMigrations(ctx, pool, migrations.FS); err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	fmt.Printf("togglerino starting on %s\n", cfg.Addr())
	if err := http.ListenAndServe(cfg.Addr(), mux); err != nil {
		log.Fatal(err)
	}
}
