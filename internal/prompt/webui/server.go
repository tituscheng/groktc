package webui

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

// assetServer wraps a localhost http.Server that serves the embedded UI
// assets to the webview window.
type assetServer struct {
	srv  *http.Server
	addr string
}

func startAssetServer() (*assetServer, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("webui: listen: %w", err)
	}

	mux := http.NewServeMux()
	fileHandler := http.FileServer(http.FS(assetsFS()))
	mux.Handle("/", noCacheMiddleware(fileHandler))

	srv := &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		// http.ErrServerClosed is the normal shutdown signal.
		_ = srv.Serve(ln)
	}()

	return &assetServer{
		srv:  srv,
		addr: ln.Addr().String(),
	}, nil
}

func (s *assetServer) URL() string {
	return "http://" + s.addr + "/index.html"
}

func (s *assetServer) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.srv.Shutdown(ctx)
}

func noCacheMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		h.ServeHTTP(w, r)
	})
}
