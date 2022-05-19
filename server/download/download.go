package download

import (
	"context"
	"encoding/binary"
	"log"
	"math"
	"math/rand"
	"net"
	"os"
	"server/util"
	"strings"
	"sync"
	"time"
)

func Download(storagePath string, recv, send chan util.IMessage, ctx context.Context) {
	dataMap := make(map[uint16]util.DownloadFile, 256)
	var mapLock sync.RWMutex
	go cleanData(dataMap, send, mapLock, ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case mess := <-recv:
			data := mess.Data
			funcCode := data[0] & 0x7f
			switch funcCode {
			case util.Init:
				fileName := string(data[util.MessHeadLen:])
				exist := fileExists(storagePath, fileName)
				if !exist {
					// file no exists
					data[0] = util.FileNoExist | util.DownloadFlag
					mess.Data = data
					send <- mess
					continue
				}
				storagePath = strings.TrimRight(storagePath, string(os.PathSeparator))
				absPath := storagePath + string(os.PathSeparator) + fileName
				file, err := os.Open(absPath)
				if err != nil {
					log.Printf("open file %s error：%s", absPath, err.Error())
					continue
				}
				fileStat, err := file.Stat()
				if err != nil {
					continue
				}
				fileData := make([]byte, fileStat.Size())
				n, err := file.Read(fileData)
				if err != nil {
					log.Println(err)
					continue
				}
				err = file.Close()
				if err != nil {
					log.Println(err)
				}
				id, ok := generateId(dataMap, mapLock)
				if !ok {
					continue
				}
				size := len(fileData[:n])
				totalLen := uint16((size-1)/util.OnceDownloadSize) + 1
				binary.BigEndian.PutUint16(data[util.MessLenIndex:util.MessLenIndex+2], totalLen)
				binary.BigEndian.PutUint16(data[util.MessIdIndex:util.MessIdIndex+2], id)
				data[0] = util.InitAck | util.DownloadFlag
				mess.Data = data
				// init ack
				send <- mess
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
				downloadFile := util.DownloadFile{
					FileName:     fileName,
					Addr:         mess.Addr,
					Data:         dataSlice,
					DownloadTime: time.Now(),
				}
				mapLock.Lock()
				dataMap[id] = downloadFile
				mapLock.Unlock()
				// send file after initialization
				var head []byte
				head = append(head, util.DownloadFlag|util.Normal)
				idBytes := make([]byte, 2, 2)
				binary.BigEndian.PutUint16(idBytes, id)
				head = append(head, idBytes...)
				go sendFile(mess.Addr, head, dataSlice, send)
			case util.DownloadSomeone:
				mapLock.Lock()
				messId := binary.BigEndian.Uint16(data[util.MessIdIndex : util.MessIdIndex+2])
				fileData, exist := dataMap[messId]
				if exist {
					messIndex := binary.BigEndian.Uint16(data[util.MessLenIndex : util.MessLenIndex+2])
					reqData := fileData.Data[messIndex%uint16(len(fileData.Data))]
					mess.Data = append(mess.Data[:util.MessHeadLen], reqData...)
					send <- mess

					// update download time
					fileData.DownloadTime = time.Now()
					dataMap[messId] = fileData
				}
				mapLock.Unlock()
			}
		}
	}
}

func cleanData(dataMap map[uint16]util.DownloadFile, send chan util.IMessage, lock sync.RWMutex, ctx context.Context) {
	for {
		time.Sleep(util.DownloadCleanTime)
		select {
		case <-ctx.Done():
			return
		default:
		}
		lock.Lock()
		//  remove ids
		var ids []uint16
		// uncompleted
		now := time.Now()
		for id, file := range dataMap {
			if file.DownloadTime.Add(util.DownloadNoUpdateTime).Before(now) {
				ids = append(ids, id)
			}
		}
		for _, id := range ids {
			// remove all the expired data
			delete(dataMap, id)
		}
		lock.Unlock()
	}
}

func sendFile(addr *net.UDPAddr, head []byte, dataSlice [][]byte, send chan util.IMessage) {
	for index, bytes := range dataSlice {
		var downloadBytes []byte
		downloadBytes = append(downloadBytes, head...)
		indexBytes := make([]byte, 2, 2)
		binary.BigEndian.PutUint16(indexBytes, uint16(index))
		downloadBytes = append(downloadBytes, indexBytes...)
		downloadBytes = append(downloadBytes, bytes...)
		go func(bytes []byte) {
			mess := util.IMessage{
				Addr: addr,
				Data: bytes,
			}
			send <- mess
		}(downloadBytes)
	}
}

func generateId(dataMap map[uint16]util.DownloadFile, lock sync.RWMutex) (uint16, bool) {
	if len(dataMap) >= math.MaxUint16 {
		return 0, false
	}
	var id uint16
	id = uint16(rand.Intn(math.MaxUint16))
	lock.RLock()
	defer lock.RUnlock()
	for {
		_, exist := dataMap[id]
		if !exist {
			return id, true
		}
		id++
	}
	return id, true
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return false
}

func fileExists(path, fileName string) bool {
	path = strings.TrimRight(path, string(os.PathSeparator))
	return pathExists(path + string(os.PathSeparator) + fileName)
}
