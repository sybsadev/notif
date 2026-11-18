package sender

import (
	"bytes"
	"notifier/internal/notification"
	"testing"
)

func TestConsoleSender(t *testing.T) {
	var buf bytes.Buffer
	sender := NewConsoleSender(&buf)

	n := notification.Notification{
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

	n := notification.Notification{
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
