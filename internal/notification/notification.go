package notification

import "fmt"

type Notification struct {
	ID        int    `json:"id"`
	Recipient string `json:"to"`
	Subject   string `json:"subject"`
	Body      string `json:"body"`
	Channel   string `json:"channel"`
	IsUrgent  bool   `json:"urgent"`
	Status    string `json:"status"`
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
