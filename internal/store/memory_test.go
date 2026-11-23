package store

import (
	"testing"

	"notifier/internal/notification"
)

func TestMemoryStore(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T, s *MemoryStore)
	}{
		{
			name: "save and get by ID",
			run: func(t *testing.T, s *MemoryStore) {
				id, err := s.Save(notification.Notification{
					Recipient: "ops@example.com",
					Subject:   "Deploy done",
					Channel:   "email",
				})
				if err != nil {
					t.Fatalf("Save() error: %s", err)
				}
				if id != 1 {
					t.Errorf("Save() id = %d, want 1", id)
				}

				got, err := s.GetById(id)
				if err != nil {
					t.Fatalf("GetByID() error: %s", err)
				}
				if got.Recipient != "ops@example.com" {
					t.Errorf("Recipient = %q, want %q", got.Recipient, "ops@example.com")
				}
				if got.Status != "pending" {
					t.Errorf("Status = %q, want %q", got.Status, "pending")
				}
			},
		},
		{
			name: "get invalid ID",
			run: func(t *testing.T, s *MemoryStore) {
				_, err := s.GetById(999)
				if err == nil {
					t.Fatal("expected error, got nil")
				}
			},
		},
		{
			name: "status update",
			run: func(t *testing.T, s *MemoryStore) {
				id, _ := s.Save(notification.Notification{
					Recipient: "admin@company.com",
					Subject:   "Alert",
				})

				if err := s.UpdateStatus(id, "sent"); err != nil {
					t.Fatalf("UpdateStatus() error: %s", err)
				}

				got, _ := s.GetById(id)
				if got.Status != "sent" {
					t.Errorf("Status = %q, want %q", got.Status, "sent")
				}
			},
		},
		{
			name: "get all notifications",
			run: func(t *testing.T, s *MemoryStore) {
				s.Save(notification.Notification{Recipient: "a@example.com"})
				s.Save(notification.Notification{Recipient: "b@example.com"})

				all, err := s.GetAll()
				if err != nil {
					t.Fatalf("GetAll() error: %s", err)
				}
				if len(all) != 2 {
					t.Errorf("len = %d, want 2", len(all))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewMemoryStore()
			tt.run(t, s)
		})
	}
}
