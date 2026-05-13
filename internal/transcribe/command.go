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
		Short: "Convert audio/video files (mp4, mp3) or URLs into plain text transcripts",
		Long: "Convert one or more audio/video files (.mp4, .mp3) or URLs into plain text " +
			"transcripts using xAI Speech-to-Text. With no arguments, discovers media files " +
			"in the current directory. URLs are downloaded as audio before transcription.",
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
	cmd.Flags().StringVarP(&opts.OutputPath, "output", "o", "", "custom output file path (default: <title>.<id>.txt for URLs, <basename>.txt for local files)")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output results as JSON to stdout")

	return cmd
}
