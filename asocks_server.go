package main

import (
    "fmt"
    "net"
    "strconv"
    "encoding/binary"
    "runtime"
)

func handleConnection(conn net.Conn) {

    fmt.Println("new conn:", conn.RemoteAddr())

    defer func(){
        fmt.Println("close connection.")
        conn.Close()
    }()

    getRequest(conn)
}

func getRequest(conn net.Conn) {
    var n int
    var err error
    buf := make([]byte, 256)
    n, err = conn.Read(buf)
    if err != nil {
        return
    }

    if n < 3 {
        fmt.Printf("get request read %d bytes", n)
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
            fmt.Println("error ATYP:", buf[0])
            return
    }
    port := binary.BigEndian.Uint16(buf[1 + dstAddrLen: 1 + dstAddrLen + 2])
    host = net.JoinHostPort(host, strconv.Itoa(int(port)))

    fmt.Println("dst:", host)

    var remote net.Conn
    remote, err = net.Dial("tcp", host)
    if err != nil {
        fmt.Println("connect to dst failed. err:", err)
        return
    }
    
    go pipeThenClose(remote, conn)
    pipeThenClose(conn, remote)
}

func pipeThenClose(src, dst net.Conn) {
    defer dst.Close()
    for {
        buf := make([]byte, 5120)
        n, err := src.Read(buf)
        if n > 0 { 
                data := buf[0:n]
                encodeData(data)
                if _, err = dst.Write(data); err != nil {
                    break; 
                }
        }
        if err != nil {
            break;
        }
    }
}

func encodeData(data []byte) {
    for i, _ := range data {
        data[i] ^= 128;
    }
}

func main() {
    numCPU := runtime.NumCPU()
    runtime.GOMAXPROCS(numCPU)
    ln, err := net.Listen("tcp", ":17570") 
    if err != nil {
        fmt.Println("listen error:", err)
        return
    }

    fmt.Println("listening ", ln.Addr())

    for {
        conn, err := ln.Accept()
        if err != nil {
            fmt.Println("accept error:", err)
            continue
        }
        go handleConnection(conn)
    }
}
