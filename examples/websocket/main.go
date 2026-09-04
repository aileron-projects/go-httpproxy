package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aileron-projects/go-httpproxy"
	"golang.org/x/net/websocket"
)

func main() {
	// Run SSE server as backend.
	go func() {
		log.Println("WebSocket server is listening at: localhost:9090")
		if err := http.ListenAndServe(":9090", websocket.Handler(websocketHandler)); err != nil && err != http.ErrServerClosed {
			panic(err)
		}
		log.Println("WebSocket server closed")
	}()

	// Instanciate a proxy.
	proxy, err := httpproxy.NewProxy("http://localhost:9090")
	if err != nil {
		panic(err)
	}

	log.Println("proxy server is listening at: localhost:8080")
	mux := &http.ServeMux{}
	mux.Handle("/ws", proxy)
	mux.Handle("/", http.FileServer(http.Dir("./")))
	if err := http.ListenAndServe(":8080", mux); err != nil && err != http.ErrServerClosed {
		panic(err)
	}
}

func websocketHandler(ws *websocket.Conn) {
	defer ws.Close()

	err := websocket.Message.Send(ws, "Hello!! This is a WebSocket server!!")
	if err != nil {
		panic(err)
	}

	done := make(chan struct{})

	// Receive message from client.
	go func() {
		for {
			msg := ""
			err = websocket.Message.Receive(ws, &msg)
			if err != nil {
				log.Println(err)
				close(done)
				return
			}
			err := websocket.Message.Send(ws, "Your message arrived: "+msg)
			if err != nil {
				log.Println(err)
				close(done)
				return
			}
		}
	}()

	// Send message to client.
	for {
		select {
		case <-ws.Request().Context().Done():
			return
		case <-done:
			return
		case <-time.After(time.Second):
		}
		err := websocket.Message.Send(ws, fmt.Sprintf("It's %s\n", time.Now().Format(http.TimeFormat)))
		if err != nil {
			log.Println(err)
			close(done)
			break
		}
	}
}
