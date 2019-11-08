// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package login

import (
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"os"

	"github.com/rditech/rdi-live/live"

	"golang.org/x/oauth2"
)

func LoginMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authSession, _ := live.Store.Get(r, "auth-session")
		if authSession.IsNew {
			domain := os.Getenv("AUTH0_DOMAIN")
			aud := "https://" + domain + "/userinfo"

			conf := &oauth2.Config{
				ClientID:     os.Getenv("AUTH0_CLIENT_ID"),
				ClientSecret: os.Getenv("AUTH0_CLIENT_SECRET"),
				RedirectURL:  os.Getenv("AUTH0_CALLBACK_URL"),
				Scopes:       []string{"openid", "profile"},
				Endpoint: oauth2.Endpoint{
					AuthURL:  "https://" + domain + "/authorize",
					TokenURL: "https://" + domain + "/oauth/token",
				},
			}

			// Generate random state
			b := make([]byte, 32)
			rand.Read(b)
			state := base64.StdEncoding.EncodeToString(b)

			session, _ := live.Store.Get(r, "state")
			session.Values["state"] = state
			if err := session.Save(r, w); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			audience := oauth2.SetAuthURLParam("audience", aud)
			url := conf.AuthCodeURL(state, audience)

			http.Redirect(w, r, url, http.StatusTemporaryRedirect)
			return
		}

		next.ServeHTTP(w, r)
	})
}
