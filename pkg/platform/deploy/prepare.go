package deploy

import (
	"context"
	"fmt"

	"github.com/francesco/hetzner_pulumi/pkg/platform/access"
	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
	"github.com/francesco/hetzner_pulumi/pkg/platform/validation"
)

type CurrentIPResolver func(context.Context) (string, error)

type PrepareOptions struct {
	ConfigPath         string
	EnvironmentName    string
	ControlPlaneCount  int
	WorkerCount        int
	WorkerCountSet     bool
	CurrentIP          string
	ResolveCurrentIPFn CurrentIPResolver
}

type PreparedEnvironment struct {
	Name              string
	Environment       config.EnvironmentSpec
	CurrentIPResolved bool
}

func PrepareEnvironment(ctx context.Context, opts PrepareOptions) (PreparedEnvironment, error) {
	catalog, err := config.LoadFile(opts.ConfigPath)
	if err != nil {
		return PreparedEnvironment{}, err
	}
	if _, ok := catalog.Environments[opts.EnvironmentName]; !ok {
		return PreparedEnvironment{}, fmt.Errorf("environment %q not found", opts.EnvironmentName)
	}

	catalog.Environments = copyEnvironmentMap(catalog.Environments)
	env := cloneEnvironment(catalog.Environments[opts.EnvironmentName])

	if opts.ControlPlaneCount > 0 {
		env, err = withControlPlaneCount(env, opts.ControlPlaneCount)
		if err != nil {
			return PreparedEnvironment{}, err
		}
	}
	if opts.WorkerCountSet {
		env, err = withWorkerCount(env, opts.WorkerCount)
		if err != nil {
			return PreparedEnvironment{}, err
		}
	}
	catalog.Environments[opts.EnvironmentName] = env

	if err := validation.ValidateCatalog(catalog); err != nil {
		return PreparedEnvironment{}, err
	}

	currentIPResolved := false
	if containsCurrentIP(env.Access.AllowedCIDRs) {
		currentIP := opts.CurrentIP
		if currentIP == "" {
			if opts.ResolveCurrentIPFn == nil {
				return PreparedEnvironment{}, fmt.Errorf("current-ip resolver is required because access.allowedCidrs contains %q", access.CurrentIPPlaceholder)
			}
			currentIP, err = opts.ResolveCurrentIPFn(ctx)
			if err != nil {
				return PreparedEnvironment{}, err
			}
		}

		resolvedAccess, err := access.ResolveCurrentIP(env.Access, currentIP)
		if err != nil {
			return PreparedEnvironment{}, err
		}
		env.Access = resolvedAccess
		currentIPResolved = true
		if err := validation.ValidateEnvironment(env); err != nil {
			return PreparedEnvironment{}, err
		}
	}

	return PreparedEnvironment{
		Name:              opts.EnvironmentName,
		Environment:       env,
		CurrentIPResolved: currentIPResolved,
	}, nil
}

func withControlPlaneCount(env config.EnvironmentSpec, count int) (config.EnvironmentSpec, error) {
	if len(env.NodePools.ControlPlane) == 0 {
		return config.EnvironmentSpec{}, fmt.Errorf("control plane count override requires at least one control plane pool")
	}

	updated := cloneEnvironment(env)
	updated.NodePools.ControlPlane[0].Count = count

	return updated, nil
}

func withWorkerCount(env config.EnvironmentSpec, count int) (config.EnvironmentSpec, error) {
	if count < 0 {
		return config.EnvironmentSpec{}, fmt.Errorf("worker count override must be zero or greater")
	}

	updated := cloneEnvironment(env)
	if count == 0 {
		updated.NodePools.Workers = nil
		return updated, nil
	}
	if len(updated.NodePools.Workers) == 0 {
		return config.EnvironmentSpec{}, fmt.Errorf("worker count override requires at least one worker pool")
	}
	updated.NodePools.Workers[0].Count = count

	return updated, nil
}

func containsCurrentIP(values []string) bool {
	for _, value := range values {
		if value == access.CurrentIPPlaceholder {
			return true
		}
	}

	return false
}

func copyEnvironmentMap(values map[string]config.EnvironmentSpec) map[string]config.EnvironmentSpec {
	copied := make(map[string]config.EnvironmentSpec, len(values))
	for name, env := range values {
		copied[name] = cloneEnvironment(env)
	}

	return copied
}

func cloneEnvironment(env config.EnvironmentSpec) config.EnvironmentSpec {
	updated := env
	updated.Access.AllowedCIDRs = append([]string(nil), env.Access.AllowedCIDRs...)
	updated.NodePools.ControlPlane = append([]config.NodePoolSpec(nil), env.NodePools.ControlPlane...)
	updated.NodePools.Workers = append([]config.NodePoolSpec(nil), env.NodePools.Workers...)

	return updated
}
