package markdown

import (
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/tituscheng/groktc/internal/prompt/webui"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	opts := Options{
		Model:  DefaultModel,
		Effort: DefaultReasoningEffort,
	}

	cmd := &cobra.Command{
		Use:          "markdown [filename]",
		Short:        "Convert raw transcript text files into Markdown",
		Long:         "Convert one transcript text file or all eligible top-level transcript text files into clean Markdown using xAI.",
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			runner := NewRunner()
			runner.PromptProvider = func(ctx context.Context) (GUIPromptResult, error) {
				var savedPrompts []SavedPrompt
				if cache, err := runner.CacheFactory(); err == nil {
					defer cache.Close()
					savedPrompts, _ = ListSavedPrompts(ctx, cache)
				}
				webuiPrompts := make([]webui.SavedPromptOption, 0, len(savedPrompts))
				for _, prompt := range savedPrompts {
					webuiPrompts = append(webuiPrompts, webui.SavedPromptOption{
						Name:    prompt.Name,
						Content: prompt.Content,
					})
				}

				result, err := webui.Prompt(ctx, "Markdown — Provide Instructions", "Pick or write the prompt that will be used to clean up your transcripts.", webuiPrompts)
				return GUIPromptResult{
					Text:      result.Text,
					Cache:     result.Cache,
					CacheName: result.CacheName,
					CacheMode: webuiCacheModeToMarkdown(result.CacheMode),
				}, err
			}
			if opts.JSON {
				runner.Stdout = io.Discard
				runner.Stderr = io.Discard
			}

			summary, err := runner.Run(cmd.Context(), opts)
			if opts.JSON {
				output := MarkdownJSONOutput{}
				for _, r := range summary.Processed {
					output.Results = append(output.Results, MarkdownJSONResult{
						InputPath:  r.InputPath,
						OutputPath: r.OutputPath,
						Status:     "processed",
					})
				}
				for _, r := range summary.Skipped {
					output.Results = append(output.Results, MarkdownJSONResult{
						InputPath:  r.InputPath,
						OutputPath: r.OutputPath,
						Status:     "skipped",
					})
				}
				for _, r := range summary.Failed {
					result := MarkdownJSONResult{
						InputPath:  r.InputPath,
						OutputPath: r.OutputPath,
						Status:     "failed",
					}
					if r.Err != nil {
						result.Error = r.Err.Error()
					}
					output.Results = append(output.Results, result)
				}
				output.Summary.Processed = len(summary.Processed)
				output.Summary.Skipped = len(summary.Skipped)
				output.Summary.Failed = len(summary.Failed)
				output.Summary.Total = len(output.Results)
				output.Summary.StartedAt = summary.StartedAt
				output.Summary.EndedAt = summary.EndedAt

				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if encErr := enc.Encode(output); encErr != nil {
					return encErr
				}
				// Swallow the aggregate error in JSON mode — details are in the JSON.
				return nil
			}
			return err
		},
	}

	cmd.Flags().StringVarP(&opts.Prompt, "prompt", "p", "", "prompt/instructions for transcript cleanup")
	cmd.Flags().StringVar(&opts.PromptFile, "prompt-file", "", "path to a file containing transcript cleanup instructions")
	cmd.Flags().BoolVar(&opts.NoGUI, "no-gui", false, "skip GUI prompt capture and fail if no prompt is supplied")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "overwrite existing non-empty Markdown files")
	cmd.Flags().StringVar(&opts.Model, "model", DefaultModel, "xAI model to use for transcript cleanup")
	cmd.Flags().StringVar(&opts.Effort, "effort", DefaultReasoningEffort, "reasoning effort to send to xAI")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output results as JSON to stdout")

	return cmd
}

func webuiCacheModeToMarkdown(m webui.CacheMode) CacheMode {
	switch m {
	case webui.CacheModeCreate:
		return CacheModeCreate
	case webui.CacheModeUpdate:
		return CacheModeUpdate
	default:
		return CacheModeNone
	}
}
