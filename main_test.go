package main

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type MockSender struct {
	Calls []Notification
}

func (m *MockSender) Send(n Notification) error {
	m.Calls = append(m.Calls, n)
	return nil
}

func TestConsoleSender(t *testing.T) {
	var buf bytes.Buffer
	sender := NewConsoleSender(&buf)

	n := Notification{
		Recipient: "user@example.com",
		Subject:   "Deploy done",
		Channel:   "console",
	}

	err := sender.Send(n)
	if err != nil {
		t.Fatalf("Send() error: %s", err)
	}

	got := buf.String()
	want := "[console] to: user@example.com | subject: Deploy done\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEmailSender(t *testing.T) {
	var buf bytes.Buffer
	sender := NewEmailSender(&buf)

	n := Notification{
		Recipient: "user@example.com",
		Subject:   "Deploy done",
		Channel:   "email",
	}

	err := sender.Send(n)
	if err != nil {
		t.Fatalf("Send() error: %s", err)
	}

	got := buf.String()
	want := "[email] to: user@example.com | subject: Deploy done\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		n       Notification
		wantErr bool
	}{
		{
			name: "valid notification",
			n: Notification{
				Recipient: "user@example.com",
				Subject:   "Deploy done",
				Channel:   "console",
			},
			wantErr: false,
		},
		{
			name: "empty recipient",
			n: Notification{
				Recipient: "",
				Subject:   "Deploy done",
				Channel:   "console",
			},
			wantErr: true,
		},
		{
			name: "empty channel",
			n: Notification{
				Recipient: "user@example.com",
				Subject:   "Deploy done",
				Channel:   "",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.n.Validate()
			if tt.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("expected no error, got %v", err)
			}
		})
	}
}

func TestHealthHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	senders := map[string]Sender{"console": &MockSender{}}

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
			mock := &MockSender{}
			senders := map[string]Sender{"console": mock}
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
