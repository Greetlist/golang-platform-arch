package main

import (
    "server/tcpserver"
    "time"
    "glog"
)

func main() {
    glog.Infoln("Init")
    tcpServer := tcpserver.TcpServer{}
    tcpServer.Init()
    glog.Infoln("Init")
    go tcpServer.Run()
    glog.Infoln("Run")
    time.Sleep(5 * time.Second)
    tcpServer.Close()
    time.Sleep(4 * time.Second)
    return
}
