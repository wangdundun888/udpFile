package upload

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

func Upload(path, fileName string,
	recv, send chan util.IMessage,
	addr *net.UDPAddr, ctx context.Context) {
	// open file
	path = strings.TrimRight(path, string(os.PathSeparator))
	filePath := path + string(os.PathSeparator) + fileName
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("open file %s error：%s", filePath, err.Error())
		return
	}
	fileStat, err := file.Stat()
	if err != nil {
		log.Printf("get file %s info error：%s", filePath, err.Error())
		return
	}
	fileData := make([]byte, fileStat.Size())
	n, err := file.Read(fileData)
	defer file.Close()
	if err != nil {
		log.Println(err)
		return
	}
	fileData = fileData[:n]
	// send upload info and wait the response
	data := make([]byte, util.MessHeadLen)
	size := len(fileData)
	totalLen := uint16((size-1)/util.OnceDownloadSize) + 1
	binary.BigEndian.PutUint16(data[util.MessLenIndex:util.MessLenIndex+2], totalLen)
	data[0] = util.UploadFlag | util.Init
	data = append(data, []byte(fileName)...)
	try := 1
	var uploadId uint16
	for try <= util.MaxUploadTry {
		log.Printf("Connect to upload file system %s %dth time", fileName, try)
		try++
		initMess := util.IMessage{
			Addr: addr,
			Data: data,
		}
		send <- initMess
		timer := time.NewTicker(time.Second * 5)
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
			case util.Busy:
				log.Printf("server busy")
				return
			case util.FileExist:
				log.Printf("file %s exist", fileName)
				return
			case util.InitAck:
			default:
				continue
			}
			uploadId = binary.BigEndian.Uint16(respData[util.MessIdIndex : util.MessIdIndex+2])
			try = util.MaxUploadTry * 2
		}
	}
	if try != util.MaxUploadTry*2 {
		log.Printf("Connect fail,exit")
		return
	}
	log.Printf("Connect success")
	// Process file data
	dataSlice := make([][]byte, totalLen)
	for i, _ := range dataSlice {
		begin := i * util.OnceDownloadSize
		end := i*util.OnceDownloadSize + util.OnceDownloadSize
		if end > len(fileData) {
			end = len(fileData)
		}
		dataSlice[i] = append(dataSlice[i], fileData[begin:end]...)
	}
	// begin to upload file
	log.Printf("Begin to upload file %s,wait", fileName)
	// Send the whole file
	for i, bytes := range dataSlice {
		var uploadBytes []byte
		uploadBytes = append(uploadBytes, util.UploadFlag|util.Normal)
		idBytes := make([]byte, 2, 2)
		binary.BigEndian.PutUint16(idBytes, uploadId)
		uploadBytes = append(uploadBytes, idBytes...)
		index := make([]byte, 2, 2)
		binary.BigEndian.PutUint16(index, uint16(i))
		uploadBytes = append(uploadBytes, index...)
		uploadBytes = append(uploadBytes, bytes...)
		go func(bytes []byte) {
			mess := util.IMessage{
				Addr: addr,
				Data: bytes,
			}
			send <- mess
		}(uploadBytes)
	}
	// wait response or upload section again
	ackMap := make(map[uint16]struct{})
	ackLen := totalLen
	// init ackMap
	for i := uint16(0); i < ackLen; i++ {
		ackMap[i] = struct{}{}
	}
	timeout := time.Tick(util.UploadTimeout)
	uploadDisplay := float32(0)
	for {
		again := time.Tick(time.Second * 20)
		select {
		case <-ctx.Done():
			log.Printf("ctx done,return")
			return
		case resp := <-recv:
			respData := resp.Data
			if len(respData) < util.MessHeadLen {
				continue
			}
			respAck := respData[0] & 0x7f
			switch respAck {
			case util.NormalAck:
				index := binary.BigEndian.Uint16(respData[util.MessLenIndex : util.MessLenIndex+2])
				delete(ackMap, index)
				uploadProcess := float32(ackLen-uint16(len(ackMap))) / float32(ackLen)
				if uploadProcess*100 >= uploadDisplay {
					log.Printf("upload process: %.2f%%\n", uploadProcess*100)
					uploadDisplay += 10
				}
				if len(ackMap) == 0 {
					uploadDisplay = 0
					return
				}
			case util.UploadFail:
				log.Printf("upload file fail")
				return
			default:
			}
		case <-again:
			for index, _ := range ackMap {
				var uploadBytes []byte
				uploadBytes = append(uploadBytes, util.UploadFlag|util.Normal)
				idBytes := make([]byte, 2, 2)
				binary.BigEndian.PutUint16(idBytes, uploadId)
				uploadBytes = append(uploadBytes, idBytes...)
				indexLen := make([]byte, 2, 2)
				binary.BigEndian.PutUint16(indexLen, index)
				uploadBytes = append(uploadBytes, indexLen...)
				uploadBytes = append(uploadBytes, dataSlice[index]...)
				go func(bytes []byte) {
					mess := util.IMessage{
						Addr: addr,
						Data: bytes,
					}
					send <- mess
				}(uploadBytes)
			}
		case <-timeout:
			log.Printf("upload fail with timeout")
			return
		}
	}
	//log.Printf("finshed upload")
}
