package spec

import (
	"fmt"
	"slices"
	"strconv"
)

// ConfigEnvVars returns the environment variables an app reads a config file
// path from. A declared ConfigFile decides alone: its env tag, the derived
// <PREFIX>_CONFIG, or none with env:"-". Without one, an app with a registered
// config decoder (decoderRegistered) reads <PREFIX>_CONFIG, and any other app
// reads none. root is the root command; envPrefix carries its trailing
// underscore. Call it on the command as schema build left it: feature
// pruning removes an inactive ConfigFile flag, after which the decoder
// fallback would apply.
func ConfigEnvVars(envPrefix string, root *Command, decoderRegistered bool) []string {
	if root != nil && root.ConfigFileFlag != nil {
		return root.ConfigFileFlag.EnvVars
	}

	if decoderRegistered && envPrefix != "" {
		return []string{envPrefix + "CONFIG"}
	}

	return nil
}

// validateConfigEnvCollisions rejects a flag or feature bound to an
// environment variable the app reads a config file path from: setting the
// variable for the flag would make config resolution read it as a path.
func validateConfigEnvCollisions(root *Command, decoderRegistered bool) error {
	reserved := ConfigEnvVars(root.EnvPrefix, root, decoderRegistered)
	if len(reserved) == 0 {
		return nil
	}

	configSide := "the config file variable"
	if root.ConfigFileFlag != nil {
		configSide = "flag --" + root.ConfigFileFlag.Name
	}

	if err := checkConfigEnvFlags(root, reserved, configSide); err != nil {
		return err
	}

	for _, feature := range root.FeatureFields {
		for _, envVar := range feature.EnvVars {
			if slices.Contains(reserved, envVar) {
				return wrapFeatureEnvCollision(root, ConfigEnvCollisionError(envVar, configSide, "feature "+strconv.Quote(feature.Name), `give the feature another env tag or exclude it with env:"-"`))
			}
		}
	}

	return nil
}

// featureMapCollision builds the collision error for the Features
// container's combined map variable: it names the feature map flag when the
// container has one on the command line, and the container otherwise.
func featureMapCollision(root *Command, envVar string, configSide string) error {
	if root.FeatureFlag.NoCLI {
		return ConfigEnvCollisionError(envVar, configSide, "the Features container", "give the Features container another env tag")
	}

	return ConfigEnvCollisionError(envVar, configSide, "the feature map flag --"+root.FeatureFlag.Name, "give the Features container another env tag")
}

// checkConfigEnvFlags walks cmd and its subcommands for a flag, other than
// the ConfigFile flag itself, bound to one of the reserved variables.
func checkConfigEnvFlags(cmd *Command, reserved []string, configSide string) error {
	for _, flagSpec := range cmd.Flags {
		if flagSpec.IsConfigFile {
			continue
		}

		for _, envVar := range flagSpec.EnvVars {
			if !slices.Contains(reserved, envVar) {
				continue
			}

			if flagSpec == cmd.FeatureFlag {
				return wrapFeatureEnvCollision(cmd, featureMapCollision(cmd, envVar, configSide))
			}

			err := ConfigEnvCollisionError(envVar, configSide, "flag --"+flagSpec.Name, `give the flag another env tag or exclude it with env:"-"`)

			return fieldContext(flagSpec.StructName, flagSpec.FieldName, err)
		}
	}

	for _, subcommand := range cmd.Subcommands {
		if err := checkConfigEnvFlags(subcommand, reserved, configSide); err != nil {
			return err
		}
	}

	return nil
}

// fieldContext wraps err with the struct and field that declare the binding.
func fieldContext(structName string, fieldName string, err error) error {
	return fmt.Errorf("struct %s field %s: %w", structName, fieldName, err)
}
