package main

import (
	"fmt"
	"net/http"

	"github.com/jyan-wish/kraken-proxy/pkg/config"
	"github.com/jyan-wish/kraken-proxy/pkg/proxy"
)

func main() {
	config := config.ParseFlags()
	proxy := proxy.GenerateProxy(config)
	s := &http.Server{
		Addr:    fmt.Sprintf(":%s", config.ListenPort),
		Handler: proxy,
	}
	fmt.Printf("Serving on port %s\n", config.ListenPort)
	s.ListenAndServe()
}
