package main

import (
    "fmt"
    "net"
    "io"
    "flag"
)

func handleConn(conn *net.TCPConn, raddr *net.TCPAddr) {
    remote, err := net.DialTCP("tcp", nil, raddr)
    if err != nil {
        fmt.Println("dial remote fail. err:", err)
        conn.Close()
        return
    }
    
    finish := make(chan bool, 2)
    go pipe(conn, remote, finish)
    pipe(remote, conn, finish)
    <- finish
    <- finish
    conn.Close()
    remote.Close()
}

func pipe(src, dst *net.TCPConn, finish chan bool) {
    defer func() {
        //src.CloseRead()
        //dst.CloseWrite()
        finish <- true
    }()

    io.Copy(dst, src)
}

func main() {
    var laddr string
    var raddr string

    flag.StringVar(&laddr, "l", "", "local address") 
    flag.StringVar(&raddr, "r", "", "remote address") 

    flag.Parse()

    if laddr == "" || raddr == "" {
        fmt.Printf("-l localAddress -r remoteAddress\n")
        return
    }

    a, err := net.ResolveTCPAddr("tcp", laddr) 
    if err != nil {
        fmt.Println(err)
        return 
    }
    b, err := net.ResolveTCPAddr("tcp", raddr)
    if err != nil {
        fmt.Println(err)
    } 
    
    ln, err := net.ListenTCP("tcp", a) 
    if err != nil {
        fmt.Println("listen error:", err)
        return;
    }
    defer ln.Close()
    fmt.Println("listening on ", ln.Addr())
    fmt.Println("remote address: ", raddr)

    for {
        conn, err := ln.AcceptTCP()
        if err != nil {
            fmt.Println("accept err:", err)
            continue
        }

        go handleConn(conn, b)
    }
}
