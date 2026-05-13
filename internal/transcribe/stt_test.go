package transcribe

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHTTPSTTClientSuccess(t *testing.T) {
	want := STTResponse{Text: "hello world", Language: "English", Duration: 2.5}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "Bearer testkey", r.Header.Get("Authorization"))

		// Verify multipart field name is "file".
		ct := r.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(ct)
		require.NoError(t, err)
		require.Equal(t, "multipart/form-data", mediaType)

		mr := multipart.NewReader(r.Body, params["boundary"])
		part, err := mr.NextPart()
		require.NoError(t, err)
		require.Equal(t, "file", part.FormName())

		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(want))
	}))
	defer srv.Close()

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "audio.mp3")
	require.NoError(t, os.WriteFile(audioPath, []byte("fake audio data"), 0o644))

	client := &HTTPSTTClient{
		APIKey:     "testkey",
		HTTPClient: srv.Client(),
		Endpoint:   srv.URL,
	}

	got, err := client.Transcribe(context.Background(), audioPath)
	require.NoError(t, err)
	require.Equal(t, want.Text, got.Text)
	require.Equal(t, want.Language, got.Language)
	require.InDelta(t, want.Duration, got.Duration, 0.001)
}

func TestHTTPSTTClientRetriesOn500(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Drain body so connection can be reused.
		io.Copy(io.Discard, r.Body)
		attempts++
		if attempts < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(STTResponse{Text: "success"})
	}))
	defer srv.Close()

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "audio.mp3")
	require.NoError(t, os.WriteFile(audioPath, []byte("data"), 0o644))

	client := &HTTPSTTClient{
		APIKey:     "testkey",
		HTTPClient: srv.Client(),
		Endpoint:   srv.URL,
	}

	got, err := client.Transcribe(context.Background(), audioPath)
	require.NoError(t, err)
	require.Equal(t, "success", got.Text)
	require.GreaterOrEqual(t, attempts, 2)
}

func TestHTTPSTTClientDoesNotRetryOn400(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		attempts++
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	audioPath := filepath.Join(dir, "audio.mp3")
	require.NoError(t, os.WriteFile(audioPath, []byte("data"), 0o644))

	client := &HTTPSTTClient{
		APIKey:     "testkey",
		HTTPClient: srv.Client(),
		Endpoint:   srv.URL,
	}

	_, err := client.Transcribe(context.Background(), audioPath)
	require.Error(t, err)
	require.Equal(t, 1, attempts)
}

func TestHTTPSTTClientMissingFile(t *testing.T) {
	client := &HTTPSTTClient{
		APIKey:   "testkey",
		Endpoint: "http://localhost:99999",
	}
	_, err := client.Transcribe(context.Background(), "/no/such/file.mp3")
	require.Error(t, err)
}

func TestHTTPSTTClientEmptyAPIKey(t *testing.T) {
	client := NewHTTPSTTClient("   ")
	dir := t.TempDir()
	audioPath := filepath.Join(dir, "a.mp3")
	require.NoError(t, os.WriteFile(audioPath, []byte("x"), 0o644))
	_, err := client.Transcribe(context.Background(), audioPath)
	require.Error(t, err)
	require.Contains(t, err.Error(), "API key")
}

func TestIsSttRetryable(t *testing.T) {
	require.True(t, isSttRetryable(&sttAPIError{StatusCode: 500}))
	require.True(t, isSttRetryable(&sttAPIError{StatusCode: 429}))
	require.False(t, isSttRetryable(&sttAPIError{StatusCode: 400}))
	require.False(t, isSttRetryable(&sttAPIError{StatusCode: 200}))
	require.False(t, isSttRetryable(nil))
}
