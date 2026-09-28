package main

import "github.com/conga-sh/conga"

// CLI is the root CLI struct. The blank envprefix field binds every root flag
// to CLOUDCTL_* environment variables; nested commands inherit the prefix
// unless they declare their own envprefix tag.
type CLI struct {
	_            struct{}                   `envprefix:"CLOUDCTL_"`
	GlobalRegion conga.OptionalFlag[string] `default:"us-east-1"          help:"Global cloud region" short:"r"`
	Cluster      conga.Command[ClusterCmd]  `summary:"Cluster operations"`
}
