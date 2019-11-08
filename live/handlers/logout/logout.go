// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package logout

import (
	"net/http"
	"net/url"
	"os"

	"github.com/rditech/rdi-live/live"
)

func Logout(w http.ResponseWriter, r *http.Request) {
	authSession, _ := live.Store.Get(r, "auth-session")
	authSession.Options.MaxAge = -1
	if err := authSession.Save(r, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	domain := os.Getenv("AUTH0_DOMAIN")

	var logoutUrl *url.URL
	logoutUrl, err := url.Parse("https://" + domain)

	if err != nil {
		panic("Logout: failed to parse url")
	}

	logoutUrl.Path += "/v2/logout"
	parameters := url.Values{}
	parameters.Add("returnTo", os.Getenv("AUTH0_LOGOUT_URL"))
	parameters.Add("client_id", os.Getenv("AUTH0_CLIENT_ID"))
	logoutUrl.RawQuery = parameters.Encode()

	http.Redirect(w, r, logoutUrl.String(), http.StatusTemporaryRedirect)
}
