package main

import (
    "server/tcpserver"
    "time"
)

func main() {
    tcpServer := tcpserver.TcpServer{}
    tcpServer.Init()
    go tcpServer.Run()
    time.Sleep(5 * time.Second)
    tcpServer.Close()
    time.Sleep(4 * time.Second)
    return
}
