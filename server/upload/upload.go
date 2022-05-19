package upload

import (
	"context"
	"encoding/binary"
	"log"
	"math"
	"math/rand"
	"os"
	"server/util"
	"strings"
	"sync"
	"time"
)

func Upload(storagePath string, recv, send chan util.IMessage, ctx context.Context) {
	dataMap := make(map[uint16]util.UploadFile, 256)
	var mapLock sync.RWMutex
	// turn on clean data goroutine
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
				if exist {
					//file exists
					data[0] = util.FileExist
					mess.Data = data
					send <- mess
					continue
				}
				id, ok := generateId(dataMap, mapLock)
				if !ok {
					// fail,return busy code
					data[0] = util.Busy
					mess.Data = data
					send <- mess
					continue
				}
				var totalLen uint16
				totalLen = binary.BigEndian.Uint16(data[util.MessLenIndex : util.MessLenIndex+2])
				uploadFile := util.UploadFile{
					Filename:   fileName,
					Addr:       mess.Addr,
					TotalLen:   totalLen,
					CurrLen:    0,
					Data:       make([][]byte, totalLen),
					UpdateTime: time.Now(),
				}
				mapLock.Lock()
				dataMap[id] = uploadFile
				mapLock.Unlock()
				binary.BigEndian.PutUint16(data[util.MessIdIndex:util.MessIdIndex+2], id)
				data[0] = util.InitAck | util.UploadFlag
				mess.Data = data
				// init ack
				send <- mess
			case util.Normal:
				mapLock.Lock()
				var id uint16
				id = binary.BigEndian.Uint16(data[util.MessIdIndex : util.MessIdIndex+2])
				uf, exist := dataMap[id]
				if exist {
					index := binary.BigEndian.Uint16(data[util.MessLenIndex : util.MessLenIndex+2])
					// Determine whether the corresponding fragment has been uploaded
					if len(uf.Data[index%uf.TotalLen]) == 0 {
						uf.Data[index%uf.TotalLen] = append(uf.Data[index%uf.TotalLen], data[util.MessHeadLen:]...)
						uf.CurrLen++
						uf.UpdateTime = time.Now()
						dataMap[id] = uf
						ack := data[:util.MessHeadLen]
						ack[0] = util.NormalAck
						if uf.TotalLen == uf.CurrLen {
							// recv all data,storage it
							go storage(storagePath, uf)
							delete(dataMap, id)
						}
					}
					// ack
					ack := data[:util.MessHeadLen]
					ack[0] = util.UploadFlag | util.NormalAck
					mess.Data = ack
					send <- mess
				}
				mapLock.Unlock()
			default:
				//ignore other funcCode
			}
		}
	}

}

func generateId(dataMap map[uint16]util.UploadFile, lock sync.RWMutex) (uint16, bool) {
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

func storage(path string, uploadFile util.UploadFile) {
	path = strings.TrimRight(path, string(os.PathSeparator))
	path = path + string(os.PathSeparator) + uploadFile.Filename
	try := 0
	var data []byte
	for _, bytes := range uploadFile.Data {
		data = append(data, bytes...)
	}
	for try < util.MaxStorageTry {
		try++
		f, err := os.Create(path)
		if err != nil {
			log.Printf("Failed to store %s %dth time", uploadFile.Filename, try)
			continue
		}
		n, err := f.Write(data)
		f.Close()
		if err != nil || n != len(data) {
			log.Printf("Failed to store %s %dth time", uploadFile.Filename, try)
			continue
		}
		break
	}
}

func cleanData(dataMap map[uint16]util.UploadFile, send chan util.IMessage, lock sync.RWMutex, ctx context.Context) {
	for {
		time.Sleep(util.CleanTime)
		select {
		case <-ctx.Done():
			return
		default:
		}
		lock.Lock()
		//  remove ids
		var ids []uint16
		// uncompleted
		var uncompleted []util.UploadFile
		now := time.Now()
		for id, file := range dataMap {
			if file.UpdateTime.Add(util.NoUpdateTime).Before(now) {
				ids = append(ids, id)
				if file.CurrLen != file.TotalLen {
					uncompleted = append(uncompleted, file)
				}
			}

		}
		for _, id := range ids {
			// remove all the expired data
			delete(dataMap, id)
		}
		for _, file := range uncompleted {
			// send fail mess
			data := make([]byte, util.MessHeadLen)
			data[0] = util.UploadFlag | util.UploadFail
			mess := util.IMessage{
				Addr: file.Addr,
				Data: data,
			}
			send <- mess
		}
		lock.Unlock()
	}
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
