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
	"log"
	"net/http"
	"time"

	"github.com/bluele/gcache"
)

// WebhookClient wraps an HTTP client with configurable timeout and retry logic.
type WebhookClient struct {
	DB                 *database.Database
	Client             *http.Client
	MaxAttempt         int
	FailureCache       gcache.Cache
	MaxFailuresAttempt int
}

// NewWebhookClient creates a new WebhookClient with the given timeout (TTL) and max attempts.
func NewWebhookClient(db *database.Database, timeout time.Duration, maxAttempt int, failureExpiration time.Duration, maxFailureAttempt int) *WebhookClient {
	failureCache := gcache.New(1000).LRU().Expiration(failureExpiration).Build()
	return &WebhookClient{
		DB:                 db,
		Client:             &http.Client{Timeout: timeout},
		MaxAttempt:         maxAttempt,
		FailureCache:       failureCache,
		MaxFailuresAttempt: 5,
	}
}

func (wc *WebhookClient) SendAllWebhookAsync(ctx context.Context, chatID int64, payload models.HookWebhookEnvelope) {
	webhooks, err := wc.DB.GetWebhooksByChatID(chatID)
	if err != nil {
		return
	}

	for _, webhook := range webhooks {
		wc.SendWebhookAsync(ctx, chatID, webhook, payload, webhook.Secret)
	}
}

// SendWebhookAsync dispatches the webhook event in a separate goroutine, signing the payload with the new signature scheme.
func (wc *WebhookClient) SendWebhookAsync(ctx context.Context, chatID int64, w models.Webhook, payload models.HookWebhookEnvelope, secret string) {
	go func() {
		_ = wc.SendWebhookWithRetry(ctx, chatID, w, payload, secret)
	}()
}

// SendWebhookWithRetry sends the webhook event with retry logic, signing the payload with the new signature scheme.
func (wc *WebhookClient) SendWebhookWithRetry(ctx context.Context, chatID int64, w models.Webhook, payload models.HookWebhookEnvelope, secret string) error {
	var lastErr error
	for attempt := 1; attempt <= wc.MaxAttempt; attempt++ {
		log.Printf("In chat %d, attempt %d to send webhook to %s", chatID, attempt, w.Url)
		if err := wc.sendWebhook(ctx, w, payload, secret); err != nil {
			lastErr = err
			time.Sleep(time.Second * time.Duration(attempt)) // Exponential backoff
			continue
		}
		return nil
	}
	return lastErr
}

func (wc *WebhookClient) registerFailure(webhookID string) {
	count := 0
	if val, err := wc.FailureCache.Get(webhookID); err == nil {
		count = val.(int)
	}
	if err := wc.FailureCache.Set(webhookID, count+1); err != nil {
		log.Printf("Failed to set failure count for %s: %v", webhookID, err)
	}
}

func (wc *WebhookClient) shouldDiscard(webhookID string) bool {
	val, err := wc.FailureCache.Get(webhookID)
	if err != nil {
		return false
	}
	log.Printf("Webhook %s has %d failures", webhookID, val.(int))
	count := val.(int)
	return count >= wc.MaxFailuresAttempt
}

// sendWebhook performs the actual HTTP POST request, signing the payload with the new signature scheme.
func (wc *WebhookClient) sendWebhook(ctx context.Context, w models.Webhook, payload models.HookWebhookEnvelope, secret string) error {
	if wc.shouldDiscard(w.UUID) {
		log.Printf("Discarding webhook %s due to repeated failures", w.UUID)
		return errors.New("webhook discarded due to repeated failures")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		wc.registerFailure(w.UUID)
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
	signature := ComputeHMACBase64(stringToSign, []byte(secret))

	// Prepare request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.Url, bytes.NewReader(body))
	if err != nil {
		wc.registerFailure(w.UUID)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-ms-date", date)
	req.Header.Set("x-ms-content-sha256", contentHash)
	req.Header.Set("X-BGNB-Signature", signature)

	resp, err := wc.Client.Do(req)
	if err != nil {
		wc.registerFailure(w.UUID)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		wc.registerFailure(w.UUID)
		return errors.New("webhook request failed with status: " + resp.Status)
	}
	return nil
}

// ComputeHMACBase64 generates the HMAC SHA256 signature for the stringToSign and encodes it in base64.
func ComputeHMACBase64(stringToSign string, secret []byte) string {
	h := hmac.New(sha256.New, secret)
	h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
