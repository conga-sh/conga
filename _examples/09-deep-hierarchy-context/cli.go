package main

import "github.com/conga-sh/conga"

// CLI is the root CLI struct. Root flags bind to FLEET_* environment variables
// through app.EnvPrefix; nested commands inherit the prefix unless their
// command field declares an envprefix tag.
type CLI struct {
	GlobalRegion conga.OptionalFlag[string] `default:"us-east-1"          help:"Global cloud region" short:"r"`
	Cluster      conga.Command[ClusterCmd]  `summary:"Cluster operations"`
}
