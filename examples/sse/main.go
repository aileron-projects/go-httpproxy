package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aileron-projects/go-httpproxy"
)

func main() {
	// Run SSE server as backend.
	go func() {
		log.Println("SSE server is listening at: localhost:9090")
		if err := http.ListenAndServe(":9090", http.HandlerFunc(sseHandler)); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
		log.Println("SSE server closed")
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

func sseHandler(w http.ResponseWriter, r *http.Request) {
	flusher, _ := w.(http.Flusher)

	// Set response headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "identity")

	// Flush before sending the body.
	flusher.Flush()

	done := make(chan struct{})
	go func() {
		fmt.Fprintf(w, "Hello !!\n")

		for range 30 {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(time.Second):
			}

			n, err := fmt.Fprintf(w, "It's %s\n", time.Now().Format(http.TimeFormat))
			if n > 0 {
				flusher.Flush()
			}
			if err != nil {
				panic(err)
			}
		}

		fmt.Fprintf(w, "Goodbye !!\n")
		flusher.Flush()
		close(done)
	}()

	select {
	case <-r.Context().Done():
		log.Println("client closed connection")
	case <-done:
		log.Println("done!!")
	}
}
