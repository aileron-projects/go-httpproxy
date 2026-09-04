<!-- markdownlint-disable MD033 MD041 -->

<div align="center">

[![Release](https://img.shields.io/github/v/release/aileron-projects/go-httpproxy?sort=semver)](https://github.com/aileron-projects/go-httpproxy/releases)
[![Reference](https://pkg.go.dev/badge/github.com/aileron-projects/go-httpproxy.svg)](https://pkg.go.dev/github.com/aileron-projects/go-httpproxy)
[![DeepWiki](https://deepwiki.com/badge.svg)](https://deepwiki.com/aileron-projects/go-httpproxy)
[![Test](https://github.com/aileron-projects/go-httpproxy/actions/workflows/test.yaml/badge.svg)](https://github.com/aileron-projects/go-httpproxy/actions/workflows/test.yaml)

[![Insights](https://badgen.net/badge/Insights/open%2Fsource%2Finsights/cyan)](https://deps.dev/go/github.com%2Faileron-projects%2Fgo-httpproxy)
[![Insights](https://badgen.net/badge/Insights/OSS%2FInsight/orange)](https://ossinsight.io/analyze/aileron-projects/go-httpproxy)

</div>

# go-httpproxy

**Flexible and powerful http proxy library for Go.**

## Features

- Reverse proxy handler.
- Custom error handler
- No panics (unlike net/http/httputil.ReverseProxy).
- Proxy streaming requests and responses.
  - **WebSocket** is supported
  - **SSE (Server Sent Event)** is supported
  - And more
- 101 protocol upgrade support

## Usages

### Built-in round-robin proxy

This example shows how to use built-in simple round-robin proxy.

```go
proxy, err := httpproxy.NewProxy("http://localhost:8081/", "http://localhost:8082/")
if err != nil {
    panic(err)
}

log.Println("proxy server is listening at: localhost:8080")
if err := http.ListenAndServe(":8080", proxy); err != nil && err != http.ErrServerClosed {
    panic(err)
}
```

### Rewrite request target

To change the proxy targets, or proxy bachends, use `PreRoundTrip` hook.

Target urls, request context and request body and other request info can be modified.

```go
target, _ := url.Parse("http://localhost:9999")

proxy := &Proxy{
    PreRoundTrip: func(pr *ProxyRequest) error {
        httpproxy.RewriteURL(pr.Out.URL, target)
        httpproxy.SetForwardedHeader(pr.In, pr.Out.Header)
        httpproxy.SetXForwardedHeaders(pr.In, pr.Out.Header)
        return nil
    }
}
```

### Custom error handler

Use `ErrorHandler` of the Proxy object.

For example:

```go
target, _ := url.Parse("http://localhost:9999")
proxy := &httpproxy.Proxy{
    PreRoundTrip: func(pr *httpproxy.ProxyRequest) error {
        httpproxy.RewriteURL(pr.Out.URL, target)
        return nil
    },
    ErrorHandler: func(w http.ResponseWriter, r *http.Request, err *httpproxy.Error) {
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
    },
}
```

## Docs & Examples

- GoDoc: <https://pkg.go.dev/github.com/aileron-projects/go-httpproxy>
- Examples:
  - [examples/rest/](./examples/rest/): an example of proxying rest apis
  - [examples/sse/](./examples/sse/): an example of proxying sse
  - [examples/websocket/](./examples/websocket/): an example of proxying websocket
  - [examples/external-service/](./examples/external-service/): an example of proxying request to external apis
  - [examples/error-handler/](./examples/error-handler/): an example of custom error handling

## References

- [net/http/httputil](https://pkg.go.dev/net/http/httputil)
