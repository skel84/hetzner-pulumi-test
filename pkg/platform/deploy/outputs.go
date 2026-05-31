package deploy

import (
	"context"
	"fmt"
	"os"

	"github.com/pulumi/pulumi/sdk/v3/go/auto"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

type OutputOptions struct {
	ProjectName string
	StackName   string
	WorkDir     string
	EnvVars     map[string]string
	Name        string

	readFn outputReader
}

type OutputResult struct {
	StackName string
	Name      string
	Value     string
	Secret    bool
}

type outputReader func(context.Context, automationStackOptions) (auto.OutputMap, error)

func ReadClusterOutput(ctx context.Context, opts OutputOptions) (OutputResult, error) {
	if opts.ProjectName == "" {
		return OutputResult{}, fmt.Errorf("project name is required")
	}
	if opts.StackName == "" {
		return OutputResult{}, fmt.Errorf("stack name is required")
	}
	if opts.WorkDir == "" {
		return OutputResult{}, fmt.Errorf("Pulumi work directory is required")
	}
	if opts.Name == "" {
		return OutputResult{}, fmt.Errorf("output name is required")
	}

	reader := opts.readFn
	if reader == nil {
		reader = readAutomationOutputs
	}

	outputs, err := reader(ctx, automationStackOptions{
		ProjectName: opts.ProjectName,
		StackName:   opts.StackName,
		WorkDir:     opts.WorkDir,
		EnvVars:     copyStringMap(opts.EnvVars),
	})
	if err != nil {
		return OutputResult{}, err
	}

	output, ok := outputs[opts.Name]
	if !ok {
		return OutputResult{}, fmt.Errorf("stack output %q not found", opts.Name)
	}
	value, ok := output.Value.(string)
	if !ok {
		return OutputResult{}, fmt.Errorf("stack output %q must be a string", opts.Name)
	}

	return OutputResult{
		StackName: opts.StackName,
		Name:      opts.Name,
		Value:     value,
		Secret:    output.Secret,
	}, nil
}

func readAutomationOutputs(ctx context.Context, opts automationStackOptions) (auto.OutputMap, error) {
	if err := os.MkdirAll(opts.WorkDir, 0o700); err != nil {
		return nil, fmt.Errorf("create Pulumi work directory: %w", err)
	}
	envVars, err := prepareAutomationEnv(opts.WorkDir, opts.EnvVars)
	if err != nil {
		return nil, err
	}

	stack, err := auto.SelectStackInlineSource(
		ctx,
		opts.StackName,
		opts.ProjectName,
		func(*pulumi.Context) error { return nil },
		stackOptions(opts.ProjectName, opts.WorkDir, envVars)...,
	)
	if err != nil {
		return nil, err
	}

	return stack.Outputs(ctx)
}
