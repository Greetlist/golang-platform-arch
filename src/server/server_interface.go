package server

type Server interface {
    Init()
    Run()
    Close()
}
