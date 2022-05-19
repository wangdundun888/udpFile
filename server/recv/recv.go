package recv

import (
	"context"
	"log"
	"net"
	"server/util"
	"time"
)

func Recv(udpConn *net.UDPConn, upload, download chan util.IMessage, ctx context.Context) {
	if udpConn == nil {
		log.Printf("udpConn is nil\n")
		return
	}
	for {
		select {
		case <-ctx.Done():
			log.Printf("Recv goroutine exit\n")
			return
		default:
		}
		data := make([]byte, util.MaxLen+util.MessHeadLen, util.MaxLen+util.MessHeadLen)
		// set read timeout
		udpConn.SetDeadline(time.Now().Add(util.ReadTimeout))
		n, addr, err := udpConn.ReadFromUDP(data)
		if err != nil {
			//log.Printf("ReadFromUDP from %s, error: %s\n", addr.String(), err.Error())
			continue
		}
		if n < util.MessHeadLen {
			continue
		}
		go process(data[:n], upload, download, addr)
	}
}

func process(messData []byte, upload, download chan util.IMessage, addr *net.UDPAddr) {
	flag := messData[0] & 0x80
	mess := util.IMessage{
		Addr: addr,
		Data: messData,
	}
	switch flag {
	case util.UploadFlag:
		upload <- mess
	case util.DownloadFlag:
		download <- mess
	}
}
