package src

import (
	"testing"
	"time"
)

func TestTimeout(t *testing.T) {
	timeout := float64(2)
	now := time.Now()
	time.Sleep(3 * time.Second)
	i := time.Since(now).Seconds()
	if i <= timeout {
		t.Fatal("err", time.Since(now).Seconds())
	}
}

//func TestParseIP(t *testing.T) {
//	host := "127.0.0.1:7777"
//	ip := net.ParseIP(host)
//	if ip.String() != "127.0.0.1" {
//		t.Fatal("err", ip.String())
//	}
//}
