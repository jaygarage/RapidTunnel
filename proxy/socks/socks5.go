/**
 * @author: Jay
 * @date: 2025/3/11
 * @file: socks5.go
 * @description: SOCKS5 隧道代理转发（优化合并回复处理）
 */

package socks

import (
	"RapidTunnel/proxy"
	"RapidTunnel/proxy/tools"
	"RapidTunnel/utils/settings"
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
)

// VER (socksVersion5): 版本号字段，通常是 0x05，表示 SOCKS5 协议。
// REP (rep): 回复码，表示连接请求的结果。例如 0x00 表示成功，0x01 表示失败等。
// RSV (0x00): 保留字节，始终设置为 0x00，不使用。
// ATYP: 地址类型字段，标识地址类型（如 IPv4、域名或 IPv6 地址等）。
// BND.ADDR: 绑定地址。根据 ATYP 的不同，绑定地址会有所不同：
// 如果 ATYP 为 0x01（IPv4 地址类型），那么绑定地址是一个 4 字节的 IPv4 地址。
// 如果 ATYP 为 0x03（域名地址类型），绑定地址则是一个域名字符串。
// 如果 ATYP 为 0x04（IPv6 地址类型），绑定地址是一个 16 字节的 IPv6 地址。
// BND.PORT: 绑定端口，是 2 字节的端口号。
const (
	socksVersion5 = uint8(0x05)

	// CD +----+-----+-------+--------+-----+--------+-----+-------+
	// | VN | CD  | RSV   | DST.ADDR       | DST.PORT |
	// +----+-----+-------+--------+-----+--------+-----+-------+
	// | 1  | 1   | 1     | variable       | 2       |
	// +----+-----+-------+--------+-----+--------+-----+-------+
	// 命令码 (CD)：
	// 0x01：请求建立连接（CONNECT）。
	// 0x02：请求绑定（BIND）。
	// 0x03：请求 UDP 转发（UDP ASSOCIATE）。
	CD = uint8(0x01)

	CommandConnect = 0x01 // CONNECT 命令
	StatusFailure  = 0x01 // 状态码 0x01 表示失败
	Reserved       = 0x00 // 保留字段
	IPv4AddrType   = 0x01 // IPv4 地址类型
	DomainAddrType = 0x03 // 域名地址类型
	IPv6AddrType   = 0x04 // IPv6 地址类型

	NoAuthMethod           = 0x00
	UsernamePasswordMethod = 0x02
	UnsupportedMethod      = 0x08

	AuthMethodNoAuth       = 0x00 // 无认证方法
	AuthMethodUsernamePass = 0x02 // 用户名密码认证方法
	AuthStatusSuccess      = 0x00 // 认证成功状态
	AuthStatusFailure      = 0x01 // 认证失败状态
	AuthMethodCount        = 0x01 // 认证方法数，这里只有一个方法

	VN            = uint8(0x00)
	StatusSuccess = uint8(0x00) // 连接成功
)

type socks5 struct {
	command byte
	header  []byte
	dstAddr string
	user    string
}

// handleSocks5 实现 SOCKS5 隧道代理功能的处理结构体
type handleSocks5 struct {
	conn   net.Conn
	reader *bufio.Reader
}

// readExactly 从 conn 中读取指定长度 n 的数据
func readExactly(conn net.Conn, n int) ([]byte, error) {
	buf := make([]byte, n)
	totalRead := 0
	for totalRead < n {
		count, err := conn.Read(buf[totalRead:])
		if err != nil {
			return nil, err
		}
		totalRead += count
	}
	return buf, nil
}

// parseDestination 从客户端连接中解析目标地址和端口
func (hs5 *handleSocks5) parseDestination() (socks5, error) {
	s5 := socks5{}

	// 读取前4字节：版本、命令、保留、地址类型
	header, err := readExactly(hs5.conn, 4)
	if err != nil {
		return s5, err
	}
	s5.header = header
	//logrus.Infof("正在建立 HandleSocks5 隧道代理 >>> %v", header)

	addrType := header[3]
	var destAddr string
	// 根据地址类型读取目标地址
	switch addrType {
	case IPv4AddrType: // IPv4 地址类型
		addr, err := readExactly(hs5.conn, 4)
		if err != nil {
			return s5, err
		}
		destAddr = net.IP(addr).String()

	case DomainAddrType: // 域名类型
		// 先读取域名长度（1字节）
		lenBuf, err := readExactly(hs5.conn, 1)
		if err != nil {
			return s5, err
		}
		domainLen := int(lenBuf[0])
		// 再读取具体的域名
		domain, err := readExactly(hs5.conn, domainLen)
		if err != nil {
			return s5, err
		}
		destAddr = string(domain)

	case IPv6AddrType: // IPv6 地址类型
		addr, err := readExactly(hs5.conn, 16)
		if err != nil {
			return s5, err
		}
		destAddr = net.IP(addr).String()

	default:
		// 地址类型不支持时，回复错误给客户端
		hs5.sendReply(UnsupportedMethod, nil)
		return s5, fmt.Errorf("不支持的地址类型: %d", addrType)
	}

	// 读取目标端口（2字节）
	portBuf, err := readExactly(hs5.conn, 2)
	if err != nil {
		return s5, err
	}
	port := int(portBuf[0])<<8 | int(portBuf[1])

	dstAddr := net.JoinHostPort(destAddr, strconv.Itoa(port))
	s5.dstAddr = dstAddr
	return s5, nil
}

// sendReply 构造并发送 SOCKS5 协议回复，告知客户端连接状态
// rep：回复状态码，addr：绑定地址（nil时默认返回 IPv4 0.0.0.0:0）
func (hs5 *handleSocks5) sendReply(rep byte, addr net.Addr) error {
	// 构造格式：VER, REP, RSV, ATYP, BND.ADDR, BND.PORT
	reply := []byte{socksVersion5, rep, Reserved}
	if addr == nil {
		// 未提供地址时，默认返回 IPv4 0.0.0.0:0
		reply = append(reply, IPv4AddrType)
		reply = append(reply, []byte{0, 0, 0, 0}...)
		reply = append(reply, Reserved, Reserved)
	} else {
		if tcpAddr, ok := addr.(*net.TCPAddr); ok {
			ip := tcpAddr.IP
			if ip == nil {
				ip = net.IPv4zero
			}
			if ip4 := ip.To4(); ip4 != nil {
				reply = append(reply, IPv4AddrType) // IPv4 地址类型
				reply = append(reply, ip4...)
			} else if ip16 := ip.To16(); ip16 != nil {
				reply = append(reply, IPv6AddrType) // IPv6 地址类型
				reply = append(reply, ip16...)
			} else {
				reply = append(reply, IPv4AddrType)
				reply = append(reply, []byte{0, 0, 0, 0}...)
			}
			// 追加端口
			portBytes := make([]byte, 2)
			binary.BigEndian.PutUint16(portBytes, uint16(tcpAddr.Port))
			reply = append(reply, portBytes...)
		} else {
			// 非 TCPAddr 类型，默认返回 IPv4 0.0.0.0:0
			reply = append(reply, IPv6AddrType)
			reply = append(reply, []byte{0, 0, 0, 0}...)
			reply = append(reply, Reserved, Reserved)
		}
	}
	_, err := hs5.conn.Write(reply)
	return err
}

// connected 主流程函数：解析目标连接、建立与二级代理的隧道连接，并回复客户端连接状态
func (hs5 *handleSocks5) connected(params *proxy.QueryParams) (net.Conn, error) {
	// 解析目标地址和端口
	h5, err := hs5.parseDestination()
	if err != nil {
		return nil, err
	}
	//logrus.Info("目标地址: ", h5.dstAddr)

	// 根据配置判断是否通过二级代理建立连接
	var targetConn net.Conn
	if settings.TunneledOrNot {
		targetConn, err = connectThroughSecondaryProxy(h5.dstAddr, params)
		if err != nil {
			return nil, err
		}
	} else {
		targetConn, err = net.Dial("tcp", h5.dstAddr)
		if err != nil {
			return nil, err
		}
	}

	// 连接成功后，回复客户端连接建立成功的信息（状态码 0x00）
	if replyErr := hs5.sendReply(StatusSuccess, targetConn.LocalAddr()); replyErr != nil {
		return nil, err
	}
	return targetConn, nil
}

// HandleSocks5 实现 SOCKS5 隧道代理功能的主入口
func HandleSocks5(srcConn net.Conn, reader *bufio.Reader) {
	hs5 := handleSocks5{conn: srcConn, reader: reader}
	NUA := NewUserAuthentication(srcConn, reader)

	ok, _ := NUA.Validation() // 用户鉴权
	if !ok {
		srcConn.Write([]byte{AuthMethodCount, AuthStatusFailure})
		return

	}
	dstConn, err := hs5.connected(NUA.params)
	if err != nil {
		//logrus.Warnf("NewHandleSocks5 创建连接失败: %s", err.Error())
		return
	}

	defer dstConn.Close()
	tools.ForwardData(srcConn, dstConn)
}
