package tcpserver

import (
    "golang.org/x/sys/unix"
    "glog"
)

func (tcpServer *TcpServer) createEpoll() int {
    ep, err := unix.EpollCreate1(0)
    if err != nil {
        glog.Errorln("epoll create error : %s", err)
        return -1
    }
    return ep
}

func (tcpServer *TcpServer) addSocketToEpoll(epfd, fd int, eventType string) error {
    event := &unix.EpollEvent{
        Fd : int32(fd),
    }
    var commonEvent uint32 = unix.EPOLLET
    switch eventType {
        case "r":
            event.Events = commonEvent | unix.EPOLLIN
        case "w":
            event.Events = commonEvent | unix.EPOLLOUT
        case "rw":
            event.Events = commonEvent | unix.EPOLLIN | unix.EPOLLOUT
    }
    return unix.EpollCtl(epfd, unix.EPOLL_CTL_ADD, fd, event)
}

func (tcpServer *TcpServer) deleteSocketFromEpoll(epfd, fd int) error {
    return unix.EpollCtl(epfd, unix.EPOLL_CTL_DEL, fd, nil)
}

func (tcpServer *TcpServer) mainEpollWait(epfd int, events []unix.EpollEvent, sec int) int {
    num, err := unix.EpollWait(epfd, events, sec * 1000)
    if err != nil {
        if num < 0 && err == unix.EINTR {
            return 0
        } else {
            glog.Errorln("Epoll Wait error : %s", err)
	    return -1
        }
    }
    return num
}
