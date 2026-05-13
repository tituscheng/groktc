package tokenize

import (
	"context"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"text/tabwriter"

	"github.com/tituscheng/groktc/internal/confirm"
	"github.com/tituscheng/groktc/pkg/filediscovery"
	"github.com/tituscheng/groktc/internal/model"
	"github.com/tituscheng/groktc/pkg/xai"

	"golang.org/x/sync/errgroup"
)

const (
	envAPIKey           = "XAI_API_KEY"
	defaultConcurrency  = 4
	defaultModelID      = model.DefaultModelID
	estimateWarningNote = "Note: Input tokens are estimated via tokenizer, and output cost uses a best-guess output token estimate. Actual billed usage may differ."
	outputGuessRatio    = 0.35
	minOutputTokens     = 64
)

type Options struct {
	Args          []string
	Model         string
	ModelExplicit bool
	Yes           bool
}

type Runner struct {
	Client          xai.TokenizerClient
	Stdout          io.Writer
	Stderr          io.Writer
	Stdin           io.Reader
	LookupEnv       func(string) string
	TerminalChecker func() bool
	Prompt          func(io.Reader, io.Writer, []string) (bool, error)
	StoreFactory    func() (model.Store, error)
}

type fileResult struct {
	Path       string
	Bytes      int
	TokenCount int
	CostUSD    float64
}

func NewRunner() *Runner {
	apiKey := os.Getenv(envAPIKey)
	return &Runner{
		Client: xai.NewClient(apiKey),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Stdin:  os.Stdin,
		LookupEnv: func(key string) string {
			return os.Getenv(key)
		},
		TerminalChecker: isTerminal,
		Prompt:          confirm.Prompt,
		StoreFactory: func() (model.Store, error) {
			return model.OpenDefaultStore(os.UserHomeDir)
		},
	}
}

func (r *Runner) Run(ctx context.Context, opts Options) error {
	if strings.TrimSpace(r.LookupEnv(envAPIKey)) == "" {
		return fmt.Errorf("%s is required", envAPIKey)
	}

	var store model.Store
	if r.StoreFactory != nil {
		resolvedStore, err := r.StoreFactory()
		if err == nil {
			store = resolvedStore
			defer store.Close()
		}
	}

	resolver := model.Resolver{
		Store:        store,
		DefaultModel: defaultModelID,
	}

	resolvedModel, err := resolver.ResolveForTokenize(ctx, opts.Model, opts.ModelExplicit)
	if err != nil {
		return err
	}

	for _, warning := range resolvedModel.Warnings {
		fmt.Fprintln(r.Stderr, warning)
	}

	pricing, err := r.resolvePricing(ctx, store, resolvedModel.ModelID)
	if err != nil {
		return err
	}

	files, err := r.resolveFiles(opts)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no text files were selected for analysis")
	}

	results, err := r.tokenizeFiles(ctx, files, resolvedModel.ModelID, pricing)
	if err != nil {
		return err
	}

	r.renderReport(resolvedModel.ModelID, pricing, results)
	return nil
}

func (r *Runner) resolvePricing(ctx context.Context, store model.Store, modelID string) (xai.Pricing, error) {
	pricing, err := xai.LookupPricing(modelID)
	if err == nil {
		return pricing, nil
	}

	if store != nil {
		cachedModel, found, modelErr := store.GetModelByID(ctx, modelID)
		if modelErr != nil {
			return xai.Pricing{}, modelErr
		}
		if found {
			pricing := xai.Pricing{}
			if parsed, ok := model.ParseInputUSDPerMillion(cachedModel.PromptTextTokenPriceRaw); ok {
				pricing.InputPerMillionUSD = parsed
			}
			if parsed, ok := model.ParseOutputUSDPerMillion(cachedModel.CompletionTextTokenPriceRaw); ok {
				pricing.OutputPerMillionUSD = parsed
			}
			if pricing.InputPerMillionUSD > 0 {
				if pricing.OutputPerMillionUSD <= 0 {
					pricing.OutputPerMillionUSD = pricing.InputPerMillionUSD * 2
				}
				return pricing, nil
			}
		}
	}

	return xai.Pricing{}, fmt.Errorf("no pricing is configured for model %q", modelID)
}

func (r *Runner) resolveFiles(opts Options) ([]string, error) {
	if len(opts.Args) > 0 {
		return filediscovery.ValidateExplicitFiles(opts.Args)
	}

	files, err := filediscovery.DiscoverTextFiles(".")
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no text files were found in %s", filepath.Clean("."))
	}

	if opts.Yes {
		return files, nil
	}

	if !r.TerminalChecker() {
		fmt.Fprintln(r.Stdout, "Discovered text files:")
		for _, file := range files {
			fmt.Fprintf(r.Stdout, "  - %s\n", file)
		}
		return nil, fmt.Errorf("interactive confirmation requires a TTY; rerun with explicit file paths or --yes")
	}

	confirmed, err := r.Prompt(r.Stdin, r.Stdout, files)
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return nil, fmt.Errorf("tokenization canceled")
	}

	return files, nil
}

func (r *Runner) tokenizeFiles(ctx context.Context, files []string, model string, pricing xai.Pricing) ([]fileResult, error) {
	results := make([]fileResult, 0, len(files))
	var (
		mu sync.Mutex
		g  errgroup.Group
	)
	g.SetLimit(defaultConcurrency)

	for _, path := range files {
		path := path
		g.Go(func() error {
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read file %q: %w", path, err)
			}

			tokens, err := r.Client.CountTokens(ctx, model, string(data))
			if err != nil {
				return fmt.Errorf("tokenize file %q: %w", path, err)
			}

			result := fileResult{
				Path:       path,
				Bytes:      len(data),
				TokenCount: tokens,
				CostUSD:    xai.EstimateInputCostUSD(tokens, pricing),
			}

			mu.Lock()
			results = append(results, result)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Path < results[j].Path
	})

	return results, nil
}

func (r *Runner) renderReport(model string, pricing xai.Pricing, results []fileResult) {
	var totalBytes int
	var totalTokens int
	var totalInputCost float64

	tw := tabwriter.NewWriter(r.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(
		tw,
		"%s\t%s\t%s\t%s\n",
		tokenizeColorHeader("FILE"),
		tokenizeColorHeader("BYTES"),
		tokenizeColorHeader("TOKENS"),
		tokenizeColorHeader("ESTIMATED INPUT COST"),
	)
	for _, result := range results {
		totalBytes += result.Bytes
		totalTokens += result.TokenCount
		totalInputCost += result.CostUSD
		fmt.Fprintf(
			tw,
			"%s\t%d\t%d\t%s\n",
			result.Path,
			result.Bytes,
			result.TokenCount,
			tokenizeColorValue(formatUSD(result.CostUSD)),
		)
	}
	_ = tw.Flush()

	if pricing.OutputPerMillionUSD <= 0 && pricing.InputPerMillionUSD > 0 {
		pricing.OutputPerMillionUSD = pricing.InputPerMillionUSD * 2
	}
	estimatedOutputTokens := estimateOutputTokens(totalTokens)
	estimatedOutputCost := xai.EstimateOutputCostUSD(estimatedOutputTokens, pricing)
	estimatedTotalCost := totalInputCost + estimatedOutputCost

	fmt.Fprintln(r.Stdout)
	summary := tabwriter.NewWriter(r.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(summary, "%s\t%s\n", tokenizeColorHeader("METRIC"), tokenizeColorHeader("VALUE"))
	fmt.Fprintf(summary, "%s\t%s\n", tokenizeColorLabel("Model"), tokenizeColorValue(model))
	fmt.Fprintf(summary, "%s\t%s\n", tokenizeColorLabel("Files"), tokenizeColorValue(fmt.Sprintf("%d", len(results))))
	fmt.Fprintf(summary, "%s\t%s\n", tokenizeColorLabel("Total bytes"), tokenizeColorValue(fmt.Sprintf("%d", totalBytes)))
	fmt.Fprintf(summary, "%s\t%s\n", tokenizeColorLabel("Total input tokens"), tokenizeColorValue(fmt.Sprintf("%d", totalTokens)))
	fmt.Fprintf(summary, "%s\t%s\n", tokenizeColorLabel("Estimated input cost"), tokenizeColorValue(formatUSD(totalInputCost)))
	fmt.Fprintf(summary, "%s\t%s\n", tokenizeColorLabel("Estimated output tokens (best guess)"), tokenizeColorValue(fmt.Sprintf("%d", estimatedOutputTokens)))
	fmt.Fprintf(summary, "%s\t%s\n", tokenizeColorLabel("Estimated output cost (best guess)"), tokenizeColorValue(formatUSD(estimatedOutputCost)))
	_ = summary.Flush()
	fmt.Fprintln(r.Stdout, tokenizeColorMuted("------------------------------------------------------------"))
	totalSummary := tabwriter.NewWriter(r.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(totalSummary, "%s\t%s\n", tokenizeColorLabel("Estimated total cost (input + output)"), tokenizeColorTotal(formatUSD(estimatedTotalCost)))
	_ = totalSummary.Flush()
	fmt.Fprintf(r.Stdout, "\n%s\n", tokenizeColorWarning(estimateWarningNote))
}

func estimateOutputTokens(inputTokens int) int {
	if inputTokens <= 0 {
		return 0
	}

	guess := int(math.Round(float64(inputTokens) * outputGuessRatio))
	if guess < minOutputTokens {
		return minOutputTokens
	}
	return guess
}

func formatUSD(amount float64) string {
	return fmt.Sprintf("$%.6f", amount)
}

func isTerminal() bool {
	stdinInfo, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	stdoutInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (stdinInfo.Mode()&os.ModeCharDevice) != 0 && (stdoutInfo.Mode()&os.ModeCharDevice) != 0
}
