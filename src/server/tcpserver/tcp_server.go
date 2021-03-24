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
                glog.Infoln("Receive Context Close")
                //close all connection first and remove it from epoll
                for fd, _ := range fdMap {
                    unix.Close(fd)
                    glog.Infoln("Remove fd from epoll : ", fd)
                    tcpServer.deleteSocketFromEpoll(epfd, fd)
                }
                return
            default:
                break
        }
        //add timeout in order to get cancel signal
        num, err := unix.EpollWait(epfd, events, 1 * 1000)
        if err != nil {
            if num < 0 && err == unix.EINTR {
                continue
            } else {
                glog.Errorln("Epoll Wait error : %s", err)
            }
        }
        for i := 0; i < num; i++ {
            if int(events[i].Fd) == listenFD {
                client, _, _ := unix.Accept(int(events[i].Fd))
                fdMap[client] = 1
                unix.SetNonblock(client, true)
                tcpServer.addSocketToEpoll(epfd, client, "rw")
            } else {
                if events[i].Events & unix.EPOLLIN != 0 {
                    glog.Infoln("Data in")
                } else if events[i].Events & unix.EPOLLOUT != 0 {
                    glog.Infoln("Data out")
                }
            }
        }
    }
}

func (tcpServer *TcpServer) Close() {
    tcpServer.cancelFunc()
    tcpServer.wg.Wait()
}
