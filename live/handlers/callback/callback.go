// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package callback

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/rditech/rdi-live/live"

	"golang.org/x/oauth2"
)

func LoginCallback(w http.ResponseWriter, r *http.Request) {
	domain := os.Getenv("AUTH0_DOMAIN")

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
	state := r.URL.Query().Get("state")
	session, err := live.Store.Get(r, "state")
	if err != nil {
		session.Options.MaxAge = -1
		if err := session.Save(r, w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	if state != session.Values["state"] {
		http.Error(w, "Invalid state parameter", http.StatusInternalServerError)
		return
	}

	code := r.URL.Query().Get("code")

	token, err := conf.Exchange(context.TODO(), code)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Getting the user info
	client := conf.Client(context.TODO(), token)
	resp, err := client.Get("https://" + domain + "/userinfo")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer resp.Body.Close()

	var profile map[string]interface{}
	if err = json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	session, _ = live.Store.Get(r, "auth-session")

	session.Values["id_token"] = token.Extra("id_token")
	session.Values["access_token"] = token.AccessToken
	for key, value := range profile {
		if strings.HasSuffix(key, "app_metadata") {
			session.Values["app_metadata"] = value
		} else if key == "nickname" {
			session.Values["nickname"] = value
		}
	}
	if err := session.Save(r, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect to logged in page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
