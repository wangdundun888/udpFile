package main

import "flag"

type Cmd struct {
	Port        string
	StoragePath string
}

func NewCmd() *Cmd {
	cmd := &Cmd{}
	flag.StringVar(&cmd.Port, "port", "9091", "-port 9090")
	flag.StringVar(&cmd.StoragePath, "sp", ".", "-sp D:\\")
	flag.Parse()
	return cmd
}
