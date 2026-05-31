package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/francesco/hetzner_pulumi/pkg/platform/access"
	"github.com/francesco/hetzner_pulumi/pkg/platform/config"
	"github.com/francesco/hetzner_pulumi/pkg/platform/validation"
	"github.com/francesco/hetzner_pulumi/pkg/pulumi/hetznertalos"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
)

const (
	defaultEnvironmentName = "dev"
	defaultConfigPath      = "../../../config/environments.yaml"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		catalog, err := config.LoadFile(configPath())
		if err != nil {
			return err
		}

		envName := environmentName()
		env, ok := catalog.Environments[envName]
		if !ok {
			return fmt.Errorf("environment %q not found", envName)
		}

		env, err = applyEnvironmentOverrides(env)
		if err != nil {
			return err
		}
		catalog.Environments[envName] = env

		if err := validation.ValidateCatalog(catalog); err != nil {
			return err
		}

		if containsCurrentIP(env.Access.AllowedCIDRs) {
			currentIP := os.Getenv("PLATFORM_CURRENT_IP")
			if currentIP == "" {
				return fmt.Errorf("PLATFORM_CURRENT_IP is required because access.allowedCidrs contains current-ip")
			}

			resolvedAccess, err := access.ResolveCurrentIP(env.Access, currentIP)
			if err != nil {
				return err
			}
			env.Access = resolvedAccess
		}

		cluster, err := hetznertalos.NewCluster(ctx, env.Cluster.Name, hetznertalos.ClusterArgsFromEnvironment(env))
		if err != nil {
			return err
		}

		ctx.Export("endpoint", cluster.Endpoint)
		ctx.Export("controlPlaneNodeNames", pulumi.ToStringArray(nodeNames(cluster.ControlPlaneNodes)))
		ctx.Export("workerNodeNames", pulumi.ToStringArray(nodeNames(cluster.WorkerNodes)))
		ctx.Export("requiredArchitectures", pulumi.ToStringArray(cluster.RequiredArchitectures))

		return nil
	})
}

func configPath() string {
	if value := os.Getenv("PLATFORM_ENV_CONFIG"); value != "" {
		return value
	}

	return defaultConfigPath
}

func environmentName() string {
	if value := os.Getenv("PLATFORM_ENV_NAME"); value != "" {
		return value
	}

	return defaultEnvironmentName
}

func applyEnvironmentOverrides(env config.EnvironmentSpec) (config.EnvironmentSpec, error) {
	controlPlaneCount := os.Getenv("PLATFORM_CONTROL_PLANE_COUNT")
	if controlPlaneCount == "" {
		return env, nil
	}

	count, err := strconv.Atoi(controlPlaneCount)
	if err != nil {
		return config.EnvironmentSpec{}, fmt.Errorf("PLATFORM_CONTROL_PLANE_COUNT must be an integer: %w", err)
	}
	if len(env.NodePools.ControlPlane) == 0 {
		return config.EnvironmentSpec{}, fmt.Errorf("PLATFORM_CONTROL_PLANE_COUNT requires at least one control plane pool")
	}

	env.NodePools.ControlPlane[0].Count = count

	return env, nil
}

func containsCurrentIP(values []string) bool {
	for _, value := range values {
		if value == access.CurrentIPPlaceholder {
			return true
		}
	}

	return false
}

func nodeNames(nodes []hetznertalos.Node) []string {
	names := make([]string, 0, len(nodes))
	for _, node := range nodes {
		names = append(names, node.Name)
	}

	return names
}
