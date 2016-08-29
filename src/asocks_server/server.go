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
    "io"
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
    buf := make([]byte, 257)

    if n, err = io.ReadAtLeast(conn, buf, 8); err != nil {
        return
    }

    if n, err = io.ReadAtLeast(conn, buf, 2); err != nil {
        return
    }

    encodeData(buf)

    addressType := buf[0]
    reqLen := 0;

    switch addressType {
        case 1:
            // ipv4
            reqLen = 1 + 4 + 2
        case 3:
            // domain
            reqLen = 1 + 1 + int(buf[1]) + 2
        case 4:
            // ipv6
            reqLen = 1 + 16 + 2
        default:
            // unnormal, close conn
            err = fmt.Errorf("error ATYP:%d\n", buf[0])
            return
    }
    
    if n < reqLen {
        if _, err = io.ReadFull(conn, buf[n : reqLen]); err != nil {
            return
        }
        encodeData(buf[n:reqLen]) 
    }

    var host string;

    switch addressType {
        case 1:
            // ipv4
            host = net.IP(buf[1:5]).String()
        case 3:
            // domain
            dstAddr := buf[2 : 2 + int(buf[1])]
            host = string(dstAddr)
        case 4:
            // ipv6
            host = net.IP(buf[1:17]).String()
    }

    port := binary.BigEndian.Uint16(buf[reqLen - 2 : reqLen])
    host = net.JoinHostPort(host, strconv.Itoa(int(port)))

    fmt.Println("dst:", host)

    var remote *net.TCPConn
    remoteAddr, _ := net.ResolveTCPAddr("tcp", host)
    if remote, err = net.DialTCP("tcp", nil, remoteAddr); err != nil {
        return
    }
    
    // 如果有额外的数据，转发给remote。正常情况下是没有额外数据的，但如果客户端通过端口转发连接服务端，就会有
    if n > reqLen {
        if _, err = remote.Write(buf[reqLen : n]); err != nil {
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
    flag.StringVar(&localAddr, "l", "0.0.0.0:8388", "监听端口")
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
