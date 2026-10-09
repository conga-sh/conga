// Package network integrates standard library network types into CONGA applications.
//
// Mount [Interface] using App.Use to register *net.Interface with net.InterfaceByName
// and enable network interface autocompletion:
//
//	type CLI struct {
//		Iface conga.OptionalFlag[*net.Interface] `short:"i" help:"Network interface"`
//	}
//
//	app := conga.New[CLI]().Use(network.Interface)
//
// Flags resolve interface names (e.g. lo, eth0) to live *net.Interface values.
// When shell completion is enabled, pressing Tab completes from available host interfaces.
//
// See https://docs.conga.sh/reference/plugins/ for the plugin reference.
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
