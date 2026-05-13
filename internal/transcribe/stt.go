package transcribe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tituscheng/groktc/pkg/retry"
)

type STTClient interface {
	Transcribe(ctx context.Context, audioPath string) (STTResponse, error)
}

type HTTPSTTClient struct {
	APIKey     string
	HTTPClient *http.Client
	Endpoint   string
}

func NewHTTPSTTClient(apiKey string) *HTTPSTTClient {
	return &HTTPSTTClient{
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 10 * time.Minute},
		Endpoint:   STTEndpoint,
	}
}

func (c *HTTPSTTClient) Transcribe(ctx context.Context, audioPath string) (STTResponse, error) {
	if strings.TrimSpace(c.APIKey) == "" {
		return STTResponse{}, fmt.Errorf("STT client: API key is required")
	}

	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		resp, err := c.doRequest(ctx, audioPath)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isSttRetryable(err) || attempt == 3 {
			break
		}
		slog.Warn("STT attempt failed, retrying",
			"attempt", attempt,
			"max_attempts", 3,
			"audio_path", audioPath,
			"error", err,
		)
		if sleepErr := retry.Sleep(ctx, retry.Backoff(250*time.Millisecond, attempt)); sleepErr != nil {
			return STTResponse{}, sleepErr
		}
	}
	slog.Error("STT transcribe failed after retries",
		"audio_path", audioPath,
		"error", lastErr,
	)
	return STTResponse{}, fmt.Errorf("STT transcribe %q: %w", filepath.Base(audioPath), lastErr)
}

func (c *HTTPSTTClient) doRequest(ctx context.Context, audioPath string) (STTResponse, error) {
	f, err := os.Open(audioPath)
	if err != nil {
		return STTResponse{}, fmt.Errorf("open audio file: %w", err)
	}
	defer f.Close()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filepath.Base(audioPath))
	if err != nil {
		return STTResponse{}, fmt.Errorf("create form file: %w", err)
	}
	if _, err := io.Copy(fw, f); err != nil {
		return STTResponse{}, fmt.Errorf("copy audio to form: %w", err)
	}
	if err := mw.Close(); err != nil {
		return STTResponse{}, fmt.Errorf("close multipart writer: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, &buf)
	if err != nil {
		return STTResponse{}, fmt.Errorf("build STT request: %w", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return STTResponse{}, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return STTResponse{}, fmt.Errorf("read STT response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return STTResponse{}, &sttAPIError{
			StatusCode: res.StatusCode,
			Body:       string(body),
		}
	}

	var out STTResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return STTResponse{}, fmt.Errorf("decode STT response: %w", err)
	}
	return out, nil
}

type sttAPIError struct {
	StatusCode int
	Body       string
}

func (e *sttAPIError) Error() string {
	return fmt.Sprintf("STT API error %d: %s", e.StatusCode, e.Body)
}

func isSttRetryable(err error) bool {
	if err == nil {
		return false
	}
	var apiErr *sttAPIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429 || apiErr.StatusCode >= 500
	}
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout()
}


