package spec

import (
	"slices"
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

	configFileFlagName := ""
	if root.ConfigFileFlag != nil {
		configFileFlagName = root.ConfigFileFlag.Name
	}

	configSide := ConfigEnvSide(configFileFlagName)

	if err := checkConfigEnvFlags(root, reserved, configSide); err != nil {
		return err
	}

	for _, feature := range root.FeatureFields {
		for _, envVar := range feature.EnvVars {
			if slices.Contains(reserved, envVar) {
				return wrapFeatureEnvCollision(root, ConfigEnvFeatureCollisionError(envVar, configSide, feature.Name))
			}
		}
	}

	return nil
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
				return wrapFeatureEnvCollision(cmd, ConfigEnvFeatureMapCollisionError(envVar, configSide, cmd.FeatureFlag.Name, cmd.FeatureFlag.NoCLI))
			}

			err := ConfigEnvFlagCollisionError(envVar, configSide, flagSpec.Name)

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

// fieldContext attaches the struct and field that declare the binding to err.
func fieldContext(structName string, fieldName string, err error) error {
	return WithOwner(err, structName, fieldName)
}
