package main

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"notifier/internal/notification"
	"notifier/internal/sender"
)

func TestHealthHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	senders := map[string]sender.Sender{"console": &sender.MockSender{}}

	s, err := NewServer(logger, senders)
	if err != nil {
		t.Fatalf("NewServer() error: %s", err)
	}

	r := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	s.healthHanlder(w, r)
	if w.Code != http.StatusOK {
		t.Errorf("status code %d, want %d", w.Code, http.StatusOK)
	}

	got := w.Body.String()
	want := `{"status": "ok", "version":"` + appVersion + `"}`
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestErrNotFoundThroughChain(t *testing.T) {
	mock := &sender.MockSender{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewServer(logger,
		map[string]sender.Sender{"email": mock})
	if err != nil {
		t.Fatalf("NewServer() error: %s", err)
	}

	_, err = s.findNotification(777)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, notification.ErrNotFound) {
		t.Errorf("errror.Is(err, ErrNotFound) = false, want true; got: %s", err)
	}
}

func TestCreateNotification(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantCalls  int
	}{
		{
			name: "valid notification",
			body: `{
			"to": "user@example.com", 
			"subject": "Deploy done", 
			"channel": "console"
			}`,
			wantStatus: http.StatusCreated,
			wantCalls:  1,
		},
		{
			name: "empty recipient",
			body: `{
			"to": "", 
			"subject": "Deploy done", 
			"channel": "console"
			}`,
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name: "empty channel",
			body: `{
			"to": "user@example.com", 
			"subject": "Deploy done", 
			"channel": ""
			}`,
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
		{
			name: "invalid channel",
			body: `{
			"to": "user@example.com", 
			"subject": "Deploy done", 
			"channel": "invalid"
			}`,
			wantStatus: http.StatusBadRequest,
			wantCalls:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &sender.MockSender{}
			senders := map[string]sender.Sender{"console": mock}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			s, err := NewServer(logger, senders)
			if err != nil {
				t.Fatalf("NewServer() error: %s", err)
			}

			r := httptest.NewRequest(http.MethodPost, "/api/notifications", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			s.createNotification(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status code %d, want %d", w.Code, http.StatusOK)
			}
			if len(mock.Calls) != tt.wantCalls {
				t.Errorf("got calls %d, want %d", len(mock.Calls), tt.wantCalls)
			}
		})
	}
}
