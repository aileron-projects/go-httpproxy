package main

import (
	"log"
	"net/http"

	"github.com/aileron-projects/go-httpproxy"
)

func main() {
	proxy, err := httpproxy.NewProxy("http://httpbin.org/")
	// proxy, err := httpproxy.NewProxy("https://httpbun.com/")
	// proxy, err := httpproxy.NewProxy("http://echo.free.beeceptor.com/")
	// proxy, err := httpproxy.NewProxy("https://gen-endpoint.com/")
	if err != nil {
		panic(err)
	}

	log.Println("proxy server is listening at: localhost:8080")
	if err := http.ListenAndServe(":8080", proxy); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}
