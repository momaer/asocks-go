package main

import (
    "fmt"
    "net"
    "strconv"
    "runtime"
)

func handleConnection(conn net.Conn) (err error) {
    closed := false

    defer func(){
        if !closed {
            //fmt.Println("close connection.")
            conn.Close()
        }
    }()

    buf := make([]byte, 256)
    var n int;
    if n, err = conn.Read(buf); err != nil {
        fmt.Println("从client读握手就读失败了。err:", err)
        return 
    }

    if n >= 3 {
        if buf[0] != 5 {
            fmt.Printf("握手时读取到的socks版本是%d。\n", buf[0])
            return
        }
    } else {
        fmt.Printf("握手时，读到了%d个字节。\n", n)
        return
    }
    
    if _, err = conn.Write([]byte{5, 0}); err != nil {
        fmt.Println("握手时，写失败。err:", err)
        return
    }
 
    err = getRequest(conn)
    if err != nil {
        closed = false
        fmt.Println("get request err:", err)
    }

    closed = true
    return
}

func getRequest(conn net.Conn) (err error){
    buf := make([]byte, 256)
    var n int 
    if n, err = conn.Read(buf); err != nil {
        fmt.Println("getRequest read error:", err)
        return
    }
    if buf[0] != 5 && buf[1] != 1 && buf[2] != 0 {
        fmt.Println("getRequest not socks5 protocol.")
        return
    }

    var rawRequest []byte

    switch buf[3] {
        case 1:
            // ipv4
            rawRequest = make([]byte, 1+4+2)
        case 3:
            // domain
            domainLen := buf[4]
            rawRequest = make([]byte, 1 + 1 + domainLen + 2)
        case 4:
            // ipv6
            rawRequest = make([]byte, 1 + 16 + 2)
        default:
            // unnormal, close conn
            return
    }
    copy(rawRequest, buf[3:n])
    host := net.JoinHostPort(serverAddr, strconv.Itoa(serverPort))
   
    var remote net.Conn  
    if remote, err = net.Dial("tcp", host); err != nil {
        return
    }
   
    encodeData(rawRequest) 
    if n, err = remote.Write(rawRequest); err != nil {
        fmt.Println("getRequest 发送request到远程服务器失败。err:", err)
        remote.Close()
        return
    }

    // send reply
    var replyBuf []byte = []byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}
    if _, err = conn.Write(replyBuf); err != nil {
        fmt.Println("getRequest 发送reply给客户端失败。err:", err) 
        return 
    }
    
    go pipeThenClose(remote, conn)
    pipeThenClose(conn, remote)
    return nil
}

func pipeThenClose(src, dst net.Conn) {
    defer dst.Close()
    for {
        buf := make([]byte, 5120) 
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

const (
    serverAddr = "123.56.160.111"
    serverPort = 17570
)

func main() {
    numCPU := runtime.NumCPU()
    runtime.GOMAXPROCS(numCPU)

    ln, err := net.Listen("tcp", ":1080")
    if  err != nil {
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
        fmt.Println("new connection:", conn.RemoteAddr())
        go handleConnection(conn)
    }
}
