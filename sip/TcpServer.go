package sip

import (
	"fmt"
	"net"
)

// 实现 server 接口
type TcpServer struct {
	Network string
	Host    string
	Port    uint16
}

func (tcp *TcpServer) Start() {
	fmt.Println("Start TcpServer...")
	// 监听
	tcp.listener()

	fmt.Println("-> Start Success")
}

func (tcp *TcpServer) Stop() {
	fmt.Println("Stop...")
}

func (tcp *TcpServer) listener() {
	tcpIp := fmt.Sprintf("%s:%d", tcp.Host, tcp.Port)
	fmt.Printf("Listener IP -> %s", tcpIp)
	// 1. 获取TCP地址
	addr, err := net.ResolveTCPAddr(tcp.Network, tcpIp)
	if err != nil {
		println("ResolveTCPAddr Error: ", err)
		return
	}
	// 2. 监听 ip
	tcpListen, err := net.ListenTCP(tcp.Network, addr)
	if err != nil {
		println("ListenTCP Error: ", err)
		return
	}

	// 阻塞监听
	for {
		conn, err := tcpListen.AcceptTCP()
		if err != nil {
			println("AcceptTCP Error: ", err)
			continue
		}
		fmt.Printf("handleRequest -> %s", conn.RemoteAddr())
		// 处理请求
		go handleRequest(conn)
	}
}

func NewTcpServer(host string, port uint16) Server {
	return &TcpServer{
		Network: "tcp",
		Host:    host,
		Port:    port,
	}
}

func handleRequest(conn *net.TCPConn) {
	for {
		buf := make([]byte, 1024)
		num, err := conn.Read(buf)
		if err != nil {
			println("handleRead Error: ", err)
			return
		}
		fmt.Printf("Read len: %d\n", num)
		fmt.Printf("Read: %s\n", buf)
	}
}
