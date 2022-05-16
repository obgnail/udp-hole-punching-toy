package main

import (
	"flag"
	"github.com/juju/errors"
	"github.com/obgnail/udp-hole-punching-toy/src"
	log "github.com/sirupsen/logrus"
)

var (
	id         string
	localPort  int
	passive    bool
	proxyPort  int
	serverIP   string
	serverPort int
	choose     string
)

func init() {
	flag.StringVar(&id, "id", "", "ID")
	flag.IntVar(&localPort, "localPort", 0, "localPort")
	flag.BoolVar(&passive, "passive", false, "passive")
	flag.IntVar(&proxyPort, "proxyPort", 0, "proxyPort")
	flag.StringVar(&serverIP, "serverIP", "127.0.0.1", "serverIP")
	flag.IntVar(&serverPort, "serverPort", 7788, "serverPort")
	flag.StringVar(&choose, "choose", "HTTP_SERVER", "choose")
	flag.Parse()
}

func main() {
	// usage:
	// 1. python3 -m http.server 7777
	// 2. cd demo/server && go run *.go
	// 3. cd demo/peer && go run *.go -id=HTTP_SERVER -passive=true -proxyPort=7777
	// 4. cd demo/peer && go run *.go -choose=HTTP_SERVER -passive=false -proxyPort=6666
	// 5. curl http://127.0.0.1:6666
	clt, err := src.NewPeer(id, passive, choose, localPort, proxyPort, serverIP, serverPort)
	if err != nil {
		log.Error(errors.ErrorStack(err))
		return
	}
	clt.Run()
}
