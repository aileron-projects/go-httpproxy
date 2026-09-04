package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/aileron-projects/go-httpproxy"
)

func main() {
	// Run mock servers as backend.
	go func() {
		log.Println("hello server is listening at: localhost:9091")
		_ = http.ListenAndServe(":9091", http.HandlerFunc(helloHandler))
	}()
	go func() {
		log.Println("sleep server is listening at: localhost:9092")
		_ = http.ListenAndServe(":9092", http.HandlerFunc(sleepHandler))
	}()
	go func() {
		log.Println("panic server is listening at: localhost:9093")
		_ = http.ListenAndServe(":9093", http.HandlerFunc(panicHandler))
	}()

	// Instanciate a proxy.
	backends := []string{"http://localhost:9091", "http://localhost:9092", "http://localhost:9093", "http://localhost:9094"}
	proxy, err := httpproxy.NewProxy(backends...)
	if err != nil {
		panic(err)
	}

	// Use custom error handler.
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err *httpproxy.Error) {
		log.Println("ERROR:", err)
		if w == nil || !err.Writable() {
			return
		}
		var status int
		switch err.Cause {
		case httpproxy.ErrPreRoundTrip, httpproxy.ErrPostRoundTrip:
			status = http.StatusInternalServerError
		case httpproxy.ErrRoundTrip:
			fallthrough
		default:
			status = http.StatusBadGateway
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(http.StatusText(status)))
	}

	log.Println("proxy server is listening at: localhost:8080")
	if err := http.ListenAndServe(":8080", proxy); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Contetnt-Type", "text/plain")
	_, _ = w.Write([]byte("hello"))
}
func sleepHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("handler sleeps 3s!!")
	time.Sleep(3 * time.Second)
	w.Header().Set("Contetnt-Type", "text/plain")
	_, _ = w.Write([]byte("slept"))
}

func panicHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("handler panics!!")
	panic(errors.New("panic handler"))
}
