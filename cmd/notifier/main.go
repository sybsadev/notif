package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"notifier/internal/notification"
	"notifier/internal/sender"
	"notifier/internal/store"
)

const (
	appName    = "notifr"
	appVersion = "0.1.0"
)

type Server struct {
	store   store.Store
	logger  *slog.Logger
	senders map[string]sender.Sender
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`{"status": "ok", "version":"` + appVersion + `"}`))
}

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	const op = "Server.listNotifications"

	notifications, err := s.store.GetAll()
	if err != nil {
		s.logger.Error("list failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if notifications == nil {
		notifications = []notification.Notification{}
	}

	json.NewEncoder(w).Encode(notifications)
}

func (s *Server) getNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.getNotification"

	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, fmt.Sprintf("%s: invalid id", op), http.StatusBadRequest)
		return
	}

	n, err := s.store.GetById(id)
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

	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if err := n.Validate(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id, err := s.store.Save(n)
	if err != nil {
		s.logger.Error("save failed", "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	n.ID = id
	n.Status = "pending"

	snd, ok := s.senders[n.Channel]
	if ok {
		if err := snd.Send(n); err != nil {
			s.logger.Error("send failed", "id", id, "err", err)
			_ = s.store.UpdateStatus(id, "failed")
			n.Status = "failed"
		} else {
			_ = s.store.UpdateStatus(id, "sent")
			n.Status = "sent"
		}
	}

	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(n)
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

func NewServer(st store.Store, logger *slog.Logger, senders map[string]sender.Sender) (*Server, error) {
	const op = "NewServer"

	if st == nil {
		return nil, fmt.Errorf("%s: store is required", op)
	}

	if logger == nil {
		return nil, fmt.Errorf("%s: logger is required", op)
	}

	if len(senders) == 0 {
		return nil, fmt.Errorf("%s: at least 1 sender is required", op)
	}

	return &Server{
		store:   st,
		logger:  logger,
		senders: senders,
	}, nil
}

func (s *Server) exportNotification(w http.ResponseWriter, r *http.Request) {
	const op = "Server.exportNotification"

	notifications, err := s.store.GetAll()
	if err != nil {
		s.logger.Error("export failed", "op", op, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	if err := WriteAuditLog("audit.log", notifications); err != nil {
		s.logger.Error("export failed", "op", op, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]int{"exported": len(notifications)})
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	const dsn = "host=localhost port=5432 user=postgres password=notifier dbname=postgres sslmode=disable"
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "sql.open: %s\n", err)
		os.Exit(1)
	}

	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "db ping failed: %s\n", err)
		os.Exit(1)
	}

	logger.Info("database connected")

	const createTable = `
	CREATE TABLE IF NOT EXISTS notifications (
		id SERIAL PRIMARY KEY,
		recipient TEXT NOT NULL,
		subject TEXT NOT NULL,
		body TEXT,
		channel TEXT,
		is_urgent BOOLEAN DEFAULT FALSE,
		status TEXT NOT NULL DEFAULT 'pending'
	)`

	if _, err := db.Exec(createTable); err != nil {
		fmt.Fprintf(os.Stderr, "create table: %s\n", err)
		os.Exit(1)
	}

	logger.Info("notifications table ready")

	senders := map[string]sender.Sender{
		"console":  sender.LoggingSender{Sender: sender.NewConsoleSender(os.Stdout), Logger: logger},
		"email":    sender.LoggingSender{Sender: sender.NewEmailSender(os.Stdout), Logger: logger},
		"telegram": sender.LoggingSender{Sender: sender.TelegramSender{}, Logger: logger},
	}

	store := store.NewPostgresStore(db)

	s, err := NewServer(store, logger, senders)
	if err != nil {
		fmt.Fprintf(os.Stderr, "server: %s\n", err)
		os.Exit(1)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("GET /api/notifications/export", s.exportNotification)
	mux.HandleFunc("GET /api/notifications", s.listNotifications)
	mux.HandleFunc("GET /api/notifications/{id}", s.getNotification)
	mux.HandleFunc("POST /api/notifications", s.createNotification)

	logger.Info("starting :8080", appName, appVersion)
	http.ListenAndServe(":8080", s.logRequest(contentType(mux)))
}
