package main

import (
	"log"
	"net/http"
	"net/http/httputil"

	"github.com/aileron-projects/go-httpproxy"
)

func main() {
	// Run rest-api server as backend.
	go func() {
		log.Println("rest-api server is listening at: localhost:9090")
		if err := http.ListenAndServe(":9090", http.HandlerFunc(restHandler)); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
		log.Println("rest-api server closed")
	}()

	// Instanciate a proxy.
	proxy, err := httpproxy.NewProxy("http://localhost:9090")
	if err != nil {
		panic(err)
	}

	log.Println("proxy server is listening at: localhost:8080")
	if err := http.ListenAndServe(":8080", proxy); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func restHandler(w http.ResponseWriter, r *http.Request) {
	dump, err := httputil.DumpRequest(r, true)
	if err != nil {
		log.Println(err)
	}
	w.Header().Set("Contetnt-Type", "text/plain")
	_, _ = w.Write([]byte("========== request dump ==========\n"))
	_, _ = w.Write(dump)
	_, _ = w.Write([]byte("\n"))
	_, _ = w.Write([]byte("==================================\n"))
}
