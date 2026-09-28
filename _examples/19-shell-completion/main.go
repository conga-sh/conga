package main

import (
	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict"
)

func main() {
	podRegistry := map[string]map[string][]string{
		"production": {
			"frontend": {"web-prod-1", "web-prod-2", "edge-router-1"},
			"backend":  {"api-prod-8a", "billing-prod-3b", "auth-prod-7c"},
			"database": {"pg-primary-0", "redis-prod-node-1"},
		},
		"staging": {
			"frontend": {"web-stage-1", "web-stage-2"},
			"backend":  {"api-stage-alpha", "auth-stage-beta"},
			"database": {"pg-stage-0", "redis-stage-0"},
		},
		"development": {
			"frontend": {"web-dev-local"},
			"backend":  {"api-dev-local", "auth-dev-local"},
			"database": {"pg-dev-local", "redis-dev-local"},
		},
	}

	commandRegistry := map[string][]string{
		"web-prod-1":   {"nginx -t", "tail -n 100 /var/log/nginx/access.log"},
		"api-prod-8a":  {"curl -s localhost:8080/healthz", "kill -USR1 1"},
		"pg-primary-0": {"pg_isready", "psql -c 'SELECT 1'"},
	}

	app := conga.New[CLI]().
		Name("cloudctl").
		Description("Cloud management CLI with dynamic shell autocompletion (domain types + predictor registry)").
		Version("1.0.0").
		EnableCompletion().
		Predictor("pods", predict.Callback(func(compCtx predict.Context) predict.Predictor {
			profile := compCtx.FlagValue("profile", "staging")
			namespace := compCtx.FlagValue("namespace", "backend")

			if nsMap, ok := podRegistry[profile]; ok {
				if pods, ok := nsMap[namespace]; ok {
					return predict.Values(pods...).Usagef("target pod in %s/%s", profile, namespace)
				}
			}

			return predict.Values("default-pod-0").Usagef("target pod in %s/%s", profile, namespace)
		})).
		Predictor("commands", predict.Callback(func(compCtx predict.Context) predict.Predictor {
			if compCtx.HasFlag("container") {
				return predict.Values("sh -c 'env'", "cat /proc/1/cmdline").Usage("container-level diagnostics")
			}

			if commands, ok := commandRegistry[compCtx.LastArg()]; ok {
				return predict.Values(commands...).Usagef("known commands for pod %s", compCtx.LastArg())
			}

			return predict.Values("/bin/sh", "/bin/bash").Usagef("default shells (pod %q)", compCtx.Arg(0))
		})).
		Predictor("containers", predict.Values("app", "istio-proxy", "cache").
			Exclude("istio-proxy").
			Usage("non-sidecar containers"))

	appendLogPredictors(app)

	app.FatalIfError(app.Run())
}
