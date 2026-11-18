package notification

import (
	"testing"
)

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
