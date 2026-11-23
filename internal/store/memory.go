package store

import (
	"fmt"
	"notifier/internal/notification"
)

type MemoryStore struct {
	notifications map[int]notification.Notification
	nextID        int
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		notifications: make(map[int]notification.Notification),
	}
}

func (s *MemoryStore) Save(n notification.Notification) (int, error) {
	s.nextID++
	n.ID = s.nextID
	n.Status = "pending"
	s.notifications[n.ID] = n
	return n.ID, nil
}

func (s *MemoryStore) GetById(id int) (notification.Notification, error) {
	const op = "MemoryStore.GetById"

	n, ok := s.notifications[id]
	if !ok {
		return notification.Notification{}, fmt.Errorf("%s: %w", op, notification.ErrNotFound)
	}
	return n, nil
}

func (s *MemoryStore) UpdateStatus(id int, status string) error {
	const op = "MemoryStore.UpdateStatus"

	n, ok := s.notifications[id]
	if !ok {
		return fmt.Errorf("%s: %w", op, notification.ErrNotFound)
	}

	n.Status = status
	s.notifications[id] = n
	return nil
}

func (s *MemoryStore) GetAll() ([]notification.Notification, error) {
	result := make([]notification.Notification, 0, len(s.notifications))
	for _, n := range s.notifications {
		result = append(result, n)
	}

	return result, nil
}
