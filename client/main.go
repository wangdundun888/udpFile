package main

import (
	"client/download"
	"client/recv"
	"client/send"
	"client/upload"
	"client/util"
	"context"
	"log"
	"net"
	"os"
	"strings"
	"time"
)

func main() {
	cmd := NewCmd()
	udpAddr, err := net.ResolveUDPAddr("udp", cmd.Ip)
	if err != nil {
		log.Printf("err:%s", err.Error())
		return
	}
	udpConn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		log.Printf("err:%s", err.Error())
		return
	}
	defer udpConn.Close()
	uploadChan := make(chan util.IMessage, util.UploadChanCnt)
	downloadChan := make(chan util.IMessage, util.DownloadChanCnt)
	sendChan := make(chan util.IMessage, util.SendChanCnt)
	bgCtx := context.Background()
	ctx, cancel := context.WithCancel(bgCtx)
	// turn on receive module
	go recv.Recv(udpConn, uploadChan, downloadChan, ctx)
	// turn on send module
	go send.Send(udpConn, sendChan, ctx)
	if !pathExists(cmd.StoragePath) {
		log.Printf("path %s no exist", cmd.StoragePath)
		cancel()
		time.Sleep(util.ExitTime)
		return
	}
	if (cmd.Upload && !fileExists(cmd.StoragePath, cmd.FileName)) || (!cmd.Upload && fileExists(cmd.StoragePath, cmd.FileName)) {
		log.Printf("upload : but file %s no exist or download but file %s exist", cmd.FileName, cmd.FileName)
		cancel()
		time.Sleep(util.ExitTime)
		return
	}

	if cmd.Upload {
		// upload
		upload.Upload(cmd.StoragePath, cmd.FileName, uploadChan, sendChan, udpAddr, ctx)
	} else {
		// download
		download.Download(cmd.StoragePath, cmd.FileName, downloadChan, sendChan, udpAddr, ctx)
	}
	cancel()
	time.Sleep(util.ExitTime)
	log.Printf("exit...")
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
