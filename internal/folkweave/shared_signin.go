// SPDX-License-Identifier: AGPL-3.0-or-later
package folkweave

import (
	"fmt"
	"github.com/airencracken/comfylib/clientip"
	sharedoidc "github.com/airencracken/comfylib/oidc"
	"github.com/airencracken/comfylib/suite"
	"github.com/airencracken/comfylib/token"
	"net/http"
)

func (a *App) registerSharedSignIn(mux *http.ServeMux) error {
	auth, err := sharedoidc.FromEnvironment("FOLKWEAVE_", a.config.BaseURL, a.store.db, suite.AuthHooks{
		RateKey: func(r *http.Request) string { return clientip.NetworkKey(a.resolver.Client(r)) },
		Current: func(r *http.Request) (int64, string) {
			u := state(r).User
			if u == nil {
				return 0, ""
			}
			binding := ""
			if c, err := r.Cookie(a.cookieName("session")); err == nil {
				binding = c.Value
			}
			return u.ID, binding
		},
		CSRF: func(r *http.Request) (string, string) { return "csrf", state(r).CSRF },
		SignIn: func(w http.ResponseWriter, r *http.Request, id int64) (string, error) {
			u, err := a.store.User(r.Context(), id)
			if err != nil || u.Disabled {
				return "", fmt.Errorf("account unavailable")
			}
			raw, err := a.store.NewSession(r.Context(), id)
			if err != nil {
				return "", err
			}
			a.cookie(w, "session", raw, 7*86400)
			a.cookie(w, "csrf", token.SessionCSRF(raw, "folkweave-csrf-v1"), 86400)
			return "/", nil
		},
	})
	if err != nil {
		return err
	}
	if auth != nil {
		auth.Register(mux)
	}
	return nil
}
