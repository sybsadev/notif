package sender

import (
	"fmt"
	"io"
	"log/slog"
	"notifier/internal/notification"
	"os"
)

// For testiing
type MockSender struct {
	Calls []notification.Notification
}

func (m *MockSender) Send(n notification.Notification) error {
	m.Calls = append(m.Calls, n)
	return nil
}

type Sender interface {
	Send(notification.Notification) error
}

type LoggingSender struct {
	Sender
	Logger *slog.Logger
}

type ConsoleSender struct {
	w io.Writer
}
type EmailSender struct {
	w io.Writer
}
type TelegramSender struct{}

func (l LoggingSender) Send(n notification.Notification) error {
	l.Logger.Info("sending", "to", n.Recipient, "channel", n.Channel)

	err := l.Sender.Send(n)
	if err != nil {
		l.Logger.Error("send failed", "to", n.Recipient, "error", err)
		return err
	}

	l.Logger.Info("sent", "to", n.Recipient, "channel", n.Channel)
	return nil
}

func NewConsoleSender(w io.Writer) *ConsoleSender {
	if w == nil {
		w = os.Stdout
	}

	return &ConsoleSender{w: w}
}

func (c ConsoleSender) Send(n notification.Notification) error {
	_, err := fmt.Fprintf(c.w, "[console] to: %s | subject: %s\n", n.Recipient, n.Subject)
	return err
}

func NewEmailSender(w io.Writer) *EmailSender {
	if w == nil {
		w = os.Stdout
	}

	return &EmailSender{w: w}
}

func (c EmailSender) Send(n notification.Notification) error {
	_, err := fmt.Fprintf(c.w, "[email] to: %s | subject: %s\n", n.Recipient, n.Subject)
	return err
}

func (c TelegramSender) Send(n notification.Notification) error {
	fmt.Printf("[telegram] to: %s | subject: %s\n", n.Recipient, n.Subject)
	return nil
}
