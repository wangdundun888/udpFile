package main

import "flag"

type Cmd struct {
	Ip          string
	StoragePath string
	FileName    string
	Upload      bool
}

func NewCmd() *Cmd {
	cmd := &Cmd{}
	flag.StringVar(&cmd.Ip, "ip", "127.0.0.1:9091", "-ip 127.0.0.1:9090")
	flag.StringVar(&cmd.StoragePath, "sp", "E:", "-sp D:\\")
	flag.StringVar(&cmd.FileName, "fn", "t1.txt", "-fn test.txt")
	flag.BoolVar(&cmd.Upload, "upload", true, "-upload=true")
	flag.Parse()
	return cmd
}
