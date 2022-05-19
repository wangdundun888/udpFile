package util

import (
	"net"
	"time"
)

type Message interface{}

type IMessage struct {
	Addr *net.UDPAddr
	Data []byte
}

type UploadFile struct {
	Filename   string
	Addr       *net.UDPAddr
	TotalLen   uint16
	CurrLen    uint16
	Data       [][]byte
	UpdateTime time.Time
}

type DownloadFile struct {
	FileName     string
	Addr         *net.UDPAddr
	Data         [][]byte
	DownloadTime time.Time
}

const (
	// MaxLen : maximum bytes in a transfer
	MaxLen = 1024
	// MessHeadLen : message head length
	MessHeadLen = 5
	// MessLenIndex : index of message length
	MessLenIndex = 3
	MessIdIndex  = 1

	CleanTime    = time.Minute * 2
	NoUpdateTime = time.Minute * 2

	DownloadCleanTime    = CleanTime
	DownloadNoUpdateTime = NoUpdateTime

	MaxStorageTry  = 100
	MaxUploadTry   = 10
	MaxDownloadTry = 10

	OnceDownloadSize = 1024

	UploadFlag   = 0x0
	DownloadFlag = 0x80

	UploadTimeout   = time.Minute * 10
	DownloadTimeout = UploadTimeout
	ExitTime        = time.Second * 5
	ReadTimeout     = time.Second * 2

	UploadChanCnt   = 10
	DownloadChanCnt = 10
	RecvChanCnt     = 10
	SendChanCnt     = 10
)

const (
	Init = iota
	InitAck
	Normal
	NormalAck
	UploadFail
	Busy
	FileExist
	FileNoExist
	DownloadSomeone
)
