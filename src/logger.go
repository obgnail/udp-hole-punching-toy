package src

import (
	"fmt"
	log "github.com/sirupsen/logrus"
)

const (
	BlueFormat   = "\u001B[34m%s\u001B[0m"
	YellowFormat = "\u001B[33m%s\u001B[0m"
	RedFormat    = "\u001B[31m%s\u001B[0m"
	GreenFormat  = "\u001B[32m%s\u001B[0m"

	GreenBackWhiteTextFormat = "\u001B[30;46m%s\u001B[0m"
)

var (
	PeerStr    = fmt.Sprintf(YellowFormat, "Peer")
	ServerStr  = fmt.Sprintf(RedFormat, "Server")
	LocalStr   = fmt.Sprintf(GreenFormat, "Local")
	RemoteStr  = fmt.Sprintf(BlueFormat, "Remote")
	SubPeerStr = fmt.Sprintf(YellowFormat, "SubPeer")
)

func init() {
	customFormatter := new(log.TextFormatter)
	customFormatter.FullTimestamp = true                        // 显示完整时间
	customFormatter.TimestampFormat = "2006-01-02 15:04:05.000" // 时间格式
	customFormatter.DisableTimestamp = false                    // 禁止显示时间
	customFormatter.DisableColors = false                       // 禁止颜色显示

	log.SetFormatter(customFormatter)
	log.SetLevel(log.DebugLevel)
}
