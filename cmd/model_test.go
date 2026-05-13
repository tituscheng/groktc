package cmd

import (
	"context"
	"testing"

	"github.com/tituscheng/groktc/internal/model"

	"github.com/spf13/cobra"
)

type spyModelRunner struct {
	called bool
	opts   model.RunnerOptions
}

func (s *spyModelRunner) Run(_ context.Context, opts model.RunnerOptions) error {
	s.called = true
	s.opts = opts
	return nil
}

func TestModelCommandDelegatesToRunner(t *testing.T) {
	spy := &spyModelRunner{}
	previousFactory := newModelRunner
	previousSet := modelSetID
	previousList := modelList
	defer func() {
		newModelRunner = previousFactory
		modelSetID = previousSet
		modelList = previousList
	}()

	modelSetID = "grok-4.3"
	modelList = true

	command := *modelCmd
	command.RunE = func(_ *cobra.Command, _ []string) error {
		return spy.Run(context.Background(), model.RunnerOptions{
			SetModelID: modelSetID,
			ListOnly:   modelList,
		})
	}

	if err := command.RunE(&command, nil); err != nil {
		t.Fatalf("RunE returned error: %v", err)
	}
	if !spy.called {
		t.Fatal("expected runner to be called")
	}
	if spy.opts.SetModelID != "grok-4.3" {
		t.Fatalf("expected set model id grok-4.3, got %q", spy.opts.SetModelID)
	}
	if !spy.opts.ListOnly {
		t.Fatalf("expected list only true")
	}
}
