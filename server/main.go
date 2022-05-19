package main

import (
	"context"
	"log"
	"net"
	"os"
	"server/download"
	"server/recv"
	"server/send"
	"server/upload"
	"server/util"
	"time"
)

func main() {
	cmd := NewCmd()
	udpAddr, err := net.ResolveUDPAddr("udp", ":"+cmd.Port)
	if err != nil {
		log.Printf("resolve port %s error %s\n", cmd.Port, err.Error())
	}
	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		log.Printf("listen %s error %s", udpAddr.String(), err.Error())
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

	// if storage path no exist,exit
	if !pathExists(cmd.StoragePath) {
		log.Printf("path %s no exist\n", cmd.StoragePath)
		cancel()
		time.Sleep(util.ExitTime)
		return
	}
	// turn on upload module
	go upload.Upload(cmd.StoragePath, uploadChan, sendChan, ctx)
	// turn on download module
	go download.Download(cmd.StoragePath, downloadChan, sendChan, ctx)
	defer func() {
		cancel()
		time.Sleep(util.ExitTime)
		log.Printf("exit\n")
	}()
	<-ctx.Done()
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	return false
}
