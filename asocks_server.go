package main

import (
    "fmt"
    "net"
    "strconv"
    "encoding/binary"
    "runtime"
    "time"
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
    port := binary.BigEndian.Uint16(buf[1 + dstAddrLen: 1 + dstAddrLen + 2])
    host = net.JoinHostPort(host, strconv.Itoa(int(port)))

    fmt.Println("dst:", host)

    var remote *net.TCPConn
    remoteAddr, _ := net.ResolveTCPAddr("tcp", host)
    if remote, err = net.DialTCP("tcp", nil, remoteAddr); err != nil {
        return
    }
    
    go pipeThenClose(conn, remote)
    pipeThenClose(remote, conn)
    return nil
}

func pipeThenClose(src, dst *net.TCPConn) {
    defer func(){
        src.CloseRead()
        dst.CloseWrite()
    }()

    for {
        src.SetReadDeadline(time.Now().Add(600 * time.Second))
        buf := make([]byte, 5120)
        n, err := src.Read(buf);
        if n > 0 {
            data := buf[0:n]
            encodeData(data)
            if _, err := dst.Write(data); err != nil {
                fmt.Println("pipe write error:", err)
                break
            }
        }
        if err != nil {
            if err != io.EOF {
                fmt.Println("pipe read error:", err)
            }
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
    numCPU := runtime.NumCPU()
    runtime.GOMAXPROCS(numCPU)
    
    bindAddr,_ := net.ResolveTCPAddr("tcp", ":17570")
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
