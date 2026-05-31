package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
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

	transport := &http.Transport{
		ResponseHeaderTimeout: 30 * time.Second,
		MaxIdleConnsPerHost:   10,
	}

	mux := http.NewServeMux()
	mux.Handle("/builds/", http.StripPrefix("/builds", makeHandler(transport, "https://builds.hex.pm")))
	mux.Handle("/", makeHandler(transport, "https://repo.hex.pm"))

	log.Printf("hexpm-envoy-proxy %s listening on %s", version, *addr)
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
