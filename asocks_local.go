package main

import (
    "fmt"
    "net"
    "runtime"
    "time"
    "flag"
    "os"
)

func handleConnection(conn *net.TCPConn) {
    closed := false

    defer func(){
        if !closed {
            conn.Close()
        }
    }()

    var n int;
    var err error;
    buf := make([]byte, 256)

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
        fmt.Println("get request err:", err.Error())
        return
    }

    closed = true
    return
}

func getRequest(conn *net.TCPConn) (err error){
    var n int
    buf := make([]byte, 256)

    if n, err = conn.Read(buf); err != nil {
        err = fmt.Errorf("getRequest read error:", err)
        return
    }

    if buf[0] != 5 && buf[1] != 1 && buf[2] != 0 {
        err = fmt.Errorf("getRequest not socks5 protocol.")
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
            err = fmt.Errorf("request type不正确：%d", buf[3])
            return
    }
    copy(rawRequest, buf[3:n])
   
    var remote *net.TCPConn
    if remote, err = net.DialTCP("tcp", nil, &server); err != nil {
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
                //fmt.Println("pipe write error:", err)
                break
            }
        }
        if err != nil {
            /*if err != io.EOF {
                fmt.Println("pipe read error:", err)
            }*/
            
            break
        }
    }
}

func encodeData(data []byte) {
    for i, _ := range data {
        data[i] ^= 128;
    }
}

func printUsage() {
    fmt.Printf("Usage:%s -s server_addr:server_port -l local_addr:local_port\n", os.Args[0])
}

var localAddr string
var serverAddr string
var server net.TCPAddr

func main() {
    flag.StringVar(&localAddr, "l", "127.0.0.1:1080", "本地监听IP:端口")
    flag.StringVar(&serverAddr, "s", "", "服务器IP:端口")
    flag.Parse()

    if serverAddr == "" {
        printUsage()
        return
    }

    i, err:= net.ResolveTCPAddr("tcp", serverAddr)
    if err != nil {
        fmt.Println("resolve ", serverAddr, " failed. err:", err)
        return
    }
    server = *i;

    numCPU := runtime.NumCPU()
    runtime.GOMAXPROCS(numCPU)
    
    bindAddr, _ := net.ResolveTCPAddr("tcp", localAddr)
    ln, err := net.ListenTCP("tcp", bindAddr)
    if  err != nil {
        fmt.Println("listen error:", err)
        return
    }
    defer ln.Close()

    fmt.Println("listening ", ln.Addr())
    fmt.Println("server:", server.String())

    for {
        conn, err := ln.AcceptTCP()
        if err != nil {
            fmt.Println("accept error:", err) 
            continue
        }

        go handleConnection(conn)
    }
}
