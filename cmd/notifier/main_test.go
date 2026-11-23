package main

import (
	"errors"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"notifier/internal/notification"
	"notifier/internal/sender"
	"notifier/internal/store"
)

func TestErrNotFoundThroughChain(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	s, err := NewServer(store.NewMemoryStore(), logger,
		map[string]sender.Sender{"email": &sender.MockSender{}})
	if err != nil {
		t.Fatalf("NewServer() error: %s", err)
	}

	_, err = s.store.GetById(777)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !errors.Is(err, notification.ErrNotFound) {
		t.Errorf("errors.Is(err, ErrNotFound) = false, want true; got: %s", err)
	}
}

func TestHealthHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	senders := map[string]sender.Sender{"console": &sender.MockSender{}}

	s, err := NewServer(store.NewMemoryStore(), logger, senders)
	if err != nil {
		t.Fatalf("NewServer() error: %s", err)
	}

	r := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	s.healthHandler(w, r)

	if w.Code != 200 {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ok") {
		t.Errorf("body = %q, want contains %q", w.Body.String(), "ok")
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
			name:       "valid requests",
			body:       `{"to": "user@example.com", "subject": "Deploy done", "channel":"email"}`,
			wantStatus: 201,
			wantCalls:  1,
		},
		{
			name:       "empty requests",
			body:       `{"to": "", "subject": "Deploy done", "channel":"email"}`,
			wantStatus: 400,
			wantCalls:  0,
		},
		{
			name:       "unknown channel",
			body:       `{"to": "user@example.com", "subject": "Deploy done", "channel":""}`,
			wantStatus: 400,
			wantCalls:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &sender.MockSender{}
			logger := slog.New(slog.NewTextHandler(io.Discard, nil))
			s, err := NewServer(store.NewMemoryStore(), logger,
				map[string]sender.Sender{"email": mock})
			if err != nil {
				t.Fatalf("NewServer() error: %s", err)
			}

			r := httptest.NewRequest("POST", "/api/notifications", strings.NewReader(tt.body))
			w := httptest.NewRecorder()
			s.createNotification(w, r)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", w.Code, tt.wantStatus)
			}
			if len(mock.Calls) != tt.wantCalls {
				t.Errorf("calls = %d, want %d", len(mock.Calls), tt.wantCalls)
			}
		})
	}
}
