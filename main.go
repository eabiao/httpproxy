package main

import (
	"bufio"
	"bytes"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

func main() {
	listenAddr := "127.0.0.1:7777"

	ssk, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("listen:", listenAddr)

	for {
		sk, err := ssk.Accept()
		if err != nil {
			log.Println(err)
		}

		go handleConnect(sk)
	}
}

// 处理请求
func handleConnect(client net.Conn) {
	defer client.Close()
	client.(*net.TCPConn).SetKeepAlive(true)

	req, err := parseRequest(client)
	if err != nil {
		return
	}
	log.Println(req.addr)

	if req.isHttps {
		client.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
	}

	target, err := net.DialTimeout("tcp", req.addr, 2*time.Second)
	if err != nil {
		return
	}
	defer target.Close()
	target.(*net.TCPConn).SetKeepAlive(true)

	if !req.isHttps {
		target.Write(req.data)
	}

	relay(client, target)
}

// http请求
type HttpRequest struct {
	isHttps bool
	addr    string
	data    []byte
}

// 解析请求
func parseRequest(client net.Conn) (*HttpRequest, error) {

	var isHttps bool
	var addr string
	var buff bytes.Buffer

	br := bufio.NewReader(client)
	for {
		line, err := br.ReadString('\n')
		if err != nil {
			return nil, err
		}
		buff.WriteString(line)

		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		} else if strings.HasPrefix(line, "CONNECT") {
			isHttps = true
		} else if strings.HasPrefix(line, "Host:") {
			addr = strings.Fields(line)[1]
		}
	}

	if !strings.Contains(addr, ":") {
		if isHttps {
			addr = addr + ":443"
		} else {
			addr = addr + ":80"
		}
	}

	request := &HttpRequest{
		isHttps: isHttps,
		addr:    addr,
		data:    buff.Bytes(),
	}
	return request, nil
}

// 数据传输
func relay(left, right net.Conn) (int64, int64) {
	ch := make(chan int64)

	go func() {
		reqN, _ := io.Copy(right, left)
		right.SetDeadline(time.Now())
		left.SetDeadline(time.Now())
		ch <- reqN
	}()

	respN, _ := io.Copy(left, right)
	right.SetDeadline(time.Now())
	left.SetDeadline(time.Now())
	reqN := <-ch

	return reqN, respN
}
