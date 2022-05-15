package main

import (
	"github.com/juju/errors"
	"github.com/obgnail/udp-hole-punching-toy/src"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.Error(errors.ErrorStack(src.NewServer(7788).Listen()))
}
