package agent

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"
)

const maximumUpstreamResponseBytes = 4096

type UploadStatus struct {
	UploadedBatches uint64     `json:"uploaded_batches"`
	UploadFailures  uint64     `json:"upload_failures"`
	LastAttemptAt   *time.Time `json:"last_attempt_at,omitempty"`
	LastSuccessAt   *time.Time `json:"last_success_at,omitempty"`
	LastError       string     `json:"last_error,omitempty"`
}

type Uploader struct {
	config Config
	outbox *Outbox
	client *http.Client
	logger *slog.Logger

	mu     sync.Mutex
	status UploadStatus
}

func NewUploader(config Config, outbox *Outbox, logger *slog.Logger) *Uploader {
	if logger == nil {
		logger = slog.Default()
	}
	return &Uploader{
		config: config,
		outbox: outbox,
		client: &http.Client{Timeout: config.RequestTimeout()},
		logger: logger,
	}
}

func (u *Uploader) Run(ctx context.Context) {
	retryDelay := u.config.MinimumRetry()
	for {
		processed, err := u.uploadNext(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			u.recordFailure(err)
			u.logger.Warn("upstream telemetry upload failed", "error", err, "retry_in", retryDelay)
			if !waitFor(ctx, retryDelay, nil) {
				return
			}
			retryDelay *= 2
			if retryDelay > u.config.MaximumRetry() {
				retryDelay = u.config.MaximumRetry()
			}
			continue
		}
		retryDelay = u.config.MinimumRetry()
		if processed {
			continue
		}
		if !waitFor(ctx, u.config.UploadInterval(), u.outbox.Wake()) {
			return
		}
	}
}

func (u *Uploader) Status() UploadStatus {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.status
}

func (u *Uploader) uploadNext(ctx context.Context) (bool, error) {
	item, ok := u.outbox.Peek()
	if !ok {
		return false, nil
	}
	payload, err := u.outbox.Read(item)
	if err != nil {
		return false, err
	}

	attemptedAt := time.Now().UTC()
	u.mu.Lock()
	u.status.LastAttemptAt = &attemptedAt
	u.mu.Unlock()

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, u.config.UpstreamEndpoint(), bytes.NewReader(payload))
	if err != nil {
		return false, fmt.Errorf("create upstream request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+u.config.UpstreamBearerToken)
	request.Header.Set("X-DayZ-Server-ID", u.config.ServerID)
	request.Header.Set("X-DayZ-Behaviour-Agent-Version", Version)

	response, err := u.client.Do(request)
	if err != nil {
		return false, fmt.Errorf("send upstream request: %w", err)
	}
	defer response.Body.Close()
	responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, maximumUpstreamResponseBytes))
	if readErr != nil {
		return false, fmt.Errorf("read upstream response: %w", readErr)
	}
	message := strings.TrimSpace(string(responseBody))

	if response.StatusCode >= 200 && response.StatusCode < 300 {
		if err := u.outbox.Remove(item); err != nil {
			return false, err
		}
		u.recordSuccess()
		return true, nil
	}
	if isPermanentUpstreamRejection(response.StatusCode) {
		reason := fmt.Sprintf("upstream returned %d", response.StatusCode)
		if message != "" {
			reason += ": " + message
		}
		if err := u.outbox.DeadLetter(item, reason); err != nil {
			return false, err
		}
		u.recordPermanentFailure(reason)
		u.logger.Error("upstream permanently rejected telemetry batch", "status", response.StatusCode, "response", message)
		return true, nil
	}
	if message == "" {
		message = http.StatusText(response.StatusCode)
	}
	return false, fmt.Errorf("upstream returned %d: %s", response.StatusCode, message)
}

func (u *Uploader) recordSuccess() {
	now := time.Now().UTC()
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status.UploadedBatches++
	u.status.LastSuccessAt = &now
	u.status.LastError = ""
}

func (u *Uploader) recordFailure(err error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status.UploadFailures++
	u.status.LastError = err.Error()
}

func (u *Uploader) recordPermanentFailure(reason string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.status.UploadFailures++
	u.status.LastError = reason
}

func isPermanentUpstreamRejection(status int) bool {
	switch status {
	case http.StatusBadRequest, http.StatusConflict, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		return true
	default:
		return false
	}
}

func waitFor(ctx context.Context, delay time.Duration, wake <-chan struct{}) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-wake:
		return true
	case <-timer.C:
		return true
	}
}
