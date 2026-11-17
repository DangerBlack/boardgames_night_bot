package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Email struct {
	To      string
	Subject string
	HTML    string
	Text    string
}

type Mailer interface {
	Send(ctx context.Context, email Email) error
}

// ResendMailer sends email through the Resend HTTP API (https://resend.com/docs/api-reference/emails/send-email).
type ResendMailer struct {
	APIKey   string
	From     string
	Endpoint string
	Client   *http.Client
}

func NewResendMailer(apiKey, from string) *ResendMailer {
	return &ResendMailer{
		APIKey:   apiKey,
		From:     from,
		Endpoint: "https://api.resend.com/emails",
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (m *ResendMailer) Send(ctx context.Context, email Email) error {
	body, err := json.Marshal(map[string]any{
		"from":    m.From,
		"to":      []string{email.To},
		"subject": email.Subject,
		"html":    email.HTML,
		"text":    email.Text,
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+m.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := m.Client.Do(req)
	if err != nil {
		return fmt.Errorf("resend: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("resend: status %d: %s", resp.StatusCode, msg)
	}
	return nil
}
