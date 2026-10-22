package predict_test

import (
	"testing"

	"github.com/alecthomas/assert/v2"

	"github.com/conga-sh/conga/predict"
)

func TestContext(t *testing.T) {
	t.Parallel()

	t.Run("has flag when present", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Flags: map[string]string{
				"env": "staging",
			},
		}

		has := ctx.HasFlag("env")

		assert.True(t, has)
	})

	t.Run("has flag in flags slice", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			FlagMultiValues: map[string][]string{
				"tag": {"core", "api"},
			},
		}

		has := ctx.HasFlag("tag")

		assert.True(t, has)
	})

	t.Run("has flag when absent or nil", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{}

		has := ctx.HasFlag("env")

		assert.False(t, has)
	})

	t.Run("flag value when present", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Flags: map[string]string{
				"output": "json",
			},
		}

		value := ctx.FlagValue("output", "yaml")

		assert.Equal(t, "json", value)
	})

	t.Run("flag value fallback to flags slice", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			FlagMultiValues: map[string][]string{
				"tag": {"first", "second"},
			},
		}

		value := ctx.FlagValue("tag", "fallback")

		assert.Equal(t, "second", value)
	})

	t.Run("flag values from flags slice", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			FlagMultiValues: map[string][]string{
				"tag": {"alpha", "beta", "gamma"},
			},
		}

		values := ctx.FlagValues("tag")

		assert.Equal(t, []string{"alpha", "beta", "gamma"}, values)
	})

	t.Run("flag values fallback to single flag map", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Flags: map[string]string{
				"format": "json",
			},
		}

		values := ctx.FlagValues("format")

		assert.Equal(t, []string{"json"}, values)
	})

	t.Run("accessors agree on a divergent context", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Flags: map[string]string{
				"tag": "scalar",
			},
			FlagMultiValues: map[string][]string{
				"tag": {"first", "second"},
			},
		}

		value := ctx.FlagValue("tag", "fallback")
		values := ctx.FlagValues("tag")
		has := ctx.HasFlag("tag")

		assert.Equal(t, "second", value)
		assert.Equal(t, []string{"first", "second"}, values)
		assert.True(t, has)
		assert.Equal(t, value, values[len(values)-1])
	})

	t.Run("empty flags slice entry counts as absent", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Flags: map[string]string{
				"tag": "scalar",
			},
			FlagMultiValues: map[string][]string{
				"tag": {},
			},
		}

		value := ctx.FlagValue("tag", "fallback")
		values := ctx.FlagValues("tag")
		has := ctx.HasFlag("tag")

		assert.Equal(t, "scalar", value)
		assert.Equal(t, []string{"scalar"}, values)
		assert.True(t, has)
	})

	t.Run("flag values absent or nil", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{}

		values := ctx.FlagValues("missing")

		assert.Equal(t, []string(nil), values)
	})

	t.Run("flag value fallback when absent", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Flags: map[string]string{},
		}

		value := ctx.FlagValue("output", "yaml")

		assert.Equal(t, "yaml", value)
	})

	t.Run("flag value fallback when flags nil", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{}

		value := ctx.FlagValue("output", "yaml")

		assert.Equal(t, "yaml", value)
	})

	t.Run("arg access in bounds", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Args: []string{"deploy", "staging", "us-east-1"},
		}

		assert.Equal(t, "deploy", ctx.Arg(0))
		assert.Equal(t, "staging", ctx.Arg(1))
		assert.Equal(t, "us-east-1", ctx.Arg(2))
	})

	t.Run("arg access out of bounds", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Args: []string{"deploy"},
		}

		assert.Equal(t, "", ctx.Arg(-1))
		assert.Equal(t, "", ctx.Arg(1))
		assert.Equal(t, "", ctx.Arg(10))
	})

	t.Run("last arg with populated slice", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Args: []string{"cluster", "nodes"},
		}

		last := ctx.LastArg()

		assert.Equal(t, "nodes", last)
	})

	t.Run("last arg with empty slice", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Args: []string{},
		}

		last := ctx.LastArg()

		assert.Equal(t, "", last)
	})

	t.Run("getenv with context env", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Env: map[string]string{
				"AWS_REGION": "eu-west-1",
			},
		}

		value := ctx.Getenv("AWS_REGION")
		absent := ctx.Getenv("MISSING_VAR")

		assert.Equal(t, "eu-west-1", value)
		assert.Equal(t, "", absent)
	})
}

func TestContext_GetenvOSFallback(t *testing.T) {
	// Not parallel because it modifies process-wide environment variables.
	t.Setenv("CONGA_TEST_VAR_XYZ", "hello_conga")

	ctx := predict.Context{}

	value := ctx.Getenv("CONGA_TEST_VAR_XYZ")

	assert.Equal(t, "hello_conga", value)
}

func TestContext_GetenvMissingKeyFallback(t *testing.T) {
	// Not parallel because it modifies process-wide environment variables.
	t.Setenv("CONGA_TEST_FALLBACK_VAR", "from_process")

	ctx := predict.Context{
		Env: map[string]string{
			"AWS_REGION": "eu-west-1",
		},
	}

	present := ctx.Getenv("AWS_REGION")
	fallback := ctx.Getenv("CONGA_TEST_FALLBACK_VAR")

	assert.Equal(t, "eu-west-1", present)
	assert.Equal(t, "from_process", fallback)
}

func TestContext_Getenv_EnvLookup(t *testing.T) {
	t.Parallel()

	t.Run("lookup wins over env map", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Env: map[string]string{"SHADOWED": "from_env_map"},
			EnvLookup: func(key string) (string, bool) {
				if key == "SHADOWED" {
					return "from_lookup", true
				}

				return "", false
			},
		}

		result := ctx.Getenv("SHADOWED")

		assert.Equal(t, "from_lookup", result)
	})

	t.Run("lookup-only key", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			EnvLookup: func(key string) (string, bool) {
				if key == "LOOKUP_ONLY" {
					return "lookup_val", true
				}

				return "", false
			},
		}

		result := ctx.Getenv("LOOKUP_ONLY")

		assert.Equal(t, "lookup_val", result)
	})

	t.Run("env map fallback", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{
			Env: map[string]string{"ENV_ONLY": "from_env_map_only"},
		}

		result := ctx.Getenv("ENV_ONLY")

		assert.Equal(t, "from_env_map_only", result)
	})

	t.Run("absent key", func(t *testing.T) {
		t.Parallel()

		ctx := predict.Context{}

		result := ctx.Getenv("NON_EXISTENT_VAR")

		assert.Zero(t, result)
	})
}

// TestContext_Getenv_ProcessEnvFallback verifies that Getenv falls back to
// the process environment when neither EnvLookup nor Env supply the key. It
// mutates process-wide environment state via t.Setenv, so it does not run in
// parallel.
func TestContext_Getenv_ProcessEnvFallback(t *testing.T) {
	t.Setenv("CONGA_TEST_PROCESS_VAR", "from_os")

	ctx := predict.Context{}

	result := ctx.Getenv("CONGA_TEST_PROCESS_VAR")

	assert.Equal(t, "from_os", result)
}

func TestContext_WithFlagValues(t *testing.T) {
	t.Parallel()

	ctx := predict.Context{}.WithFlagValues(map[string][]string{
		"env":  {"dev", "prod"},
		"tier": {"web"},
	})

	assert.Equal(t, []string{"dev", "prod"}, ctx.FlagValues("env"))
	assert.Equal(t, "prod", ctx.FlagValue("env", ""))
	assert.Equal(t, "web", ctx.FlagValue("tier", ""))
	assert.Equal(t, "prod", ctx.Flags["env"])
	assert.Equal(t, "web", ctx.Flags["tier"])
}
