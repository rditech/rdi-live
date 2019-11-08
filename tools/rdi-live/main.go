// Copyright 2019 Radiation Detection and Imaging (RDI), LLC
// Use of this source code is governed by the BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/pprof"
	"strconv"
	"strings"
	"time"

	"github.com/rditech/rdi-live/live"
	"github.com/rditech/rdi-live/live/handlers/callback"
	"github.com/rditech/rdi-live/live/handlers/client"
	"github.com/rditech/rdi-live/live/handlers/ingress"
	"github.com/rditech/rdi-live/live/handlers/login"
	"github.com/rditech/rdi-live/live/handlers/logout"

	"github.com/alicebob/miniredis"
	"github.com/go-redis/redis"
	"github.com/gorilla/mux"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/net/websocket"
)

var (
	openBrowser = flag.Bool("b", false, "open a browser window and connect to server")
	cpuProfile  = flag.String("cpuprofile", "", "output file for cpu profiling")
)

func printUsage() {
	fmt.Fprintf(os.Stderr,
		`Usage: `+os.Args[0]+` [options]

Description

options:
`,
	)
	flag.PrintDefaults()
}

func main() {
	flag.Usage = printUsage
	flag.Parse()

	// Define redis connection
	redisAddr := os.Getenv("REDIS_ADDR")
	if len(redisAddr) == 0 {
		s, err := miniredis.Run()
		if err != nil {
			log.Println("unable to start miniredis server:", err)
		}
		redisAddr = s.Addr()
	}
	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()
	ping := redisClient.Ping()
	if ping.Err() != nil {
		log.Fatalf("unable to ping redis server: %v\n", ping.Err())
	} else {
		log.Printf("successfully connected to redis server at %v with status %v\n", redisAddr, ping.String())
	}

	// Define handlers
	callbackHandler := http.HandlerFunc(callback.LoginCallback)
	clientHandler := &client.ClientHandler{Redis: redisClient, Addr: redisAddr}
	clientHandler.MaxNPR = float64(100)
	if len(os.Getenv("MAX_NPR")) > 0 {
		if max, err := strconv.ParseFloat(os.Getenv("MAX_NPR"), 64); err == nil {
			clientHandler.MaxNPR = max
		}
	}
	clientHandler.EnableCompression = true
	wsc := &ingress.WsCollector{Redis: redisClient, Addr: redisAddr}
	ingressHandler := websocket.Handler(wsc.Collect)
	logoutHandler := http.HandlerFunc(logout.Logout)
	webdataHandler := http.StripPrefix("/webdata/", http.FileServer(live.WebdataBox))
	rootHandler := http.StripPrefix("/", http.FileServer(live.WebdataBox))

	// Define http server and routes
	port := os.Getenv("PORT")
	if len(port) == 0 {
		port = "8080"
	}
	router := mux.NewRouter()
	if len(os.Getenv("AUTH0_CLIENT_ID")) > 0 {
		log.Println("Enabling Auth0 login with client ID", os.Getenv("AUTH0_CLIENT_ID"))

		wsc.DefaultNamespace = "rdi-data-dev1"

		router.Handle("/callback", callbackHandler)
		router.Handle("/client", login.LoginMiddleware(clientHandler))
		router.Handle("/ingress", ingressHandler)
		router.Handle("/logout", logoutHandler)
		router.PathPrefix("/webdata/").Handler(webdataHandler)
		router.PathPrefix("/").Handler(login.LoginMiddleware(rootHandler))
	} else {
		wsc.DefaultNamespace = "everyone"

		router.Handle("/client", clientHandler)
		router.Handle("/ingress", ingressHandler)
		router.PathPrefix("/webdata/").Handler(webdataHandler)
		router.PathPrefix("/").Handler(rootHandler)
	}

	srv := &http.Server{Addr: ":" + port, Handler: router}
	switch strings.ToLower(os.Getenv("SECURE_ONLY")) {
	case "true", "on":
		log.Println("Enabling HTTP proxy securing middleware")
		srv = &http.Server{Addr: ":" + port, Handler: Secure(router)}
	}

	// Turn on cpu profiling if output file is specified
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err != nil {
			log.Fatal("could not create cpu profile file: ", err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	// Set up interrupt for nice quitting
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
		srv.Shutdown(context.Background())
	}()

	// Open a browser window if flag is set
	if *openBrowser {
		// Instruct the clientHandler to shutdown the server when clients all
		// disconnect
		clientHandler.Srv = srv
		go func() {
			time.Sleep(10 * time.Millisecond)
			open.Run("http://localhost:" + port)
		}()
	}

	// Launch HTTP server and main display routine
	log.Println("http server started on :" + port)
	if err := srv.ListenAndServe(); err != nil {
		log.Println("ListenAndServe: ", err)
	}

	log.Println("successful quit")
}

// Middleware for redirecting http requests that are behind an HTTP proxy to
// https
func Secure(next http.Handler) http.Handler {
	return http.HandlerFunc(
		func(w http.ResponseWriter, r *http.Request) {
			if strings.ToLower(r.Header.Get("x-forwarded-proto")) == "http" {
				target := "https://" + r.Host + r.URL.Path
				if len(r.URL.RawQuery) > 0 {
					target += "?" + r.URL.RawQuery
				}
				log.Printf("redirect to: %s", target)
				http.Redirect(w, r, target,
					http.StatusTemporaryRedirect)
				return
			}

			next.ServeHTTP(w, r)
		},
	)
}
