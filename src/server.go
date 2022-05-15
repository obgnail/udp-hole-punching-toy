package src

import (
	"fmt"
	"github.com/juju/errors"
	log "github.com/sirupsen/logrus"
	"net"
	"strings"
	"sync"
	"time"
)

const (
	// 过了这段时间没有响应,就从Server中移除此Peer
	peerExpireDuration = 20 * time.Second
)

type peer struct {
	id         string
	addr       *net.UDPAddr
	updateTime time.Time
}

type Server struct {
	port  int
	peers sync.Map // map[peerID]*peer
}

func NewServer(port int) *Server {
	return &Server{port: port}
}

func (s *Server) storePeer(peerID string, addr *net.UDPAddr) {
	s.peers.Store(peerID, &peer{
		id:         peerID,
		addr:       addr,
		updateTime: time.Now(),
	})
}

func (s *Server) getOtherPeers(exclude string) map[string]*net.UDPAddr {
	m := make(map[string]*net.UDPAddr)
	s.peers.Range(func(key, value interface{}) bool {
		id, peer := key.(string), value.(*peer)
		if id != exclude {
			m[id] = peer.addr
		}
		return true
	})
	return m
}

func (s *Server) getPeerAddr(peerID string) (*net.UDPAddr, error) {
	ret, ok := s.peers.Load(peerID)
	if !ok {
		return nil, fmt.Errorf("no such peer id: %s", peerID)
	}
	return ret.(*peer).addr, nil
}

func (s *Server) deleteExpirePeer() {
	timeout := peerExpireDuration.Seconds()
	for {
		var toDelete []string
		s.peers.Range(func(key, value interface{}) bool {
			id, peer := key.(string), value.(*peer)
			if time.Since(peer.updateTime).Seconds() > timeout {
				toDelete = append(toDelete, id)
			}
			return true
		})
		for _, id := range toDelete {
			s.peers.Delete(id)
		}
		time.Sleep(time.Second * 10)
	}
}

func (s *Server) handleConn(listener *net.UDPConn, remoteAddr *net.UDPAddr, typ string, content string) error {
	switch typ {
	case FlagHeartbeat:
		s.storePeer(content, remoteAddr)
		heartbeatMsg := BuildHeartbeatMsg("NULL")
		if _, err := listener.WriteTo(heartbeatMsg, remoteAddr); err != nil {
			return errors.Trace(err)
		}
	case FlagRegisterPeer:
		// content: peerID
		log.Debugf("%s -> %s | %s | %s", PeerStr, ServerStr, remoteAddr.String(), "Register peer")
		s.storePeer(content, remoteAddr)
		otherPeers := s.getOtherPeers(content)
		peerListMsg := BuildPeerListMsg(otherPeers)
		if _, err := listener.WriteToUDP(peerListMsg, remoteAddr); err != nil {
			return errors.Trace(err)
		}
	case FlagPunchHole:
		// content: peerID=127.0.0.1:7777
		log.Debugf("%s -> %s | %s | %s", PeerStr, ServerStr, remoteAddr.String(), "Punch Hole")
		contentList := strings.Split(content, "=")
		if len(contentList) != 2 {
			return fmt.Errorf("error message format: %s", content)
		}
		peerID := contentList[0]
		toJoinPeerAddr, err := s.getPeerAddr(peerID)
		if err != nil {
			return errors.Trace(err)
		}
		msg := BuildPunchHoleMsg("NULL", remoteAddr.String())
		if _, err := listener.WriteToUDP(msg, toJoinPeerAddr); err != nil {
			return errors.Trace(err)
		}
	default:
		return fmt.Errorf("error message type: %s, content %s", typ, content)
	}
	return nil
}

func (s *Server) Listen() error {
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: s.port})
	if err != nil {
		return errors.Trace(err)
	}
	log.Info("Server Listen ", fmt.Sprintf(GreenBackWhiteTextFormat, listener.LocalAddr().String()))
	go s.deleteExpirePeer()

	// 只是用来接收同步消息,不需要太大
	data := make([]byte, 1024)
	for {
		n, remoteAddr, err := listener.ReadFromUDP(data)
		if err != nil {
			log.Errorf("listener error during read: %s", err)
			continue
		}
		// why 10? len("[00000001]") == len("[00000002]") == 10
		typ, content := string(data[:10]), string(data[10:n])
		if err := s.handleConn(listener, remoteAddr, typ, content); err != nil {
			log.Error(errors.ErrorStack(err))
			continue
		}
	}
}
