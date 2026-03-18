/**
 * @author: Jay
 * @date: 2025/3/11
 * @file: servers.go
 * @description: 连接的客户端
 */

package services

import (
	"RapidTunnel/proxy"
	"RapidTunnel/proxy/httpx"
	"RapidTunnel/proxy/socks"
	"RapidTunnel/proxy/tools"
	"RapidTunnel/utils/logrus"
	"bufio"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
)

type HandleClient struct {
	conn net.Conn
}

// 返回 407 鉴权异常
func (hc *HandleClient) createAnException(w net.Conn, r *http.Request, err error) {
	resp := tools.CreateAnException("Proxy Authentication Required", http.StatusProxyAuthRequired, err, r)
	// 将目标服务器的响应写回给客户端
	err = resp.Write(w)
	if err != nil {
		fmt.Println("写入响应错误:", err)
		return
	}
	return
}

// 自定义路由: /ip
func (hc *HandleClient) handleCustomRoutesIP(w net.Conn, r *http.Request, params *proxy.QueryParams) {
	var (
		resp *http.Response
		body string
	)
	if params == nil {
		params = proxy.SetFieldFromQuery(r.URL.Query())
	}

	if _, parseErr, sp := proxy.GetProxy(nil, params); parseErr == nil {
		if psJSON, err := json.Marshal(sp); err != nil {
			body = "无法完成您的请求"
			resp = tools.CreateAnException(body, http.StatusServiceUnavailable, err, r)
		} else {
			body = string(psJSON)
			resp = &http.Response{
				StatusCode: http.StatusOK,
				Proto:      "HTTP/1.1",
				ProtoMajor: 1,
				ProtoMinor: 1,
				Header:     make(http.Header),
				Body:       io.NopCloser(bytes.NewReader(psJSON)),
				Request:    r,
			}
		}
	} else {
		body = "无法完成您的请求"
		resp = tools.CreateAnException(body, http.StatusServiceUnavailable, parseErr, r)
	}
	resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	err := resp.Write(w)
	if err != nil {
		logrus.Infof("Get Proxy Err: %s", err.Error())
	}
}

// 获取用户权限验证、解析过滤表单
func (hc *HandleClient) extractBasicAuthFromHeader(w net.Conn, r *http.Request) (*url.Userinfo, error, *proxy.QueryParams) {
	headerProxyAuthorization := r.Header.Get("Proxy-Authorization")
	if headerProxyAuthorization == "" {
		headerProxyAuthorization = r.Header.Get("Authorization")
	}
	r.Header.Del("Proxy-Authorization")

	// 日志：记录认证相关信息
	func(auth string) {
		// 弃用减少表格CPU渲染耗时
		//var buf bytes.Buffer
		//table := tablewriter.NewWriter(&buf)
		//table.Header([]string{"Authorization", "IP Address", "X-Forwarded-For", "Method", "URL"})
		//table.Append([]string{
		//	auth,
		//	hc.clientIP(),
		//	r.Header.Get("X-Forwarded-For"),
		//	r.Method,
		//	r.URL.String(),
		//})
		//table.Render()
		//logrus.Infof("Formatted Table Output:\n%s", buf.String())

		logrus.Infof("auth=%q client_ip=%s xff=%q method=%s url=%s", headerProxyAuthorization, hc.clientIP(), r.Header.Get("X-Forwarded-For"), func() string {
			if r.Method == "CONNECT" {
				return "https"
			}
			return "http"
		}(), r.URL.String())
	}(headerProxyAuthorization)

	proxyAuthorizationErr := fmt.Errorf("proxy authorization invalid, client ip %s authorization failed, proxy ip %s", hc.clientIP(), hc.conn.LocalAddr().String())

	// 检查是否是 `Basic` 认证
	if headerProxyAuthorization == "" || !strings.HasPrefix(headerProxyAuthorization, "Basic ") {
		hc.createAnException(w, r, proxyAuthorizationErr)
		return nil, proxyAuthorizationErr, nil
	}

	// 提取 Base64 部分
	decodedCreds, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(headerProxyAuthorization, "Basic "))
	if err != nil {
		hc.createAnException(w, r, proxyAuthorizationErr)
		return nil, proxyAuthorizationErr, nil
	}

	// 假设 auth 是 "jay|country=HK&province=HK&region=HK&isp=aws:password"
	creds := strings.SplitN(string(decodedCreds), ":", 2)
	if len(creds) != 2 {
		hc.createAnException(w, r, nil)
		return nil, proxyAuthorizationErr, nil
	}

	// 再用 | 分割用户名和参数
	up := strings.SplitN(creds[0], "|", 2)
	username := up[0]

	paramsStr := ""
	if len(up) > 1 {
		paramsStr = up[1]
	}
	params, err := url.ParseQuery(paramsStr)
	if err != nil {
		return nil, fmt.Errorf("表单解析异常: %s ", err.Error()), nil
	}

	// 生成 url.Userinfo
	user := url.UserPassword(username, creds[1])
	return user, proxyAuthorizationErr, proxy.SetFieldFromQuery(params)
}

// handlerHttpAndHttpsServer 统一处理HTTP和HTTPS请求的入口
func (hc *HandleClient) handlerHttpAndHttpsServer(w net.Conn, reader *bufio.Reader) {
	r, err := http.ReadRequest(reader)
	if err != nil {
		fmt.Println("读取请求错误:", err)
		return
	}
	defer r.Body.Close()

	user, proxyAuthorizationErr, params := hc.extractBasicAuthFromHeader(w, r)
	if user == nil || !tools.BasicAuth(user) {
		hc.createAnException(w, r, proxyAuthorizationErr)
		return
	}

	// ✅ 自定义 GET /ip 接口
	if r.Method == http.MethodGet && r.URL.Path == "/ip" {
		hc.handleCustomRoutesIP(w, r, params)
		return
	}

	switch r.Method {
	case http.MethodConnect: // https
		httpx.HandleHTTPSProxy(w, r, params)
		break
	default: // 默认 http
		httpx.HandleHTTPProxy(w, r, params)
		break
	}
}

// 获取客户端 IP
func (hc *HandleClient) clientIP() string {
	return hc.conn.RemoteAddr().(*net.TCPAddr).IP.String()
}

// handleClient 根据客户端的协议类型（SOCKS5或HTTP CONNECT）选择处理逻辑
func (hc *HandleClient) connected() {
	reader := bufio.NewReader(hc.conn)
	// 判断协议
	protocol := hc.protocol(reader)
	//logrus.Infof("客户端IP`%s` <--> 当前协议`%s`", hc.clientIP(), protocol)

	switch protocol {
	case "SOCKS5":
		socks.HandleSocks5(hc.conn, reader)
		break
	//case "SOCKS4":
	//	socks.HandleSocks4(hc.conn, reader)
	//	break
	case "http", "https":
		hc.handlerHttpAndHttpsServer(hc.conn, reader)
		break
	default:
		logrus.Warnf("未定义的协议类型.")
		return
	}
}

// Protocol 网络协议检测: http、https、socket、socket5、udp
func (hc *HandleClient) protocol(reader *bufio.Reader) string {
	// UDP检测
	if _, ok := hc.conn.(net.PacketConn); ok {
		return "UDP"
	}

	// 预读取数据，不消耗流
	heads, err := reader.Peek(1)
	if err != nil {
		log.Printf("读取数据失败: %v", err)
		return "Unknown"
	}
	// 根据读取到的字节判断协议类型
	switch heads[0] {
	case 0x05:
		reader.Discard(1)
		// SOCKS5 协议的首字节通常是 0x05
		return "SOCKS5"
	case 0x04:
		reader.Discard(1)
		// SOCKS4 协议的首字节通常是 0x04
		return "SOCKS4"
	}

	// 检查是否为 HTTPS 请求
	connectLine, _ := reader.Peek(7) // 预读取足够长的连接数据
	if strings.HasPrefix(string(connectLine), "CONNECT") {
		return "https"
	}

	// 检查 HTTP 请求
	httpLine, _ := reader.Peek(7) // 预读取足够长的 HTTP 请求头
	if strings.HasPrefix(string(httpLine), "GET") || strings.HasPrefix(string(httpLine), "POST") {
		return "http"
	}

	return "Unknown"
}

// NewHandleClient 创建新的客户端连接
func NewHandleClient(conn net.Conn) {
	defer conn.Close()

	// 会影响长连接
	//// 设置连接的读超时时间为10秒
	//err := conn.SetReadDeadline(time.Now().Add(3 * time.Minute))
	//if err != nil {
	//	logrus.Warn(err)
	//	return
	//}
	//
	//// 设置连接的写超时时间为10秒
	//err = conn.SetWriteDeadline(time.Now().Add(3 * time.Minute))
	//if err != nil {
	//	logrus.Warn(err)
	//	return
	//}

	hc := HandleClient{conn: conn}
	hc.connected()
}
