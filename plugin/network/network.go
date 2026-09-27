// Package network provides standard library network plugins for the CONGA CLI
// builder. Apply [Interface] with App.Use to register *net.Interface through
// net.InterfaceByName and add network interface completion candidates.
package network

import (
	"net"

	"github.com/conga-sh/conga"
	"github.com/conga-sh/conga/predict/network"
)

// Interface registers *net.Interface with net.InterfaceByName and network interface completion.
func Interface[T any](app *conga.App[T]) {
	app.RegisterType(net.InterfaceByName, conga.WithPredictor(network.NetInterfaces()))
}
