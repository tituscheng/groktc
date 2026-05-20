package groktc

import (
	"strings"
	"testing"

	"github.com/tituscheng/groktc/internal/markdown"
	"github.com/stretchr/testify/assert"
)

func TestNewClient(t *testing.T) {
	c := NewClient("test-key")
	assert.NotNil(t, c)
}

func TestMarkdownOptions(t *testing.T) {
	tests := []struct {
		name     string
		opt      MarkdownOption
		wantModel string
		wantEffort string
		wantMaxTokens int
		wantTemp float32
	}{
		{
			name:     "WithModel",
			opt:      WithModel("grok-test"),
			wantModel: "grok-test",
		},
		{
			name:     "WithEffort",
			opt:      WithEffort("high"),
			wantEffort: "high",
		},
		{
			name:     "WithMaxTokens",
			opt:      WithMaxTokens(4096),
			wantMaxTokens: 4096,
		},
		{
			name:     "WithTemperature",
			opt:      WithTemperature(0.5),
			wantTemp: 0.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := markdown.NewProcessor(nil, "", "")
			tt.opt(p)

			if tt.wantModel != "" {
				assert.Equal(t, tt.wantModel, p.Model)
			}
			if tt.wantEffort != "" {
				assert.Equal(t, tt.wantEffort, p.Effort)
			}
			if tt.wantMaxTokens != 0 {
				assert.Equal(t, tt.wantMaxTokens, p.MaxTokens)
			}
			if tt.wantTemp != 0 {
				assert.Equal(t, tt.wantTemp, p.Temperature)
			}
		})
	}
}

func TestModelType(t *testing.T) {
	m := Model{
		ID:      "grok-4.3",
		Aliases: []string{"grok"},
		Version: "1.0",
	}
	assert.Equal(t, "grok-4.3", m.ID)
	assert.Equal(t, []string{"grok"}, m.Aliases)
}

func TestTranscriptionResultType(t *testing.T) {
	tr := TranscriptionResult{
		Text:     "hello world",
		Language: "en",
		Duration: 1.5,
		Words: []Word{
			{Text: "hello", Start: 0.0, End: 0.5},
			{Text: "world", Start: 0.6, End: 1.0},
		},
		VTT: "WEBVTT\n\n00:00:00.000 --> 00:00:01.000\nhello world\n\n",
	}
	assert.Equal(t, "hello world", tr.Text)
	assert.Equal(t, "en", tr.Language)
	assert.Equal(t, 1.5, tr.Duration)
	assert.Len(t, tr.Words, 2)
	assert.Equal(t, "hello", tr.Words[0].Text)
	assert.True(t, strings.HasPrefix(tr.VTT, "WEBVTT"))
}
