package estimate

import (
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	opts := Options{}

	cmd := &cobra.Command{
		Use:   "estimate [files...]",
		Short: "Estimate xAI STT transcription cost for audio/video files",
		Long: "Estimate xAI Speech-to-Text cost for one or more .mp3 or .mp4 files using " +
			"local ffprobe metadata. With no arguments, discovers media files in the current " +
			"directory. No API key is required.",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Args = args
			return NewRunner().Run(cmd.Context(), opts)
		},
	}

	cmd.Flags().BoolVar(&opts.Yes, "yes", false, "skip interactive confirmation when discovering files in the current directory")
	cmd.Flags().BoolVar(&opts.JSON, "json", false, "output results as JSON to stdout")

	return cmd
}

type Options struct {
	Args []string
	Yes  bool
	JSON bool
}
