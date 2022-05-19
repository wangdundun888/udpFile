package download

import (
	"client/util"
	"context"
	"encoding/binary"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

func Download(storagePath, fileName string,
	recv, send chan util.IMessage,
	addr *net.UDPAddr, ctx context.Context) {
	// send init message and wait response
	initData := make([]byte, util.MessHeadLen)
	initData[0] = util.DownloadFlag | util.Init
	initData = append(initData, []byte(fileName)...)
	var downloadId, size uint16
	try := 0
	for try <= util.MaxDownloadTry {
		log.Printf("Connect to download file system %s %dth time", fileName, try)
		try++
		initMess := util.IMessage{
			Addr: addr,
			Data: initData,
		}
		send <- initMess
		timer := time.NewTicker(time.Second * 2)
		select {
		case <-timer.C:
			continue
		case resp := <-recv:
			respData := resp.Data
			if len(respData) < util.MessHeadLen {
				continue
			}
			respAck := respData[0] & 0x7f
			switch respAck {
			case util.FileNoExist:
				log.Printf("file %s no exists", fileName)
				return
			case util.InitAck:
			default:
				continue
			}
			downloadId = binary.BigEndian.Uint16(respData[util.MessIdIndex : util.MessIdIndex+2])
			size = binary.BigEndian.Uint16(respData[util.MessLenIndex : util.MessLenIndex+2])
			try = util.MaxDownloadTry * 2
		}
	}
	log.Printf("Begin to download %s,wait", fileName)
	dataSlice := make([][]byte, size)
	ackMap := make(map[uint16]struct{})
	ackLen := size
	// init ackMap
	for i := uint16(0); i < ackLen; i++ {
		ackMap[i] = struct{}{}
	}
	timeout := time.Tick(util.DownloadTimeout)
	downloadDisplay := float32(0)
	for {
		again := time.Tick(time.Second * 20)
		select {
		case <-ctx.Done():
			return
		case resp := <-recv:
			respData := resp.Data
			if len(respData) < util.MessHeadLen {
				continue
			}
			respAck := respData[0] & 0x7f
			switch respAck {
			case util.Normal:
			default:
				continue
			}
			index := binary.BigEndian.Uint16(respData[util.MessLenIndex : util.MessLenIndex+2])
			index = index % size
			_, exist := ackMap[index]
			if !exist {
				continue
			}
			delete(ackMap, index)
			dataSlice[index] = append(dataSlice[index], respData[util.MessHeadLen:]...)
			downloadProcess := float32(ackLen-uint16(len(ackMap))) / float32(ackLen)
			if downloadProcess*100 >= downloadDisplay {
				log.Printf("download process: %.2f%%\n", downloadProcess*100)
				downloadDisplay += 10
			}
			if len(ackMap) == 0 {
				downloadDisplay = 0
				downloadFile := util.DownloadFile{
					FileName: fileName,
					Data:     dataSlice,
				}
				storage(storagePath, downloadFile)
				return
			}
		case <-again:
			for index, _ := range ackMap {
				var downloadBytes []byte
				downloadBytes = append(downloadBytes, util.DownloadFlag|util.DownloadSomeone)
				idBytes := make([]byte, 2, 2)
				binary.BigEndian.PutUint16(idBytes, downloadId)
				downloadBytes = append(downloadBytes, idBytes...)
				indexLen := make([]byte, 2, 2)
				binary.BigEndian.PutUint16(indexLen, index)
				downloadBytes = append(downloadBytes, indexLen...)
				go func(bytes []byte) {
					mess := util.IMessage{
						Addr: addr,
						Data: bytes,
					}
					send <- mess
				}(downloadBytes)
			}
		case <-timeout:
			log.Printf("download fail with timeout")
			return
		}
	}
}

func storage(path string, downloadFile util.DownloadFile) {
	path = strings.TrimRight(path, string(os.PathSeparator))
	path = path + string(os.PathSeparator) + downloadFile.FileName
	try := 0
	var data []byte
	for _, bytes := range downloadFile.Data {
		data = append(data, bytes...)
	}
	for try < util.MaxStorageTry {
		try++
		f, err := os.Create(path)
		if err != nil {
			log.Printf("Failed to store %s %dth time", downloadFile.FileName, try)
			continue
		}
		n, err := f.Write(data)
		if err != nil || n != len(data) {
			log.Printf("Failed to store %s %dth time", downloadFile.FileName, try)
			continue
		}
		break
	}
}
