package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/5/stdlib"

	"notifier/internal/notification"
	"notifier/internal/sender"
)

const (
	appName    = "notifr"
	appVersion = "0.1.0"
)

type Server struct {
	notifications map[int]notification.Notification
	nextID        int
	logger        *slog.Logger
	senders       map[string]sender.Sender
}

func (s *Server) findNotification(id int) (notification.Notification, error) {
	const op = "Server.findNotification"

	n, ok := s.notifications[id]
	if !ok {
		return notification.Notification{}, fmt.Errorf("%s: %w", op, notification.ErrNotFound)
	}

	return n, nil
}

func (s *Server) healthHanlder(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status": "ok", "version":"` + appVersion + `"}`))
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	var all []notification.Notification
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
		if errors.Is(err, notification.ErrNotFound) {
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

	var n notification.Notification

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

func NewServer(logger *slog.Logger, senders map[string]sender.Sender) (*Server, error) {
	const op = "NewServer"

	if logger == nil {
		return nil, fmt.Errorf("%s: logger is required", op)
	}
	if len(senders) == 0 {
		return nil, fmt.Errorf("%s: at least 1 sender is required", op)
	}

	return &Server{
		notifications: map[int]notification.Notification{},
		logger:        logger,
		senders:       senders,
	}, nil
}

func (s *Server) exportNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.exportNotification"

	if err := WriteAuditLog("audit.log", s.notifications); err != nil {
		s.logger.Error("export failed", "op", op, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]int{"exported": len(s.notifications)})
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	senders := map[string]sender.Sender{
		"console":  sender.LoggingSender{Sender: sender.NewConsoleSender(os.Stdout), Logger: logger},
		"email":    sender.LoggingSender{Sender: sender.NewEmailSender(os.Stdout), Logger: logger},
		"telegram": sender.LoggingSender{Sender: sender.TelegramSender{}, Logger: logger},
	}

	s, err := NewServer(logger, senders)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %s\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHanlder)
	mux.HandleFunc("GET /api/notifications/export", s.exportNotification)
	mux.HandleFunc("GET /api/notifications", s.listNotifications)
	mux.HandleFunc("GET /api/notifications/{id}", s.getNotification)
	mux.HandleFunc("POST /api/notifications", s.createNotification)

	logger.Info("starting :8080", appName, appVersion)
	http.ListenAndServe(":8080", s.logRequest(contentType(mux)))
}
