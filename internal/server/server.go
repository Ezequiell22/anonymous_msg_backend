package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"strings"
	"time"

	applog "backend_msgs_golang/internal/log"
	"backend_msgs_golang/internal/storage"
)

type Config struct {
	Addr              string
	PlaceholderTTL    time.Duration
	MessageTTL        time.Duration
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	MaxBodyBytes      int64
}

type Server struct {
	cfg    Config
	store  storage.Storage
	router http.Handler
	log    applog.Logger
}

func New(cfg Config, st storage.Storage, lg applog.Logger) *Server {
	s := &Server{cfg: cfg, store: st, log: lg}
	mux := http.NewServeMux()
	mux.HandleFunc("/code", s.getCode)
	mux.HandleFunc("/message/", s.message)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { s.secHeaders(w); w.WriteHeader(http.StatusOK) })
	s.router = mux
	return s
}

func (s *Server) Handler() http.Handler { return s.router }

func (s *Server) secHeaders(w http.ResponseWriter) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Pragma", "no-cache")
}

func (s *Server) generateCode(n int) string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"
	var b strings.Builder
	for i := 0; i < n; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b.WriteByte(letters[idx.Int64()])
	}
	return b.String()
}

func (s *Server) getCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	s.secHeaders(w)
	ctx := r.Context()
	var code string
	for {
		code = s.generateCode(8)
		ok, err := s.store.ReserveCode(ctx, code, s.cfg.PlaceholderTTL)
		if err != nil {
			if s.log != nil {
				s.log.Error("reserve_code_error", map[string]any{"endpoint": "code"})
			}
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if ok {
			break
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"code": code})
}

func (s *Server) message(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, "/message/") {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	if r.Method == http.MethodPut {
		s.putMessage(w, r)
		return
	}
	if r.Method == http.MethodGet {
		s.getMessage(w, r)
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (s *Server) putMessage(w http.ResponseWriter, r *http.Request) {
	s.secHeaders(w)
	code := strings.TrimPrefix(r.URL.Path, "/message/")
	max := s.cfg.MaxBodyBytes
	if max <= 0 {
		max = 1 << 20
	}
	r.Body = http.MaxBytesReader(w, r.Body, max)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if s.log != nil {
			s.log.Warn("body_read_error", map[string]any{"endpoint": "message_put"})
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	ct := strings.TrimSpace(string(body))
	if ct == "" {
		if s.log != nil {
			s.log.Warn("empty_body", map[string]any{"endpoint": "message_put"})
		}
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	ok, err := s.store.AttachCipher(r.Context(), code, ct, s.cfg.MessageTTL)
	if err != nil {
		if s.log != nil {
			s.log.Error("attach_cipher_error", map[string]any{"endpoint": "message_put"})
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !ok {
		if s.log != nil {
			s.log.Warn("attach_conflict", map[string]any{"endpoint": "message_put"})
		}
		w.WriteHeader(http.StatusConflict)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) getMessage(w http.ResponseWriter, r *http.Request) {
	s.secHeaders(w)
	code := strings.TrimPrefix(r.URL.Path, "/message/")
	ct, ok, err := s.store.GetAndDelete(r.Context(), code)
	if err != nil {
		if s.log != nil {
			s.log.Error("get_delete_error", map[string]any{"endpoint": "message_get"})
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte(ct))
}

func validBase64(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '+' || c == '/' || c == '=' {
			continue
		}
		return false
	}
	return true
}

func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Addr:              s.cfg.Addr,
		Handler:           s.router,
		ReadTimeout:       s.cfg.ReadTimeout,
		ReadHeaderTimeout: s.cfg.ReadHeaderTimeout,
		WriteTimeout:      s.cfg.WriteTimeout,
		IdleTimeout:       s.cfg.IdleTimeout,
	}
	go func() {
		<-ctx.Done()
		c, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(c)
	}()
	return srv.ListenAndServe()
}
