package main

import (
    "fmt"
    "net"
    "strconv"
    "encoding/binary"
    "runtime"
    "time"
    "flag"
    "asocks"
)

func handleConnection(conn *net.TCPConn) {
    err := getRequest(conn)

    if err != nil {
        fmt.Println("err:", err)
        conn.Close()
    }
}

func getRequest(conn *net.TCPConn) (err error){
    var n int
    buf := make([]byte, 256)

    if n, err = conn.Read(buf); err != nil {
        return
    }

    if n < 3 {
        err = fmt.Errorf("get request read %d bytes.", n)
        return
    }

    encodeData(buf)

    var dstAddr []byte;
    var host string;
    var dstAddrLen int;

    switch buf[0] {
        case 1:
            // ipv4
            dstAddr = buf[1:5]
            host = net.IP(dstAddr).String()
            dstAddrLen = 4 
        case 3:
            // domain
            domainLen := buf[1]
            dstAddr = buf[2 : 2 + domainLen]
            host = string(dstAddr)
            dstAddrLen = int(domainLen) + 1
        case 4:
            // ipv6
            dstAddr = buf[1:17]
            host = net.IP(dstAddr).String()
            dstAddrLen = 16
        default:
            // unnormal, close conn
            err = fmt.Errorf("error ATYP:%d\n", buf[0])
            return
    }
    port := binary.BigEndian.Uint16(buf[1 + dstAddrLen : 1 + dstAddrLen + 2])
    host = net.JoinHostPort(host, strconv.Itoa(int(port)))

    fmt.Println("dst:", host)

    var remote *net.TCPConn
    remoteAddr, _ := net.ResolveTCPAddr("tcp", host)
    if remote, err = net.DialTCP("tcp", nil, remoteAddr); err != nil {
        return
    }
    
    // 如果有额外的数据，转发给remote。正常情况下是没有额外数据的，但如果客户端通过端口转发连接服务端，就会有
    if n > 1 + dstAddrLen + 2 {
        if _, err = remote.Write(buf[1 + dstAddrLen + 2 : n]); err != nil {
            return  
        }
    }
   
    finish := make(chan bool, 2) 
    go pipeThenClose(conn, remote, finish)
    pipeThenClose(remote, conn, finish)
    <- finish
    <- finish
    conn.Close()
    remote.Close()

    return nil
}

func pipeThenClose(src, dst *net.TCPConn, finish chan bool) {
    defer func(){
        src.CloseRead()
        dst.CloseWrite()
        finish <- true
    }()

    buf := asocks.GetBuffer()
    defer asocks.GiveBuffer(buf)

    for {
        src.SetReadDeadline(time.Now().Add(60 * time.Second))
        n, err := src.Read(buf);
        if n > 0 {
            data := buf[0:n]
            encodeData(data)
            if _, err := dst.Write(data); err != nil {
                break
            }
        }
        if err != nil {
            break
        }
    }
}

func encodeData(data []byte) {
    for i, _ := range data {
        data[i] ^= 128;
    }
}

func main() {
    var localAddr string
    flag.StringVar(&localAddr, "l", "0.0.0.0:1080", "监听端口")
    flag.Parse()

    numCPU := runtime.NumCPU()
    runtime.GOMAXPROCS(numCPU)
    
    bindAddr, err := net.ResolveTCPAddr("tcp", localAddr)
    if err != nil {
        fmt.Printf("resolve %s failed. err:%s\n", localAddr, err)
        return
    }

    ln, err := net.ListenTCP("tcp", bindAddr) 
    if err != nil {
        fmt.Println("listen error:", err)
        return
    }
    defer ln.Close()

    fmt.Println("listening ", ln.Addr())

    for {
        conn, err := ln.AcceptTCP()
        if err != nil {
            fmt.Println("accept error:", err)
            continue
        }

        go handleConnection(conn)
    }
}
