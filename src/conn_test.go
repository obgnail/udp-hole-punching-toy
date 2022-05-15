package src

import (
	"net"
	"testing"
)


func TestBuildClientListMsg(t *testing.T) {
	clientList := map[string]*net.UDPAddr{
		"name1":{IP: net.IPv4zero, Port: 7777},
		"name2":{IP: net.IPv4zero, Port: 8888},
	}
	res := BuildPeerListMsg(clientList)
	if string(res) != "37?name1=0.0.0.0:7777&name2=0.0.0.0:8888" {
		t.Fatal(string(res))
	}

	clientList2 := make(map[string]*net.UDPAddr)
	res2 := BuildPeerListMsg(clientList2)
	if string(res2) != "0?" {
		t.Fatal(string(res2))
	}
}
