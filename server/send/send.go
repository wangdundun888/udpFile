package send

import (
	"context"
	"log"
	"net"
	"server/util"
)

func Send(udpConn *net.UDPConn, send chan util.IMessage, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("Send goroutine exit\n")
			return
		case mess := <-send:
			go udpConn.WriteToUDP(mess.Data, mess.Addr)
		}
	}
}
