package main

import (
	"github.com/juju/errors"
	"github.com/obgnail/udp-hole-punching-toy/src"
	log "github.com/sirupsen/logrus"
	"os"
	"strconv"
)

func main() {
	// python3 -m http.server 7777
	// go run *.go 7777
	// go run *.go 6666
	// curl http://127.0.0.1:6666
	proxyPort, err := strconv.Atoi(os.Args[1])
	if err != nil {
		log.Error(errors.ErrorStack(err))
		return
	}
	clt, err := src.NewPeer("", 0, true, proxyPort, "127.0.0.1", 7788)
	if err != nil {
		log.Error(errors.ErrorStack(err))
		return
	}
	if err := clt.Dial(); err != nil {
		log.Error(errors.ErrorStack(err))
	}
}
