package src

import (
	"bufio"
	"fmt"
	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	startListenTCPTiming = 3
	startDialTCPTiming   = 4
)

const (
	heartbeatInterval = 5 * time.Second
)

type Peer struct {
	id string
	// 常驻server端
	keepalive bool
	// 打洞后:
	// 对于充当服务端的peer,将LocalAddr的消息代理到proxyPort,由proxyPort提供服务
	// 对于充当客户端的peer,将proxyPort的消息代理到LocalAddr,由proxyPort请求数据
	proxyPort  int
	serverAddr *net.UDPAddr
	LocalAddr  *net.UDPAddr
}

func NewPeer(id string, localPort int, keepalive bool, proxyLocalPort int, serverIP string, serverPort int) (clt *Peer, err error) {
	if len(id) == 0 {
		id = strconv.FormatInt(time.Now().UnixNano(), 10) // 默认使用时间戳作为id
	}
	if localPort == 0 {
		localPort, err = GetFreePort()
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	localAddr := &net.UDPAddr{IP: net.IPv4zero, Port: localPort}
	serverAddr := &net.UDPAddr{IP: net.ParseIP(serverIP), Port: serverPort}
	clt = &Peer{
		id:         id,
		keepalive:  keepalive,
		proxyPort:  proxyLocalPort,
		serverAddr: serverAddr,
		LocalAddr:  localAddr,
	}
	return clt, nil
}

func (p *Peer) heartbeatToServer(listener *net.UDPConn) error {
	for {
		heartbeatMsg := BuildHeartbeatMsg(p.id)
		//log.Debug("send heartbeat: ", string(heartbeatMsg))
		if _, err := listener.WriteTo(heartbeatMsg, p.serverAddr); err != nil {
			return errors.Trace(err)
		}
		time.Sleep(heartbeatInterval)
	}
}

func (p *Peer) registerPeer(listener *net.UDPConn) (err error) {
	registerMsg := BuildRegisterMsg(p.id)
	if _, err = listener.WriteTo(registerMsg, p.serverAddr); err != nil {
		return errors.Trace(err)
	}
	if p.keepalive {
		//go errors.ErrorStack(errors.Trace(p.heartbeatToServer(listener)))
	}
	return nil
}

func (p *Peer) PunchingHole(listener *net.UDPConn, otherClientID string, otherClientAddr string) error {
	clientAddr, err := net.ResolveUDPAddr("udp", otherClientAddr)
	if err != nil {
		return fmt.Errorf("no such addr: %s", otherClientAddr)
	}
	msg := BuildPunchHoleMsg(otherClientID, clientAddr.String())
	if _, err = listener.WriteTo(msg, p.serverAddr); err != nil {
		return errors.Trace(err)
	}
	// 为了打开NAT通道
	invalidMsg := BuildHeartbeatMsg(p.id)
	if _, err = listener.WriteTo(invalidMsg, clientAddr); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (p *Peer) getPeerListFromMsg(msg string) map[string]string {
	// 37?name1=0.0.0.0:7777&name2=0.0.0.0:8888
	temp := strings.Split(msg, "?")
	Len, PeerList := temp[0], temp[1]
	if Len == "0" {
		return nil
	}
	ret := make(map[string]string)
	for _, client := range strings.Split(PeerList, "&") {
		t := strings.Split(client, "=")
		name, addr := t[0], t[1]
		ret[name] = addr
	}
	return ret
}

func (p *Peer) choosePeer(src map[string]string) (id, addr string) {
	input := bufio.NewScanner(os.Stdin)

	fmt.Println("\n============ PEERS ============")
	for id, addr := range src {
		fmt.Printf("id: %s \t addr: %s\n", id, addr)
	}
	fmt.Printf("\nPlease type peer id to connect: ")
	for input.Scan() {
		line := input.Text()
		if addr, ok := src[strings.TrimSpace(line)]; ok {
			return line, addr
		}
	}
	return "", ""
}

func (p *Peer) parsePunchHoleMsg(msg string) (*net.UDPAddr, error) {
	// peerID=127.0.0.1:7777
	contentList := strings.Split(msg, "=")
	if len(contentList) != 2 {
		return nil, fmt.Errorf("error message format: %s", msg)
	}
	peerAddr, err := net.ResolveUDPAddr("udp", contentList[1])
	if err != nil {
		return nil, errors.Trace(err)
	}
	return peerAddr, nil
}

func (p *Peer) proxyRemoteToLocal(UDPListener *net.UDPConn) error {
	if err := UDPListener.Close(); err != nil {
		return errors.Trace(err)
	}

	// listen remote
	tcpListener, err := ListenTCP(p.LocalAddr.String())
	if err != nil {
		return errors.Trace(err)
	}
	log.Info("TCP Listen: ", fmt.Sprintf(GreenBackWhiteTextFormat, tcpListener.Addr().String()))

	// proxy to local
	proxyConn, err := DialTCP("", fmt.Sprintf("127.0.0.1:%d", p.proxyPort))
	if err != nil {
		return errors.Trace(err)
	}
	for {
		tcpConn, err := tcpListener.AcceptTCP()
		if err != nil {
			continue
		}
		log.Debugf(
			"Proxy: [%s -> %s](%s) <-> [%s -> %s](%s)",
			tcpConn.LocalAddr(),
			tcpConn.RemoteAddr(),
			RemoteStr,
			proxyConn.LocalAddr(),
			proxyConn.RemoteAddr(),
			LocalStr,
		)
		if err := Join(proxyConn, tcpConn); err != nil {
			err := errors.Trace(err)
			log.Error(errors.ErrorStack(err))
		}
	}
}

func (p *Peer) proxyLocalToRemote(UDPListener *net.UDPConn, remoteAddr *net.UDPAddr) error {
	if err := UDPListener.Close(); err != nil {
		return errors.Trace(err)
	}
	// connect to remote
	tcpConn, err := DialTCP(UDPListener.LocalAddr().String(), remoteAddr.String())
	if err != nil {
		return errors.Trace(err)
	}

	// connect to proxy
	proxyListener, err := ListenTCP(fmt.Sprintf("127.0.0.1:%d", p.proxyPort))
	if err != nil {
		return errors.Trace(err)
	}
	log.Info("Client Proxy Listen: ", fmt.Sprintf(GreenBackWhiteTextFormat, proxyListener.Addr().String()))
	for {
		localConn, err := proxyListener.AcceptTCP()
		if err != nil {
			continue
		}
		log.Debugf(
			"Proxy: [%s -> %s](%s) <-> [%s -> %s](%s)",
			localConn.RemoteAddr(),
			localConn.LocalAddr(),
			LocalStr,
			tcpConn.LocalAddr(),
			tcpConn.RemoteAddr(),
			RemoteStr,
		)
		if err := Join(localConn, tcpConn); err != nil {
			err := errors.Trace(err)
			log.Error(errors.ErrorStack(err))
		}
	}
}

func (p *Peer) HandlerConn(listener *net.UDPConn, remoteAddr *net.UDPAddr, typ, content string) error {
	switch typ {
	case FlagHeartbeat:
		// heartbeat信息,有必要可以处理
		log.Debug("received--:", content)
	case FlagPeerList:
		log.Debugf("%s -> %s | %s -> %s | %s", ServerStr, PeerStr, remoteAddr.String(), listener.LocalAddr().String(), "Get Peer List")
		// 37?name1=0.0.0.0:7777&name2=0.0.0.0:8888
		peers := p.getPeerListFromMsg(content)
		if peers == nil {
			log.Debug("you are the first peer")
			return nil
		}
		id, addr := p.choosePeer(peers)
		return errors.Trace(p.PunchingHole(listener, id, addr))
	case FlagPunchHole:
		log.Debugf("%s -> %s | %s -> %s | %s", ServerStr, PeerStr, remoteAddr.String(), listener.LocalAddr().String(), "Try To Punch Hole")
		targetAddr, err := p.parsePunchHoleMsg(content)
		if err != nil {
			return errors.Trace(err)
		}
		handshakeCount := []byte{'1'}
		msg := BuildConvertMsg(handshakeCount)
		if _, err := listener.WriteToUDP(msg, targetAddr); err != nil {
			return errors.Trace(err)
		}
		// udp -> tcp
	case FlagConvert:
		time.Sleep(time.Second) // 等待发送成功
		log.Debug("handshake: ", content)
		handshakeCount, err := strconv.Atoi(content)
		if err != nil {
			return errors.Trace(err)
		}
		handshakeCount++
		resp := strconv.Itoa(handshakeCount)
		msg := BuildConvertMsg([]byte(resp))
		// 此时需要使用UDP的端口Dail对方的TCP端口,所以不再发送UDP请求
		if handshakeCount != startDialTCPTiming {
			if _, err := listener.WriteToUDP(msg, remoteAddr); err != nil {
				return errors.Trace(err)
			}
		}
		if handshakeCount == startListenTCPTiming {
			return errors.Trace(p.proxyRemoteToLocal(listener))
		}
		if handshakeCount == startDialTCPTiming {
			return errors.Trace(p.proxyLocalToRemote(listener, remoteAddr))
		}
	default:
		return fmt.Errorf("error message type: %s, content %s", typ, content)
	}
	return nil
}

func (p *Peer) Dial() error {
	log.Infof("Peer %s Listen %s", fmt.Sprintf(GreenBackWhiteTextFormat, p.id), fmt.Sprintf(GreenBackWhiteTextFormat, p.LocalAddr.String()))
	listener, err := net.ListenUDP("udp", p.LocalAddr)
	if err != nil {
		return errors.Trace(err)
	}
	if err = p.registerPeer(listener); err != nil {
		return errors.Trace(err)
	}
	data := make([]byte, 1024)
	for {
		n, remoteAddr, err := listener.ReadFromUDP(data)
		if err != nil {
			return errors.Trace(err)
		}
		// why 10? len("[00000001]") == len("[00000002]") == 10
		typ, content := string(data[:10]), string(data[10:n])
		if err := p.HandlerConn(listener, remoteAddr, typ, content); err != nil {
			return errors.Trace(err)
		}
	}
}
