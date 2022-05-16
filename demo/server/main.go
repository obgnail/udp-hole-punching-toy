package main

import (
	"flag"
	"github.com/juju/errors"
	"github.com/obgnail/udp-hole-punching-toy/src"
	log "github.com/sirupsen/logrus"
)

var (
	serverPort int
)

func init() {
	flag.IntVar(&serverPort, "serverPort", 7788, "serverPort")
	flag.Parse()
}

func main() {
	log.Error(errors.ErrorStack(src.NewServer(serverPort).Listen()))
}
