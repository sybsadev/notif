package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	appName    = "notifr"
	appVersion = "0.1.0"
)

var ErrNotFound = errors.New("notification not found")

type Sender interface {
	Send(Notification) error
}

type LoggingSender struct {
	Sender
	logger *slog.Logger
}

type ConsoleSender struct {
	w io.Writer
}
type EmailSender struct {
	w io.Writer
}
type TelegramSender struct{}

type Server struct {
	notifications map[int]Notification
	nextID        int
	logger        *slog.Logger
	senders       map[string]Sender
}

type Notification struct {
	ID        int    `json:"id"`
	Recipient string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Channel   string `json:"channel"`
	IsUrgent  bool   `json:"urgent"`
}

func (l LoggingSender) Send(n Notification) error {
	l.logger.Info("sending", "to", n.Recipient, "channel", n.Channel)

	err := l.Sender.Send(n)
	if err != nil {
		l.logger.Error("send failed", "to", n.Recipient, "error", err)
		return err
	}

	l.logger.Info("sent", "to", n.Recipient, "channel", n.Channel)
	return nil
}

func NewConsoleSender(w io.Writer) *ConsoleSender {
	if w == nil {
		w = os.Stdout
	}

	return &ConsoleSender{w: w}
}

func (c ConsoleSender) Send(n Notification) error {
	_, err := fmt.Fprintf(c.w, "[console] to: %s | subject: %s\n", n.Recipient, n.Subject)
	return err
}

func NewEmailSender(w io.Writer) *EmailSender {
	if w == nil {
		w = os.Stdout
	}

	return &EmailSender{w: w}
}

func (c EmailSender) Send(n Notification) error {
	_, err := fmt.Fprintf(c.w, "[email] to: %s | subject: %s\n", n.Recipient, n.Subject)
	return err
}

func (c TelegramSender) Send(n Notification) error {
	fmt.Printf("[telegram] to: %s | subject: %s\n", n.Recipient, n.Subject)
	return nil
}

func (n Notification) Format() string {
	return fmt.Sprintf("to: %s | subject: %s | body: %s | channel: %s", n.Recipient, n.Subject, n.Body, n.Channel)
}

func (n Notification) Validate() error {
	const op = "Notification.Validate"

	if n.Recipient == "" {
		return fmt.Errorf("%s: recipient not set", op)
	}
	if n.Channel == "" {
		return fmt.Errorf("%s: channel not set", op)
	}
	return nil
}

func (s *Server) findNotification(id int) (Notification, error) {
	const op = "Server.findNotification"

	n, ok := s.notifications[id]
	if !ok {
		return Notification{}, fmt.Errorf("%s: %w", op, ErrNotFound)
	}

	return n, nil
}

func (s *Server) healthHanlder(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status": "ok", "version":"` + appVersion + `"}`))
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	var all []Notification
	for _, n := range s.notifications {
		all = append(all, n)
	}

	js, err := json.Marshal(all)
	if err != nil {
		s.logger.Error("marshal", "error", err)
		return
	}

	w.Write(js)
}

func (s *Server) getNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.getNotification"

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("%s: invalid id", op), http.StatusBadRequest)
	}

	n, err := s.findNotification(id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			http.Error(w, fmt.Errorf("%s: %w", op, err).Error(), http.StatusNotFound)
			return
		}

		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(n)
}

func (s *Server) createNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.createNotification"

	var n Notification

	err := json.NewDecoder(r.Body).Decode(&n)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid json"}`))
		return
	}

	err = n.Validate()
	if err != nil {
		err = fmt.Errorf("%s: %w", op, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sender, ok := s.senders[n.Channel]
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "invalid channel"}`))
		return
	}

	s.nextID++
	n.ID = s.nextID
	s.notifications[n.ID] = n

	err = sender.Send(n)
	if err != nil {
		s.logger.Error("send failed", "error", err, "channel", n.Channel)
	}

	js, err := json.Marshal(n)
	if err != nil {
		s.logger.Error("marshal", "error:", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	w.Write(js)
}

func (s *Server) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		s.logger.Info("request completed", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start), "from", r.RemoteAddr)
	})
}

func contentType(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func NewServer(logger *slog.Logger, senders map[string]Sender) (*Server, error) {
	const op = "NewServer"

	if logger == nil {
		return nil, fmt.Errorf("%s: logger is required", op)
	}
	if len(senders) == 0 {
		return nil, fmt.Errorf("%s: at least 1 sender is required", op)
	}

	return &Server{
		notifications: map[int]Notification{},
		logger:        logger,
		senders:       senders,
	}, nil
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	senders := map[string]Sender{
		"console":  LoggingSender{Sender: NewConsoleSender(os.Stdout), logger: logger},
		"email":    LoggingSender{Sender: NewEmailSender(os.Stdout), logger: logger},
		"telegram": LoggingSender{TelegramSender{}, logger},
	}

	s, err := NewServer(logger, senders)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %s\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHanlder)
	mux.HandleFunc("GET /api/notifications", s.listNotifications)
	mux.HandleFunc("GET /api/notifications/{id}", s.getNotification)
	mux.HandleFunc("POST /api/notifications", s.createNotification)

	logger.Info("starting :8080", appName, appVersion)
	http.ListenAndServe(":8080", s.logRequest(contentType(mux)))
}
