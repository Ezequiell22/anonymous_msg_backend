package handler

import (
	"crypto/tls"
	"net/http"
	"os"
	"strconv"
	"time"

	applog "backend_msgs_golang/internal/log"
	"backend_msgs_golang/internal/server"
	redisstore "backend_msgs_golang/internal/storage/redis"

	redis "github.com/redis/go-redis/v9"
)

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func envInt64(key string, def int64) int64 {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	i, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return i
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	if v == "1" || v == "true" || v == "TRUE" {
		return true
	}
	if v == "0" || v == "false" || v == "FALSE" {
		return false
	}
	return def
}

var handler http.Handler

func init() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}
	placeholderTTL := envDuration("PLACEHOLDER_TTL", 30*time.Minute)
	messageTTL := envDuration("MESSAGE_TTL", 24*time.Hour)
	redisAddr := os.Getenv("REDIS_ADDR")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisDBStr := os.Getenv("REDIS_DB")
	redisDB := 0
	if redisDBStr != "" {
		if i, err := strconv.Atoi(redisDBStr); err == nil {
			redisDB = i
		}
	}
	useTLS := envBool("REDIS_TLS", false)

	ropts := &redis.Options{Addr: redisAddr, Password: redisPassword, DB: redisDB}
	if useTLS {
		ropts.TLSConfig = &tls.Config{}
	}
	st := redisstore.NewWithOptions(ropts)
	lg := applog.New(os.Getenv("LOG_LEVEL"))
	cfg := server.Config{
		Addr:              addr,
		PlaceholderTTL:    placeholderTTL,
		MessageTTL:        messageTTL,
		ReadTimeout:       envDuration("READ_TIMEOUT", 5*time.Second),
		ReadHeaderTimeout: envDuration("READ_HEADER_TIMEOUT", 5*time.Second),
		WriteTimeout:      envDuration("WRITE_TIMEOUT", 10*time.Second),
		IdleTimeout:       envDuration("IDLE_TIMEOUT", 60*time.Second),
		MaxBodyBytes:      envInt64("MAX_BODY_BYTES", 1<<20),
	}
	handler = server.New(cfg, st, lg).Handler()
}

func Handler(w http.ResponseWriter, r *http.Request) {
	handler.ServeHTTP(w, r)
}
