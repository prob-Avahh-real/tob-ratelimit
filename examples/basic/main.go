package main

import (
	"fmt"
	"log"
	"net/http"

	goredis "github.com/redis/go-redis/v9"

	"github.com/skat/tob-ratelimit/internal/middleware"
	"github.com/skat/tob-ratelimit/pkg/ratelimit"
)

func main() {
	cfg := ratelimit.DefaultConfig()
	cfg.BlockedIPs["203.0.113.99"] = struct{}{}

	// Pass nil Redis client to use in-memory buckets (demo); use real client in production.
	var redisClient *goredis.Client
	limiter := ratelimit.New(cfg, redisClient)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "login ok")
	})
	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "data ok")
	})

	handler := middleware.Middleware(limiter)(mux)

	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
