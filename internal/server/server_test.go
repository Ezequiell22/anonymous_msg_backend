package server

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type mockStore struct {
	reserveOK bool
	attachOK  bool
	attachErr error
	getVal    string
	getOK     bool
	getErr    error
	pingErr   error
}

func (m *mockStore) ReserveCode(_ context.Context, code string, ttl time.Duration) (bool, error) {
	return m.reserveOK, nil
}
func (m *mockStore) AttachCipher(_ context.Context, code string, ciphertext string, ttl time.Duration) (bool, error) {
	return m.attachOK, m.attachErr
}
func (m *mockStore) GetAndDelete(_ context.Context, code string) (string, bool, error) {
	return m.getVal, m.getOK, m.getErr
}
func (m *mockStore) Ping(_ context.Context) error { return m.pingErr }

func newTestServer(store *mockStore) *Server {
	cfg := Config{Addr: ":0", PlaceholderTTL: time.Minute, MessageTTL: time.Hour}
	logger := &nopLogger{}
	server := New(cfg, store, logger)
	return server
}

type nopLogger struct{}

func (*nopLogger) Debug(string, map[string]any) {}
func (*nopLogger) Info(string, map[string]any)  {}
func (*nopLogger) Warn(string, map[string]any)  {}
func (*nopLogger) Error(string, map[string]any) {}

func TestPostCode201(t *testing.T) {
	store := &mockStore{reserveOK: true}
	server := newTestServer(store)
	request := httptest.NewRequest(http.MethodPost, "/code", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", recorder.Code)
	}
	if recorder.Header().Get("Location") == "" {
		t.Fatalf("missing Location header")
	}
}

func TestPutMessageValid(t *testing.T) {
	store := &mockStore{attachOK: true}
	server := newTestServer(store)
	initializationVector := make([]byte, 12)
	payload := []byte("abc")
	body := base64.StdEncoding.EncodeToString(append(initializationVector, payload...))
	request := httptest.NewRequest(http.MethodPut, "/message/xyz", strings.NewReader(body))
	request.Header.Set("Content-Type", "text/plain")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
}

func TestPutMessageBadBase64(t *testing.T) {
	server := newTestServer(&mockStore{})
	request := httptest.NewRequest(http.MethodPut, "/message/xyz", strings.NewReader("%%%"))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestPutMessageInvalidIV(t *testing.T) {
	server := newTestServer(&mockStore{})
	body := base64.StdEncoding.EncodeToString([]byte("short"))
	request := httptest.NewRequest(http.MethodPut, "/message/xyz", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", recorder.Code)
	}
}

func TestPutMessageConflict(t *testing.T) {
	server := newTestServer(&mockStore{attachOK: false})
	initializationVector := make([]byte, 12)
	body := base64.StdEncoding.EncodeToString(append(initializationVector, []byte("x")...))
	request := httptest.NewRequest(http.MethodPut, "/message/xyz", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", recorder.Code)
	}
}

func TestPutMessageError(t *testing.T) {
	server := newTestServer(&mockStore{attachOK: false, attachErr: assertErr{}})
	initializationVector := make([]byte, 12)
	body := base64.StdEncoding.EncodeToString(append(initializationVector, []byte("x")...))
	request := httptest.NewRequest(http.MethodPut, "/message/xyz", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "err" }

func TestGetMessageOK(t *testing.T) {
	server := newTestServer(&mockStore{getVal: "abc", getOK: true})
	request := httptest.NewRequest(http.MethodGet, "/message/xyz", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
	if strings.TrimSpace(recorder.Body.String()) != "abc" {
		t.Fatalf("unexpected body")
	}
}

func TestGetMessageNotFound(t *testing.T) {
	server := newTestServer(&mockStore{getOK: false})
	request := httptest.NewRequest(http.MethodGet, "/message/xyz", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", recorder.Code)
	}
}

func TestGetMessageError(t *testing.T) {
	server := newTestServer(&mockStore{getErr: assertErr{}})
	request := httptest.NewRequest(http.MethodGet, "/message/xyz", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", recorder.Code)
	}
}

func TestHealth(t *testing.T) {
	server := newTestServer(&mockStore{})
	request := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recorder.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	store := &mockStore{}
	cfg := Config{AllowedOrigins: []string{"*"}}
	logger := &nopLogger{}
	server := New(cfg, store, logger)
	request := httptest.NewRequest(http.MethodOptions, "/code", nil)
	request.Header.Set("Origin", "http://example.com")
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", recorder.Code)
	}
	if recorder.Header().Get("Access-Control-Allow-Origin") == "" {
		t.Fatalf("missing CORS header")
	}
}

func TestRateLimit(t *testing.T) {
	store := &mockStore{reserveOK: true}
	cfg := Config{RateLimitRPS: 1, RateBurst: 1}
	logger := &nopLogger{}
	server := New(cfg, store, logger)
	server.tokens = make(chan struct{}, 1)
	server.tokens <- struct{}{}
	firstRequest := httptest.NewRequest(http.MethodPost, "/code", nil)
	firstRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(firstRecorder, firstRequest)
	if firstRecorder.Code == http.StatusTooManyRequests {
		t.Fatalf("rate limited unexpectedly")
	}
	secondRequest := httptest.NewRequest(http.MethodPost, "/code", nil)
	secondRecorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(secondRecorder, secondRequest)
	if secondRecorder.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", secondRecorder.Code)
	}
}
