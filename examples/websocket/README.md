# WebSocket proxy example

## About

This directory provides an example of proxying websocket.

- [index.html](./index.html): websocket client for testing
- [main.go](./main.go): runs proxy server and mock websocket server

## Run

```sh
go run ./main.go
```

This runs the following servers.

- `localhost:9090/`: mock websocket server
- `localhost:8080/ws`: expect websocket. proxy server proxies
- `localhost:8080/`: reverse proxy

## Test

- Access the mock websocket client page at `localhost:8080`
- Fill a message and submit it
- Then server answeres to your message
