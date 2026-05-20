package transcribe

import (
	"encoding/json"
	"os"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	opts := Options{}

	cmd := &cobra.Command{
		Use:   "transcribe [file|url ...]",
		Short: "Transcribe audio/video files (mp4, mp3) or URLs into text and VTT subtitles",
		Long: "Convert one or more audio/video files (.mp4, .mp3) or URLs into a plain-text " +
			"transcript (.txt) and a WebVTT subtitle file (.vtt) using xAI Speech-to-Text. " +
			"Both files are written from a single transcription call. With no arguments, " +
			"discovers media files in the current directory. URLs are downloaded as audio " +
			"before transcription.",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			runner := NewRunner()

			// In JSON mode, suppress animated progress output.
			if opts.JSON {
				runner.Stdout = os.Stdout // keep for possible future use, but runner writes to this
			}

			result, err := runner.Run(cmd.Context(), opts)
			if opts.JSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				if encErr := enc.Encode(result.Tasks); encErr != nil {
					return encErr
				}
				// Swallow the aggregate error in JSON mode — details are in the JSON array.
				return nil
			}
			return err
		},
	}

	cmd.Flags().BoolVarP(&opts.Force, "force", "f", false, "overwrite existing non-empty transcript files")
	cmd.Flags().StringVarP(&opts.OutputPath, "output", "o", "", "custom output base path; .txt and .vtt are written by swapping the extension (default: <title>.<id> for URLs, <basename> for local files)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output results as JSON to stdout")

	return cmd
}
