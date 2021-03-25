package tcpserver

import (
    "strconv"
    "flag"
    "glog"
    "config"
    "net"
    "context"
    "sync"
    "golang.org/x/sys/unix"
    "os/signal"
    "os"
    "syscall"
)

const (
    PAGE_SIZE = 4096
    RECVBUF_SIZE = 65535
    SENDBUF_SIZE = 65535
    HEARTBEAT_INTERVAL = 600 //second
)

func init() {
    flag.Parse()
}

type TcpServer struct {
    ListenAddr string
    ListenPort int
    quit bool
    cl config.ConfigLoader
    rootContext context.Context
    cancelFunc context.CancelFunc
    goroutineNum int
    wg sync.WaitGroup
}

func (tcpServer *TcpServer) Init() error {
    tcpServer.cl = config.ConfigLoader{ConfigType : "tcp"}
    if err := tcpServer.cl.Init(); err != nil {
        glog.Errorln("Init config is : %s.\n", err)
        return err
    }
    tcpServer.rootContext, tcpServer.cancelFunc = context.WithCancel(context.Background())
    tcpServer.goroutineNum = tcpServer.cl.GetInt("tcp_server", "listen_routine_num")
    tcpServer.wg = sync.WaitGroup{}
    return nil
}

func (tcpServer *TcpServer) Run() {
    for i := 0; i < tcpServer.goroutineNum; i++ {
        tcpServer.wg.Add(1)
        go tcpServer.createWorker()
    }
    ch := make(chan os.Signal, 1)
    signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
    for {
        select {
            case <-ch:
                tcpServer.Close()
                return
        }
    }
}

func (tcpServer *TcpServer) Close() {
    tcpServer.cancelFunc()
    tcpServer.wg.Wait()
}

func (tcpServer *TcpServer) createWorker() {
    var listenFD int = tcpServer.newTCPListenSocket()
    var epfd int = tcpServer.createEpoll()
    if listenFD == -1 || epfd == -1 {
        glog.Info("123")
        return
    }
    if err := tcpServer.addSocketToEpoll(epfd, listenFD, "rw"); err != nil {
        glog.Errorln("Epoll Ctl Error : %s", err)
        return
    }
    tcpServer.startServe(epfd, listenFD)
    tcpServer.wg.Done()
}

func (tcpServer *TcpServer) newTCPListenSocket() int {
    network := tcpServer.cl.GetString("tcp_server", "server_network")
    address := tcpServer.cl.GetString("tcp_server", "listen_address")
    port := tcpServer.cl.GetInt("tcp_server", "listen_port")
    tcpAddr, _ := net.ResolveTCPAddr(network, address + ":" + strconv.Itoa(port))
    listenFD, err := unix.Socket(unix.AF_INET, unix.SOCK_STREAM, unix.IPPROTO_TCP)
    if err != nil {
        glog.Errorln("Create Listen fd error : %s", err)
        return -1
    }

    unix.SetsockoptInt(listenFD, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)
    unix.SetNonblock(listenFD, true)
    var sa unix.Sockaddr
    inet4Addr := &unix.SockaddrInet4{Port : port}
    copy(inet4Addr.Addr[:], tcpAddr.IP.To4())
    glog.Infoln(inet4Addr.Addr, inet4Addr.Port)
    sa = inet4Addr
    if err != unix.Bind(listenFD, sa) {
        glog.Errorln("Bind fd error : %s", err)
        return -1
    }
    return listenFD
}

func (tcpServer *TcpServer) startServe(epfd, listenFD int) {
    //record this routine connection
    fdMap := make(map[int]int)
    backlog := tcpServer.cl.GetInt("tcp_server", "listen_backlog")
    if err := unix.Listen(listenFD, backlog); err != nil {
        glog.Errorln("Listen err : ", err)
    }
    events := make([]unix.EpollEvent, 128)
    for {
        select {
            case <-tcpServer.rootContext.Done():
		tcpServer.closeAllClientConnection(epfd, fdMap)
                return
            default:
                break
        }
        //add timeout in order to get cancel signal
	num := tcpServer.mainEpollWait(epfd, events, 1)
        for i := 0; i < num; i++ {
            if int(events[i].Fd) == listenFD {
		tcpServer.acceptClient(int(events[i].Fd), epfd, fdMap)
            } else {
                if events[i].Events & unix.EPOLLIN != 0 {
		    buf := make([]byte, PAGE_SIZE)
                    glog.Infoln("Data in")
		    if n, err := unix.Read(int(events[i].Fd), buf); err != nil {
                        glog.Errorln(err)
		    } else if n == 0 {
                        tcpServer.doCloseFd(epfd, int(events[i].Fd))
			delete(fdMap, int(events[i].Fd))
                    }
                    glog.Infoln(string(buf))
                } else if events[i].Events & unix.EPOLLOUT != 0 {
                    glog.Infoln("Data out")
                }
            }
        }
    }
}

func (tcpServer *TcpServer) acceptClient(listenFD, epfd int, fdMap map[int]int) {
    var client int
    var err error
    if client, _, err = unix.Accept(listenFD); err != nil {
        return
    }
    unix.SetNonblock(client, true)
    unix.SetsockoptInt(client, unix.SOL_SOCKET, unix.SO_KEEPALIVE, HEARTBEAT_INTERVAL)
    tcpServer.addSocketToEpoll(epfd, client, "rw")
    fdMap[client] = 1
}

func (tcpServer *TcpServer) closeAllClientConnection(epfd int, fdMap map[int]int) {
    glog.Infoln("Receive Context Close")
    //close all connection first and remove it from epoll
    for fd, _ := range fdMap {
        tcpServer.doCloseFd(epfd, fd)
    }
}

func (tcpServer *TcpServer) doCloseFd(epfd, fd int) {
    unix.Close(fd)
    glog.Infoln("Remove fd from epoll : ", fd)
    tcpServer.deleteSocketFromEpoll(epfd, fd)
}
