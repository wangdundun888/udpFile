package send

import (
	"client/util"
	"context"
	"log"
	"net"
)

func Send(udpConn *net.UDPConn, send chan util.IMessage, ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("Send goroutine exit\n")
			return
		case mess := <-send:
			go udpConn.Write(mess.Data)

		}
	}
}
