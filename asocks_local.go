package main

import (
    "fmt"
    "net"
    "strconv"
)

func handleConnection(conn net.Conn) (err error) {
    closed := false
    defer func(){
        if closed {
            fmt.Println("close connection.")
            conn.Close()
        }
    }()
    buf := make([]byte, 256)
    var n int;
    if n, err = conn.Read(buf); err != nil {
    
        closed = true
        return 
    }

    if n >= 3 {
        if buf[0] != 5 {
            fmt.Println("ver:", buf[0])
            closed = true
            return
        }
    } else {
        fmt.Printf("read %d bytes.\n", n)
        closed = true
        return
    }
    
    if _, err = conn.Write([]byte{5, 0}); err != nil {
        closed = true
        return
    }
    
    fmt.Println("send handshark reply.")
 
    err = getRequest(conn)
    if err != nil {
        fmt.Println("get request err:", err)
    }

    fmt.Println("get request done")
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
        fmt.Println("not socks5 protocol.")
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
    fmt.Println("connect to server successed.")
   
    encodeData(rawRequest) 
    if n, err = remote.Write(rawRequest); err != nil {
        return
    }

    // send reply
    var replyBuf []byte = []byte{5, 0, 0, 1, 0, 0, 0, 0, 0, 0}
    conn.Write(replyBuf)
    fmt.Println("send request reply.")
    
    go pipeThenClose(conn, remote)
    pipeThenClose(remote, conn)
    return
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
    serverAddr = "actself.me"
    serverPort = 17570
)

func main() {

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
        fmt.Println("connection:", conn.RemoteAddr())
        go handleConnection(conn)
    }
}
