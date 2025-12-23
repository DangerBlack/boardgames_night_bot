package hooks

import (
	"boardgame-night-bot/src/database"
	"boardgame-night-bot/src/models"
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

// WebhookClient wraps an HTTP client with configurable timeout and retry logic.
type WebhookClient struct {
	DB         *database.Database
	Client     *http.Client
	MaxAttempt int
}

// NewWebhookClient creates a new WebhookClient with the given timeout (TTL) and max attempts.
func NewWebhookClient(db *database.Database, timeout time.Duration, maxAttempt int) *WebhookClient {
	return &WebhookClient{
		DB:         db,
		Client:     &http.Client{Timeout: timeout},
		MaxAttempt: maxAttempt,
	}
}

func (wc *WebhookClient) SendAllWebhookAsync(ctx context.Context, chatID int64, payload models.HookWebhookEnvelope) {
	webhooks, err := wc.DB.GetWebhooksByChatID(chatID)
	if err != nil {
		return
	}

	for _, webhook := range webhooks {
		wc.SendWebhookAsync(ctx, webhook.Url, payload, webhook.Secret)
	}
}

// SendWebhookAsync dispatches the webhook event in a separate goroutine, signing the payload with the new signature scheme.
func (wc *WebhookClient) SendWebhookAsync(ctx context.Context, url string, payload models.HookWebhookEnvelope, secret string) {
	go func() {
		_ = wc.SendWebhookWithRetry(ctx, url, payload, secret)
	}()
}

// SendWebhookWithRetry sends the webhook event with retry logic, signing the payload with the new signature scheme.
func (wc *WebhookClient) SendWebhookWithRetry(ctx context.Context, url string, payload models.HookWebhookEnvelope, secret string) error {
	var lastErr error
	for attempt := 1; attempt <= wc.MaxAttempt; attempt++ {
		if err := wc.sendWebhook(ctx, url, payload, secret); err != nil {
			lastErr = err
			time.Sleep(time.Second * time.Duration(attempt)) // Exponential backoff
			continue
		}
		return nil
	}
	return lastErr
}

// sendWebhook performs the actual HTTP POST request, signing the payload with the new signature scheme.
func (wc *WebhookClient) sendWebhook(ctx context.Context, url string, payload models.HookWebhookEnvelope, secret string) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Compute content hash (SHA256, hex-encoded)
	contentHashBytes := sha256.Sum256(body)
	contentHash := hex.EncodeToString(contentHashBytes[:])

	// Use current UTC time in RFC1123 format for x-ms-date
	date := time.Now().UTC().Format(http.TimeFormat)

	// Build string to sign
	stringToSign := fmt.Sprintf("%s;%s", date, contentHash)

	// Compute HMAC signature (base64-encoded)
	signature := computeHMACBase64(stringToSign, []byte(secret))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-ms-date", date)
	req.Header.Set("x-ms-content-sha256", contentHash)
	req.Header.Set("X-BGNB-Signature", signature)

	resp, err := wc.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("webhook request failed with status: " + resp.Status)
	}
	return nil
}

// computeHMACBase64 generates the HMAC SHA256 signature for the stringToSign and encodes it in base64.
func computeHMACBase64(stringToSign string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
