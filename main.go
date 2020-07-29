package main

import (
	"github.com/jyan-wish/kraken-proxy/pkg/config"
	"github.com/jyan-wish/kraken-proxy/pkg/proxy"
)

func main() {
	config := config.ParseFlags()
	proxy.StartProxy(config)
}
