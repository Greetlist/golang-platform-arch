package main

import (
    "server/tcpserver"
)

func main() {
    tcpServer := tcpserver.TcpServer{}
    tcpServer.Init()
    tcpServer.Run()
}
