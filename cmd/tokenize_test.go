package cmd

import (
	"context"
	"testing"

	"github.com/tituscheng/groktc/internal/tokenize"

	"github.com/spf13/cobra"
)

type spyRunner struct {
	called bool
	opts   tokenize.Options
}

func (s *spyRunner) Run(_ context.Context, opts tokenize.Options) error {
	s.called = true
	s.opts = opts
	return nil
}

func TestTokenizeCommandDefaultsAndDelegation(t *testing.T) {
	spy := &spyRunner{}
	previousFactory := newTokenizeRunner
	previousModel := tokenizeModel
	previousYes := tokenizeYes
	defer func() {
		newTokenizeRunner = previousFactory
		tokenizeModel = previousModel
		tokenizeYes = previousYes
	}()

	newTokenizeRunner = func() *tokenize.Runner {
		return &tokenize.Runner{}
	}

	tokenizeModel = defaultTokenizeModel
	tokenizeYes = false

	command := *tokenizeCmd
	command.RunE = func(_ *cobra.Command, args []string) error {
		return spy.Run(context.Background(), tokenize.Options{
			Args:          args,
			Model:         tokenizeModel,
			ModelExplicit: false,
			Yes:           tokenizeYes,
		})
	}

	if err := command.RunE(&command, []string{"a.txt"}); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !spy.called {
		t.Fatal("expected runner to be called")
	}
	if spy.opts.Model != defaultTokenizeModel {
		t.Fatalf("expected default model %q, got %q", defaultTokenizeModel, spy.opts.Model)
	}
	if len(spy.opts.Args) != 1 || spy.opts.Args[0] != "a.txt" {
		t.Fatalf("unexpected args: %v", spy.opts.Args)
	}
}
