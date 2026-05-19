package proxy

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
)

type wsProxy struct {
	target *url.URL
}

func newWebSocketProxy(target *url.URL) *wsProxy {
	return &wsProxy{target: target}
}

func (p *wsProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	host := p.target.Host
	if p.target.Port() == "" {
		if p.target.Scheme == "https" || p.target.Scheme == "wss" {
			host += ":443"
		} else {
			host += ":80"
		}
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "server does not support websocket hijack", http.StatusInternalServerError)
		return
	}
	clientConn, clientBuf, err := hj.Hijack()
	if err != nil {
		slog.Error("ws hijack", "err", err)
		return
	}
	defer clientConn.Close()

	upConn, err := net.Dial("tcp", host)
	if err != nil {
		slog.Error("ws upstream dial", "host", host, "err", err)
		fmt.Fprintf(clientConn, "HTTP/1.1 502 Bad Gateway\r\nContent-Length: 0\r\n\r\n")
		return
	}
	defer upConn.Close()

	outReq := r.Clone(r.Context())
	outReq.RequestURI = ""
	outReq.URL.Scheme = wsUpstreamScheme(p.target.Scheme)
	outReq.URL.Host = p.target.Host

	if err := outReq.Write(upConn); err != nil {
		slog.Error("ws write upgrade request", "err", err)
		return
	}

	upBuf := bufio.NewReader(upConn)
	resp, err := http.ReadResponse(upBuf, outReq)
	if err != nil {
		slog.Error("ws read upstream response", "err", err)
		return
	}
	if resp.Body != nil {
		resp.Body.Close()
	}

	if err := resp.Write(clientBuf); err != nil {
		slog.Error("ws write upgrade response to client", "err", err)
		return
	}
	if err := clientBuf.Flush(); err != nil {
		slog.Error("ws flush client response", "err", err)
		return
	}

	upReader := io.MultiReader(upBuf, upConn)
	clientReader := io.MultiReader(clientBuf.Reader, clientConn)

	errc := make(chan error, 2)
	go func() { _, err := io.Copy(upConn, clientReader); errc <- err }()
	go func() { _, err := io.Copy(clientConn, upReader); errc <- err }()
	<-errc
}

func wsUpstreamScheme(scheme string) string {
	if scheme == "https" || scheme == "wss" {
		return "wss"
	}
	return "ws"
}
