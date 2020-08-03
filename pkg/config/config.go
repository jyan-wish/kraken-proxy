package config

import (
	"flag"
)

type Config struct {
	ListenPort      string
	DestinationPort string
	DesinationHost  string
}

func ParseFlags() *Config {
	var config Config
	flag.StringVar(&config.ListenPort, "listen-port", "2000", "port to listen on")
	flag.StringVar(&config.DesinationHost, "destination-host", "localhost", "host of registry to forward to")
	flag.StringVar(&config.DestinationPort, "destination-port", "5454", "port to listen on")
	flag.Parse()
	return &config
}
