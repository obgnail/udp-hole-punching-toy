package src

import (
	"fmt"
	"github.com/juju/errors"
	"io"
	"net"
	"strings"
)

var NetWorkClosedError = fmt.Errorf("network connection closed")

func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

func ListenTCP(addr string) (*net.TCPListener, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp4", addr)
	if err != nil {
		return nil, errors.Trace(err)
	}
	tcpListener, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return tcpListener, nil
}

func DialTCP(laddr, raddr string) (tcpConn *net.TCPConn, err error) {
	var lAddr, rAddr *net.TCPAddr
	if len(laddr) != 0 {
		if lAddr, err = net.ResolveTCPAddr("tcp4", laddr); err != nil {
			return nil, errors.Trace(err)
		}
	}
	if len(raddr) != 0 {
		if rAddr, err = net.ResolveTCPAddr("tcp4", raddr); err != nil {
			return nil, errors.Trace(err)
		}
	}
	tcpConn, err = net.DialTCP("tcp", lAddr, rAddr)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return tcpConn, nil
}

// will block until connection close
func Join(c1, c2 net.Conn) error {
	errChan := make(chan error, 2)
	pipe := func(c1, c2 net.Conn) {
		defer c1.Close()
		defer c2.Close()
		_, err := io.Copy(c1, c2)
		if err != nil {
			errChan <- err
		}
	}

	go pipe(c1, c2)
	go pipe(c2, c1)
	return <-errChan
}

func handlerCloseError(err error) error {
	if strings.Contains(err.Error(), "use of closed network connection") {
		return NetWorkClosedError
	}
	return errors.Trace(err)
}
