package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var version = "dev"

func main() {
	addr := flag.String("addr", "127.0.0.1:8787", "listen address")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("hexpm-envoy-proxy", version)
		os.Exit(0)
	}

	httpProxy := os.Getenv("HTTP_PROXY")
	if httpProxy == "" {
		httpProxy = os.Getenv("http_proxy")
	}
	httpsProxy := os.Getenv("HTTPS_PROXY")
	if httpsProxy == "" {
		httpsProxy = os.Getenv("https_proxy")
	}
	noProxy := os.Getenv("NO_PROXY")
	if noProxy == "" {
		noProxy = os.Getenv("no_proxy")
	}

	log.Printf("hexpm-envoy-proxy %s", version)
	log.Printf("  HTTP_PROXY:  %s", valueOrNone(httpProxy))
	log.Printf("  HTTPS_PROXY: %s", valueOrNone(httpsProxy))
	log.Printf("  NO_PROXY:    %s", valueOrNone(noProxy))

	transport := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		ResponseHeaderTimeout: 30 * time.Second,
		MaxIdleConnsPerHost:   10,
	}

	// Verify proxy resolution for upstream targets
	for _, target := range []string{"https://repo.hex.pm/", "https://builds.hex.pm/"} {
		u, _ := url.Parse(target)
		req := &http.Request{URL: u}
		proxyURL, err := http.ProxyFromEnvironment(req)
		if err != nil {
			log.Printf("  proxy resolution error for %s: %v", target, err)
		} else if proxyURL != nil {
			log.Printf("  %s -> via proxy %s", target, proxyURL)
		} else {
			log.Printf("  %s -> direct (no proxy)", target)
		}
	}

	mux := http.NewServeMux()
	mux.Handle("/builds/", http.StripPrefix("/builds", makeHandler(transport, "https://builds.hex.pm")))
	mux.Handle("/", makeHandler(transport, "https://repo.hex.pm"))

	log.Printf("listening on %s", *addr)
	log.Printf("  /        -> https://repo.hex.pm")
	log.Printf("  /builds/ -> https://builds.hex.pm")
	log.Fatal(http.ListenAndServe(*addr, mux))
}

func makeHandler(transport http.RoundTripper, upstream string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		outReq, err := http.NewRequest(r.Method, upstream+r.URL.RequestURI(), r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		for key, vals := range r.Header {
			switch strings.ToLower(key) {
			case "te", "host", "connection":
				continue
			}
			for _, val := range vals {
				outReq.Header.Add(key, val)
			}
		}

		resp, err := transport.RoundTrip(outReq)
		if err != nil {
			log.Printf("upstream error: %s %s -> %v", r.Method, upstream+r.URL.RequestURI(), err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for key, vals := range resp.Header {
			for _, val := range vals {
				w.Header().Add(key, val)
			}
		}
		w.WriteHeader(resp.StatusCode)
		io.Copy(w, resp.Body)
	}
}

func valueOrNone(s string) string {
	if s == "" {
		return "(not set)"
	}
	return s
}
