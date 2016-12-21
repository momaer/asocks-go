package main

import (
	"asocks"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"runtime"
	"time"
	"math/rand"
)

func handleConnection(conn *net.TCPConn) {
	closed := false

	defer func() {
		if !closed {
			conn.Close()
		}
	}()

	var n int
	var err error
	buf := make([]byte, 257)

	if n, err = io.ReadAtLeast(conn, buf, 2); err != nil {
		fmt.Println(err)
		return
	}

	if buf[0] != 0x05 {
		fmt.Printf("握手时读取到的socks版本是%d。\n", buf[0])
		return
	}

	nmethods := int(buf[1])
	if n < nmethods+2 {
		if _, err = io.ReadFull(conn, buf[n:nmethods+2]); err != nil {
			fmt.Println(err)
			return
		}
	}

	if _, err = conn.Write([]byte{5, 0}); err != nil {
		fmt.Println("握手时，写失败。err:", err)
		return
	}

	err = getRequest(conn)
	if err != nil {
		fmt.Println("get request err:", err.Error())
		return
	}

	closed = true
	return
}

func getRequest(conn *net.TCPConn) (err error) {
	var n int
	buf := make([]byte, 260)

	if n, err = io.ReadAtLeast(conn, buf, 5); err != nil {
		return
	}

	if buf[0] != 5 && buf[1] != 1 && buf[2] != 0 {
		err = fmt.Errorf("getRequest not socks5 protocol.")
		return
	}

	var reqLen = 0

	switch buf[3] {
	case 1:
		// ipv4
		reqLen = 4 + 4 + 2
	case 3:
		// domain
		domainLen := int(buf[4])
		reqLen = 4 + 1 + domainLen + 2
	case 4:
		// ipv6
		reqLen = 4 + 16 + 2
	default:
		// unnormal, close conn
		err = fmt.Errorf("request type不正确：%d", buf[3])
		return
	}

	if n < reqLen {
		if _, err = io.ReadFull(conn, buf[n:reqLen]); err != nil {
			return
		}
	}

    // 第一个字节是噪音长度，后面的是噪音，再往后才是正常的数据
	rand.Seed(time.Now().UnixNano())
	noiseLength := rand.Intn(255)
	noiseLength = noiseLength + 1
	noise := make([]byte, noiseLength)

	rawRequest := make([]byte, reqLen - 3 + 1 + noiseLength)
    copy(rawRequest, []byte{byte(noiseLength)})
	copy(rawRequest[1:], noise)
	copy(rawRequest[1 + noiseLength:], buf[3:n])

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
		remote.Close()
		return
	}

	finishChannel := make(chan bool, 2)
	go pipeThenClose(conn, remote, finishChannel)
	pipeThenClose(remote, conn, finishChannel)
	<-finishChannel
	<-finishChannel
	conn.Close()
	remote.Close()

	return nil
}

func pipeThenClose(src, dst *net.TCPConn, finishChannel chan bool) {
	defer func() {
		src.CloseRead()
		dst.CloseWrite()
		finishChannel <- true
	}()

	buf := asocks.GetBuffer()
	defer asocks.GiveBuffer(buf)

	for {
		src.SetReadDeadline(time.Now().Add(60 * time.Second))
		n, err := src.Read(buf)
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
		data[i] ^= 128
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

	i, err := net.ResolveTCPAddr("tcp", serverAddr)
	if err != nil {
		fmt.Println("resolve ", serverAddr, " failed. err:", err)
		return
	}
	server = *i

	numCPU := runtime.NumCPU()
	runtime.GOMAXPROCS(numCPU)

	bindAddr, _ := net.ResolveTCPAddr("tcp", localAddr)
	ln, err := net.ListenTCP("tcp", bindAddr)
	if err != nil {
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
