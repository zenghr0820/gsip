package sip

import (
	"fmt"
	"net"

	"github.com/zenghr0820/gsip/logger"
)

// 实现 server 接口
type UdpServer struct {
	Network string
	Host    string
	Port    uint16
}

func (udp *UdpServer) Start() {
	fmt.Println("Start UdpServer...")
	logger.Info("Start UdpServer...")
	// 监听
	udp.listener()

	fmt.Println("-> Start Success")
}

func (udp *UdpServer) Stop() {
	fmt.Println("Stop...")
}

func (udp *UdpServer) listener() {
	tcpIp := fmt.Sprintf("%s:%d", udp.Host, udp.Port)
	fmt.Printf("Listener IP -> %s", tcpIp)
	// 1. 获取TCP地址
	addr, err := net.ResolveUDPAddr(udp.Network, tcpIp)
	if err != nil {
		println("ResolveTCPAddr Error: ", err)
		return
	}
	// 2. 监听 ip
	udpConn, err := net.ListenUDP(udp.Network, addr)
	if err != nil {
		println("ListenTCP Error: ", err)
		return
	}

	for {
		// 处理请求
		buf := make([]byte, 1024)
		num, udpAddr, err := udpConn.ReadFrom(buf)
		if err != nil {
			println("handleRead Addr Error: ", udpAddr.String())
			println("handleRead Error: ", err)
			return
		}
		fmt.Printf("Read len: %d", num)
		fmt.Printf("Read: %s", buf)
	}

	//go handleRequest(udpConn)

}

func NewUdpServer(host string, port uint16) Server {
	return &UdpServer{
		Network: "udp",
		Host:    host,
		Port:    port,
	}
}

//func handleRequest(conn *net.UDPConn) {
//
//}
