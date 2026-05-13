package xai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestClientCountTokens(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tokenize-text" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}

		var payload map[string]string
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "grok-4.3" {
			t.Fatalf("unexpected model: %s", payload["model"])
		}
		if payload["text"] != "hello" {
			t.Fatalf("unexpected text: %s", payload["text"])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"token_ids": []int{1, 2, 3},
		})
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		APIKey:     "test-key",
		HTTPClient: server.Client(),
	}

	got, err := client.CountTokens(context.Background(), "grok-4.3", "hello")
	if err != nil {
		t.Fatalf("CountTokens returned error: %v", err)
	}
	if got != 3 {
		t.Fatalf("expected 3 tokens, got %d", got)
	}
}

func TestClientCountTokensObjectPayload(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token_ids": []map[string]any{
				{"token_id": 1, "string_token": "Hel"},
				{"token_id": 2, "string_token": "lo"},
			},
		})
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		APIKey:     "test-key",
		HTTPClient: server.Client(),
	}

	got, err := client.CountTokens(context.Background(), "grok-4.3", "Hello")
	if err != nil {
		t.Fatalf("CountTokens returned error: %v", err)
	}
	if got != 2 {
		t.Fatalf("expected 2 tokens, got %d", got)
	}
}

func TestClientCountTokensAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "bad request"},
		})
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		APIKey:     "test-key",
		HTTPClient: server.Client(),
	}

	_, err := client.CountTokens(context.Background(), "grok-4.3", "hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "bad request") {
		t.Fatalf("expected API error message, got %v", err)
	}
}

func TestClientListLanguageModels(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/language-models" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("unexpected auth header: %s", got)
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{
					"id":                          "grok-4.3",
					"aliases":                     []string{"grok-4.3-latest", "grok-4.3"},
					"input_modalities":            []string{"text"},
					"output_modalities":           []string{"text"},
					"version":                     "2026-05-01",
					"prompt_text_token_price":     "1.25",
					"completion_text_token_price": "2.50",
				},
				{
					"name": "grok-4.20-reasoning",
					"pricing": map[string]any{
						"prompt_text_token_price":     1.1,
						"completion_text_token_price": 2.2,
					},
				},
			},
		})
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		APIKey:     "test-key",
		HTTPClient: server.Client(),
	}

	models, err := client.ListLanguageModels(context.Background())
	if err != nil {
		t.Fatalf("ListLanguageModels returned error: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}

	if models[0].ID != "grok-4.20-reasoning" {
		t.Fatalf("expected sorted model IDs, got first %q", models[0].ID)
	}
	if models[1].ID != "grok-4.3" {
		t.Fatalf("expected second model to be grok-4.3, got %q", models[1].ID)
	}

	if !reflect.DeepEqual(models[1].InputModalities, []string{"text"}) {
		t.Fatalf("unexpected modalities: %v", models[1].InputModalities)
	}
	if models[0].PromptTextTokenPriceRaw != "1.1" {
		t.Fatalf("expected nested prompt price, got %q", models[0].PromptTextTokenPriceRaw)
	}
	if models[0].CompletionTextTokenPriceRaw != "2.2" {
		t.Fatalf("expected nested completion price, got %q", models[0].CompletionTextTokenPriceRaw)
	}
}

func TestClientListLanguageModelsAPIError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]string{"message": "unauthorized"},
		})
	}))
	defer server.Close()

	client := &Client{
		BaseURL:    server.URL,
		APIKey:     "test-key",
		HTTPClient: server.Client(),
	}

	_, err := client.ListLanguageModels(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "unauthorized") {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}
