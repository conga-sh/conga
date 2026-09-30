package main

import (
	"context"
	"io/fs"
	"slices"
	"strings"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/_examples/internal/inspector"
	"github.com/conga-sh/conga/predict"
)

// Region demonstrates self-completing domain types via predict.Predictable.
type Region string

func (r Region) Predict() predict.Predictor {
	return predict.Items(
		predict.NewItem("us-east-1", "North America (N. Virginia)"),
		predict.NewItem("eu-west-1", "Europe (Ireland)"),
		predict.NewItem("ap-southeast-1", "Asia Pacific (Singapore)"),
	)
}

// Cluster demonstrates dynamic contextual autocompletion on domain types via predict.Predictable.
type Cluster string

func (c Cluster) Predict() predict.Predictor {
	return predict.Callback(func(compCtx predict.Context) predict.Predictor {
		env := compCtx.FlagValue("env", "staging")

		clusterRegistry := map[string][]string{
			"production": {"prod-us-east-1", "prod-eu-west-1", "prod-ap-southeast-1"},
			"qa":         {"qa-test-1", "qa-test-2"},
			"staging":    {"staging-cluster-alpha", "staging-cluster-beta"},
		}

		clusters, ok := clusterRegistry[env]
		if !ok {
			clusters = clusterRegistry["staging"]
		}

		return predict.Values(clusters...).Usagef("target cluster in %s", env)
	})
}

// Image demonstrates segmented MultiParts autocompletion on domain types via predict.Predictable.
type Image string

func (i Image) Predict() predict.Predictor {
	imageTagRegistry := map[string][]string{
		"api-gateway":     {"v3.1.0", "v3.2.0", "v3.2.1-rc1", "latest"},
		"auth-service":    {"v1.0.0", "v1.1.0", "v1.2.0", "latest"},
		"billing-service": {"v2.0.0", "v2.0.1", "latest"},
	}

	return predict.MultiParts(":", func(compCtx predict.Context) predict.Predictor {
		if len(compCtx.Parts) == 0 {
			repos := make([]string, 0, len(imageTagRegistry))
			for repo := range imageTagRegistry {
				repos = append(repos, repo+":")
			}

			slices.Sort(repos)

			return predict.Values(repos...).NoSpace().Usage("container image repository")
		}

		repo := compCtx.Parts[0]
		if tags, ok := imageTagRegistry[repo]; ok {
			return predict.Values(tags...).Usagef("image tag for %s", repo)
		}

		return predict.Values("latest").Usage("image tag")
	})
}

// Feature demonstrates slice value autocompletion on domain types via predict.Predictable.
type Feature string

func (f Feature) Predict() predict.Predictor {
	return predict.Values("canary", "tracing", "metrics", "debug", "profiling").
		Usage("deployment feature flags")
}

// ManifestFile demonstrates filesystem filtering on domain types via predict.FileFilterable.
type ManifestFile string

func (m ManifestFile) MatchFile(path string, entry fs.DirEntry) bool {
	return strings.HasSuffix(path, ".yaml") || strings.HasSuffix(path, ".yml")
}

// DeployCmd uses the recommended completion style: every flag's type carries
// its own completion behaviour, so no registry wiring is needed.
type DeployCmd struct {
	Env      conga.OptionalFlag[string]       `default:"staging"                                   enum:"staging,production,qa"                 help:"Target deployment environment" short:"e"`
	Region   conga.OptionalFlag[Region]       `default:"us-east-1"                                 help:"Target cloud region (self-completing)" short:"r"`
	Cluster  conga.OptionalFlag[Cluster]      `help:"Target cluster (contextual)"                  short:"c"`
	Image    conga.OptionalFlag[Image]        `help:"Container image (repository:tag)"             short:"i"`
	Features conga.OptionalFlag[[]Feature]    `help:"Enabled deployment features"                  sep:","`
	Manifest conga.OptionalFlag[ManifestFile] `help:"Deployment manifest file (yaml/yml filtered)"`
	OutDir   conga.OptionalFlag[conga.Dir]
	DryRun   conga.OptionalFlag[bool]  `default:"false"               help:"Simulate deployment" negatable:"true"`
	Service  conga.RequiredArg[string] `help:"Service name to deploy"`
}

func (d *DeployCmd) Run(ctx context.Context, session *conga.Session) error {
	inspector.Print(session, session.Root[CLI]())

	return nil
}
