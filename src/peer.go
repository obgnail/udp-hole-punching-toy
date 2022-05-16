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

var peerScheduler chan *Peer

func init() {
	peerScheduler = make(chan *Peer, 1)
}

type Peer struct {
	id string
	// 被动(不会主动搜寻其他peer)
	passive bool
	// 默认选择的peerID.只用于主动的peer
	choose string
	// 打洞后:
	// 对于被动的peer,将LocalAddr的消息代理到proxyPort,由proxyPort提供服务
	// 对于主动的peer,将proxyPort的消息代理到LocalAddr,由proxyPort请求数据
	proxyPort  int
	serverAddr *net.UDPAddr
	localAddr  *net.UDPAddr

	stopHeartbeat chan struct{} // send one when stop keepalive
}

func NewPeer(id string, passive bool, choose string, localPort int, proxyPort int, serverIP string, serverPort int) (p *Peer, err error) {
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
	p = &Peer{
		id:            id,
		passive:       passive,
		proxyPort:     proxyPort,
		serverAddr:    serverAddr,
		localAddr:     localAddr,
		choose:        choose,
		stopHeartbeat: make(chan struct{}, 1),
	}
	log.Debugf("%s: %s | passive: %t | %s -> %s", PeerStr, p.id, p.passive, fmt.Sprintf(GreenBackWhiteTextFormat, p.localAddr), p.serverAddr)
	return p, nil
}

func (p *Peer) genSubPeer() error {
	localPort, err := GetFreePort()
	if err != nil {
		return errors.Trace(err)
	}
	subPeer := &Peer{
		id:            p.id,
		passive:       p.passive,
		choose:        p.choose,
		proxyPort:     p.proxyPort,
		serverAddr:    p.serverAddr,
		localAddr:     &net.UDPAddr{IP: net.IPv4zero, Port: localPort},
		stopHeartbeat: make(chan struct{}, 1),
	}
	log.Debugf("%s: %s | passive: %t | %s -> %s", SubPeerStr, subPeer.id, subPeer.passive, fmt.Sprintf(GreenBackWhiteTextFormat, subPeer.localAddr), subPeer.serverAddr)
	peerScheduler <- subPeer
	return nil
}

func (p *Peer) registerPeer(listener *net.UDPConn) (err error) {
	registerMsg := BuildRegisterMsg(p.id)
	if _, err = listener.WriteToUDP(registerMsg, p.serverAddr); err != nil {
		return errors.Trace(err)
	}
	return nil
}

func (p *Peer) closeKeepalive() {
	if p.passive {
		p.stopHeartbeat <- struct{}{}
	}
}

func (p *Peer) keepPeerAlive(listener *net.UDPConn) {
	if !p.passive {
		return
	}

	// heartbeatToServer
	go func() {
		for {
			select {
			case <-p.stopHeartbeat:
				return
			default:
				heartbeatMsg := BuildHeartbeatMsg(p.id)
				if _, err := listener.WriteToUDP(heartbeatMsg, p.serverAddr); err != nil {
					log.Error(errors.ErrorStack(err))
					continue
				}
				time.Sleep(heartbeatInterval)
			}
		}
	}()
}

func (p *Peer) punchingHole(listener *net.UDPConn, otherClientID string, otherClientAddr string) error {
	clientAddr, err := net.ResolveUDPAddr("udp", otherClientAddr)
	if err != nil {
		return fmt.Errorf("no such addr: %s", otherClientAddr)
	}
	msg := BuildPunchHoleMsg(otherClientID, clientAddr.String())
	if _, err = listener.WriteToUDP(msg, p.serverAddr); err != nil {
		return errors.Trace(err)
	}
	// 为了打开NAT通道
	invalidMsg := BuildHeartbeatMsg(p.id)
	if _, err = listener.WriteToUDP(invalidMsg, clientAddr); err != nil {
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

	if addr, ok := src[p.choose]; ok {
		fmt.Printf("\nchoose: %s\n", p.choose)
		return p.choose, addr
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
	log.Info("Change UDP To TCP...")
	p.closeKeepalive()
	if err := UDPListener.Close(); err != nil {
		return errors.Trace(err)
	}
	log.Info("[OK] Close UDP")

	// listen remote
	tcpListener, err := ListenTCP(p.localAddr.String())
	if err != nil {
		return errors.Trace(err)
	}
	defer tcpListener.Close()
	log.Info("[OK] Start TCP Listen: ", fmt.Sprintf(GreenBackWhiteTextFormat, tcpListener.Addr().String()))

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
			"[OK] Proxy: [%s -> %s](%s) <-> [%s -> %s](%s)",
			tcpConn.LocalAddr(),
			tcpConn.RemoteAddr(),
			RemoteStr,
			proxyConn.LocalAddr(),
			proxyConn.RemoteAddr(),
			LocalStr,
		)
		if err := Join(proxyConn, tcpConn); err != nil {
			return errors.Trace(err)
		}
	}
}

func (p *Peer) proxyLocalToRemote(UDPListener *net.UDPConn, remoteAddr *net.UDPAddr) error {
	log.Info("Change UDP To TCP...")
	p.closeKeepalive()
	if err := UDPListener.Close(); err != nil {
		return errors.Trace(err)
	}
	log.Info("[OK] Close UDP")

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
	defer proxyListener.Close()
	log.Info("[OK] Start Proxy Listen: ", fmt.Sprintf(GreenBackWhiteTextFormat, proxyListener.Addr().String()))
	for {
		proxyConn, err := proxyListener.AcceptTCP()
		if err != nil {
			continue
		}
		log.Debugf(
			"[OK] Proxy: [%s -> %s](%s) <-> [%s -> %s](%s)",
			proxyConn.RemoteAddr(),
			proxyConn.LocalAddr(),
			LocalStr,
			tcpConn.LocalAddr(),
			tcpConn.RemoteAddr(),
			RemoteStr,
		)
		if err := Join(proxyConn, tcpConn); err != nil {
			return errors.Trace(err)
		}
	}
}

func (p *Peer) handlerConn(listener *net.UDPConn, remoteAddr *net.UDPAddr, typ, content string) error {
	switch typ {
	case FlagHeartbeat:
		// heartbeat信息,有必要可以处理(实际上为了保持简单,server只收不发,所以没有heartbeat信息)
		log.Debugf("%s -> %s | %s -> %s | %s", ServerStr, PeerStr, remoteAddr.String(), listener.LocalAddr().String(), "heartbeat")
	case FlagPeerList:
		log.Debugf("%s -> %s | %s -> %s | %s", ServerStr, PeerStr, remoteAddr.String(), listener.LocalAddr().String(), "Get Peer List")
		// serveOnly不需要choosePeer
		if p.passive {
			return nil
		}
		// 37?name1=0.0.0.0:7777&name2=0.0.0.0:8888
		peers := p.getPeerListFromMsg(content)
		if peers == nil {
			log.Debug("you are the first peer")
			return nil
		}
		id, addr := p.choosePeer(peers)
		return errors.Trace(p.punchingHole(listener, id, addr))
	case FlagPunchHole:
		log.Debugf("%s -> %s | %s -> %s | %s", ServerStr, PeerStr, remoteAddr.String(), listener.LocalAddr().String(), "Try To punch Hole")
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
		log.Debug("received handshake: ", content)
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

		// 启动新的peer
		if p.passive && handshakeCount == startListenTCPTiming {
			if err := p.genSubPeer(); err != nil {
				return errors.Trace(err)
			}
		}

		if handshakeCount == startListenTCPTiming {
			err := p.proxyRemoteToLocal(listener)
			return handlerCloseError(err)
		}
		if handshakeCount == startDialTCPTiming {
			err := p.proxyLocalToRemote(listener, remoteAddr)
			if !p.passive {
				if e := p.genSubPeer(); e != nil {
					return errors.Trace(e)
				}
			}
			return handlerCloseError(err)
		}
	default:
		return fmt.Errorf("error message type: %s, content %s", typ, content)
	}
	return nil
}

func (p *Peer) punch() error {
	//log.Infof("Peer %s Listen %s", fmt.Sprintf(GreenBackWhiteTextFormat, p.id), fmt.Sprintf(GreenBackWhiteTextFormat, p.localAddr.String()))
	listener, err := net.ListenUDP("udp", p.localAddr)
	if err != nil {
		return errors.Trace(err)
	}
	if err = p.registerPeer(listener); err != nil {
		return errors.Trace(err)
	}
	p.keepPeerAlive(listener)
	data := make([]byte, 1024)
	for {
		n, remoteAddr, err := listener.ReadFromUDP(data)
		if err != nil {
			return errors.Trace(err)
		}
		// why 10? len("[00000001]") == len("[00000002]") == 10
		typ, content := string(data[:10]), string(data[10:n])
		if err := p.handlerConn(listener, remoteAddr, typ, content); err != nil {
			if err == NetWorkClosedError {
				log.Warn(err)
				return nil
			}
			return errors.Trace(err)
		}
	}
}

func (p *Peer) Run() {
	peerScheduler <- p
	closeChan := make(chan struct{}, 1)
	go func() {
		for peer := range peerScheduler {
			log.Debugf("handler peer... | %s | %s", p.id, p.localAddr)
			go func(peer *Peer) {
				if err := peer.punch(); err != nil {
					log.Error(errors.ErrorStack(errors.Trace(err)))
				}
				// 有需要可以直接关闭
				//if !peer.passive {
				//	closeChan <- struct{}{}
				//}
				log.Debugf("peer was dead: %s | %s", peer.id, peer.localAddr)
			}(peer)
		}
	}()
	<-closeChan
}
