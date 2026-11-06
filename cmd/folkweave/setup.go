// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"github.com/airencracken/comfylib/setup"
	"github.com/airencracken/folkweave/internal/folkweave"
)

func firstSetup(store *folkweave.Store, data, base, addr string) (*setup.Handler, error) {
	return setup.New(setup.Config{App: "folkweave", Directory: data, BaseURL: base, Address: addr, Ready: store.HasOwner, Create: store.CreateFirstOwner})
}
