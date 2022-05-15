package src

import (
	"bytes"
	"fmt"
	"net"
)

const (
	FlagHeartbeat    = "[00000001]"
	FlagPunchHole    = "[00000002]"
	FlagRegisterPeer = "[00000003]"
	FlagPeerList     = "[00000004]"
	FlagConvert      = "[00000005]"
)

func BuildMessage(typ string, message []byte) []byte {
	buf := bytes.NewBufferString(typ)
	buf.Write(message)
	return buf.Bytes()
}

func BuildHeartbeatMsg(id string) []byte {
	return BuildMessage(FlagHeartbeat, []byte(id))
}

func BuildConvertMsg(msg []byte) []byte {
	return BuildMessage(FlagConvert, msg)
}

// [00000002]peer=127.0.0.1:7777
func BuildPunchHoleMsg(id string, addr string) []byte {
	buf := bytes.NewBufferString(id)
	buf.WriteByte('=')
	buf.WriteString(addr)
	return BuildMessage(FlagPunchHole, buf.Bytes())
}

// [00000003]peerID
func BuildRegisterMsg(id string) []byte {
	return BuildMessage(FlagRegisterPeer, []byte(id))
}

// [00000004]Len?name1=127.0.0.1:7777&name2=127.0.0.1:8888
func BuildPeerListMsg(peerList map[string]*net.UDPAddr) []byte {
	buf := bytes.NewBuffer(nil)
	for id, addr := range peerList {
		buf.WriteString(id)
		buf.WriteByte('=')
		buf.WriteString(addr.String())
		buf.WriteByte('&')
	}
	listBytes := buf.Bytes()
	listBytes = bytes.TrimRight(listBytes, "&")
	data := bytes.NewBufferString(fmt.Sprintf("%d", len(listBytes)))
	data.WriteByte('?')
	data.Write(listBytes)
	return BuildMessage(FlagPeerList, data.Bytes())
}
