/**
 * @author: Jay
 * @date: 2025/3/12
 * @file: socks5SecondaryProxy.go
 * @description: 创建二级代理转发
 */

package socks

import (
	"RapidTunnel/proxy"
	"RapidTunnel/utils/settings"
	"fmt"
	"net"
	"net/url"
	"strconv"
)

var name = "socks5"

// connectThroughSecondaryProxy 通过二级代理建立连接
func connectThroughSecondaryProxy(target string, params *proxy.QueryParams) (net.Conn, error) {
	// 获取代理
	secondaryProxy, _, fixedURLErr := proxy.GetProxy(&name, params)
	if fixedURLErr != nil {
		return nil, fmt.Errorf("无法连接到二级代理: %s", fixedURLErr)
	}

	// 连接二级代理
	secondaryConn, err := net.DialTimeout("tcp", secondaryProxy.Host, settings.Timeout)
	if err != nil {
		return nil, fmt.Errorf("无法连接到二级代理: %s", err)
	}

	// 发送SOCKS5认证请求（用户名密码认证）
	if err := sendSocks5AuthRequest(secondaryProxy, secondaryConn); err != nil {
		secondaryConn.Close()
		return nil, err
	}

	// 构建并发送二级代理请求（假设是 SOCKS5 协议）
	connectReq, _ := buildSocks5ConnectRequest(target)
	if _, err := secondaryConn.Write(connectReq); err != nil {
		secondaryConn.Close()
		return nil, fmt.Errorf("通过二级代理发送请求失败: %s", err)
	}

	// 等待二级代理回复
	connectResp, err := readExactly(secondaryConn, 10) // 假设回复 10 字节
	if err != nil {
		secondaryConn.Close()
		return nil, fmt.Errorf("接收二级代理回复失败: %s", err)
	}

	// 检查二级代理是否连接成功
	if connectResp[1] != StatusSuccess {
		secondaryConn.Close()
		return nil, fmt.Errorf("二级代理连接失败: 错误代码 %x", connectResp[1])
	}

	// 返回通过二级代理建立的连接
	return secondaryConn, nil
}

// sendSocks5AuthRequest 向二级代理发送用户名密码认证请求
func sendSocks5AuthRequest(secondProxyURL *url.URL, conn net.Conn) error {
	// SOCKS5 身份验证请求结构：版本、认证方法数、认证方法（用户名密码认证）
	authReq := []byte{socksVersion5, AuthMethodUsernamePass, Reserved, UsernamePasswordMethod} /// 0x05: SOCKS5, 0x01: 认证方法数, 0x02: 用户名密码认证方法
	if _, err := conn.Write(authReq); err != nil {
		return fmt.Errorf("发送认证请求失败: %s", err)
	}

	// 等待 SOCKS5 认证响应
	authResp, err := readExactly(conn, 2) // 预计 2 字节响应
	if err != nil {
		return fmt.Errorf("接收认证响应失败: %s", err)
	}
	//logrus.Info(authResp)

	// 检查认证是否成功，仅支持0x02
	if authResp[1] != AuthMethodUsernamePass {
		return fmt.Errorf("SOCKS5 认证方式不存在: 错误代码 %x", authResp[1])
	}
	//logrus.Info(authResp)

	username := secondProxyURL.User.Username()
	password, _ := secondProxyURL.User.Password()
	// 发送用户名密码进行认证
	authDetails := []byte{byte(len(username))}
	authDetails = append(authDetails, []byte(username)...)
	authDetails = append(authDetails, byte(len(password)))
	authDetails = append(authDetails, []byte(password)...)
	//logrus.Info(authDetails)

	// 发送认证信息
	authReq = append([]byte{CommandConnect}, authDetails...) // 0x01 表示用户名密码认证请求
	if _, err := conn.Write(authReq); err != nil {
		return fmt.Errorf("发送用户名密码认证请求失败: %s", err)
	}

	// 等待认证响应
	authResp, err = readExactly(conn, 2) // 预计 2 字节响应
	if err != nil {
		return fmt.Errorf("接收用户名密码认证响应失败: %s", err)
	}

	// 检查用户名密码认证是否成功
	if authResp[1] != AuthStatusSuccess {
		return fmt.Errorf("SOCKS5 用户名密码认证失败: 错误代码 %x", authResp[1])
	}
	//logrus.Info(authResp)

	return nil
}

// buildSocks5ConnectRequest 构建 SOCKS5 CONNECT 请求
func buildSocks5ConnectRequest(target string) ([]byte, error) {
	// 解析目标地址和端口
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		return nil, fmt.Errorf("解析目标地址失败: %s", err)
	}

	// 解析端口号
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return nil, fmt.Errorf("无效的端口号: %s", err)
	}

	// 解析目标地址类型
	ip := net.ParseIP(host)
	var request []byte

	// 如果是IPv4地址
	if ipv4Addr := ip.To4(); ipv4Addr != nil {
		// 构建SOCKS5请求: [版本, 命令, 保留字段, 地址类型(IPv4), 地址, 端口]
		request = []byte{socksVersion5, CommandConnect, Reserved, IPv4AddrType} // IPv4 地址类型
		request = append(request, ipv4Addr...)
		request = append(request, byte(portNum>>8), byte(portNum&0xff))

	} else if ip6Addr := ip.To16(); ip6Addr != nil {
		// 如果是IPv6地址
		// 构建SOCKS5请求: [版本, 命令, 保留字段, 地址类型(IPv6), 地址, 端口]
		request = []byte{socksVersion5, CommandConnect, Reserved, IPv6AddrType} // IPv6 地址类型
		request = append(request, ip6Addr...)
		request = append(request, byte(portNum>>8), byte(portNum&0xff))

	} else {
		// 如果是域名地址
		// 构建SOCKS5请求: [版本, 命令, 保留字段, 地址类型(域名), 地址长度, 地址, 端口]
		request = []byte{socksVersion5, CommandConnect, Reserved, DomainAddrType} // 域名地址类型
		request = append(request, byte(len(host)))                                // 域名长度
		request = append(request, host...)                                        // 域名
		request = append(request, byte(portNum>>8), byte(portNum&0xff))           // 端口号
	}

	return request, nil
}
