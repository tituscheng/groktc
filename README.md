# groktc

A Go CLI tool and library for working with the **xAI API** (Grok). Convert transcripts to Markdown, transcribe audio/video, estimate token costs, and manage model selection — from the terminal or from your own Go code.

---

## Installation

### As a CLI tool

```bash
go install github.com/example/groktc@latest
```

### As a Go library

```bash
go get github.com/example/groktc/pkg/groktc
```

> **Note:** Replace `github.com/example/groktc` with your actual module path before publishing.

---

## Configuration

All commands and library calls require an xAI API key.

```bash
export XAI_API_KEY="your-api-key-here"
```

---

## CLI Usage

### `groktc markdown` — Convert transcripts to Markdown

```bash
# Clean up a single transcript file
groktc markdown transcript.txt

# Process all .txt files in the current directory
groktc markdown

# Use a custom prompt
groktc markdown transcript.txt -p "Fix spelling and format as bullet points"

# Read prompt from a file
groktc markdown transcript.txt --prompt-file instructions.txt

# Skip the GUI and fail if no prompt is given
groktc markdown transcript.txt --no-gui

# Output results as JSON
groktc markdown transcript.txt --json
```

### `groktc transcribe` — Transcribe audio/video

Each run writes **two files** from a single Speech-to-Text call: a plain-text
transcript (`.txt`) and a WebVTT subtitle file (`.vtt`) with word-level timing.

```bash
# Transcribe an audio file -> audio.txt + audio.vtt
groktc transcribe audio.mp3

# Transcribe a video (ffmpeg extracts audio automatically) -> video.txt + video.vtt
groktc transcribe video.mp4

# Transcribe a YouTube URL -> <title>.<id>.txt + <title>.<id>.vtt
groktc transcribe "https://youtube.com/watch?v=..."

# Custom output base path -> notes.txt + notes.vtt (extension is swapped per format)
groktc transcribe audio.mp3 --output notes.txt

# Force overwrite existing transcripts
groktc transcribe audio.mp3 --force
```

> A file is skipped only when **both** its `.txt` and `.vtt` already exist; if
> either is missing the file is re-transcribed. Use `--force` to always overwrite.

### `groktc tokenize` — Estimate token usage and cost

```bash
# Analyze a single file
groktc tokenize essay.txt

# Analyze all text files in the current directory
groktc tokenize

# Use a specific model for pricing
groktc tokenize essay.txt --model grok-4.3
```

### `groktc model` — Manage xAI models

```bash
# List available models
groktc model

# Set a default model
groktc model --set grok-4.3
```

### `groktc prompt` — Manage saved prompts (GUI)

```bash
# Open the webview prompt manager
groktc prompt
```

---

## Library Usage

Import the high-level client:

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/example/groktc/pkg/groktc"
)

func main() {
    ctx := context.Background()
    client := groktc.NewClient(os.Getenv("XAI_API_KEY"))

    // Convert raw text to Markdown
    md, err := client.ConvertToMarkdown(ctx,
        "Speaker 1: uh, so today we're gonna talk about Go...",
        "Clean up formatting and fix speaker labels",
        groktc.WithModel("grok-4.3"),
        groktc.WithEffort("medium"),
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(md)
}
```

### Transcription

A single call returns the transcript, per-word timings, and a ready-to-write
WebVTT document:

```go
result, err := client.TranscribeAudio(ctx, "interview.mp3")
if err != nil {
    log.Fatal(err)
}
fmt.Println("Language:", result.Language)
fmt.Println("Duration:", result.Duration)
fmt.Println("Text:", result.Text)

// Write subtitles straight to disk.
if err := os.WriteFile("interview.vtt", []byte(result.VTT), 0o644); err != nil {
    log.Fatal(err)
}

// Or build your own output from the raw word timings.
for _, w := range result.Words {
    fmt.Printf("[%.2f-%.2f] %s\n", w.Start, w.End, w.Text)
}
```

### Token counting

```go
count, err := client.CountTokens(ctx, "grok-4.3", "Hello, world!")
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Tokens: %d\n", count)
```

### Model catalog

```go
models, err := client.ListModels(ctx)
if err != nil {
    log.Fatal(err)
}
for _, m := range models {
    fmt.Println(m.ID, m.Version)
}
```

---

## Low-level packages

For advanced use cases, import individual packages directly:

| Package | Path | Purpose |
|---------|------|---------|
| `xai` | `pkg/xai` | xAI HTTP client (tokenizer, catalog, chat) |
| `runner` | `pkg/runner` | Generic concurrent batch processor |
| `retry` | `pkg/retry` | Exponential backoff retry logic |
| `filediscovery` | `pkg/filediscovery` | Text file discovery and UTF-8 sniffing |
| `fileutil` | `pkg/fileutil` | File utility helpers |

---

## Requirements

- **Go 1.26.2** or later
- **xAI API key** ([x.ai](https://x.ai))
- **ffmpeg** (optional, for MP4 and YouTube URL transcription)

---

## Project Layout

```
groktc/
├── cmd/            # Cobra CLI commands
├── internal/       # Private implementation (not importable externally)
│   ├── markdown/   # Markdown conversion engine
│   ├── model/      # Model catalog and selection
│   ├── tokenize/   # Token estimation
│   ├── transcribe/ # Audio/video transcription pipeline
│   └── ...
├── pkg/            # Public API (importable by other programs)
│   ├── groktc/     # High-level facade client
│   ├── xai/        # xAI API client
│   ├── runner/     # Batch processor
│   └── ...
└── main.go         # CLI entry point
```

---

## License

[Your license here]
