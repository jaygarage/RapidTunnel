/**
 * @author: Jay
 * @date: 2025/3/11
 * @file: http.go
 * @description: http 隧道协议
 */

package httpx

import (
	"RapidTunnel/proxy"
	"RapidTunnel/proxy/tools"
	"RapidTunnel/utils/settings"
	"fmt"
	"net"
	"net/http"
)

// HandleHTTPProxy 处理普通 HTTP 请求，通过 RoundTrip 转发请求并将响应返回给客户端
func HandleHTTPProxy(clientConn net.Conn, req *http.Request, params *proxy.QueryParams) {
	// 创建 HTTP 客户端 Transport，并设置自定义 DialContext
	defaultTransport := &http.Transport{
		//DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
		//	dialer := &net.Dialer{
		//		Timeout:   10 * time.Second,
		//		KeepAlive: 10 * time.Second,
		//	}
		//	return dialer.DialContext(ctx, network, addr)
		//},
		ForceAttemptHTTP2:     true,
		IdleConnTimeout:       settings.IdleConnTimeout,
		TLSHandshakeTimeout:   settings.TLSHandshakeTimeout,
		ExpectContinueTimeout: settings.ExpectContinueTimeout,
		//MaxIdleConns:          2048,
		//MaxIdleConnsPerHost:   256,
	}

	// 从环境变量获取代理设置
	if settings.TunneledOrNot {
		protocol := "http"
		fixedURL, fixedURLErr, _ := proxy.GetProxy(&protocol, params)
		if fixedURLErr != nil {
			resp := tools.CreateAnException("无法完成您的请求", http.StatusServiceUnavailable, fixedURLErr, req)
			if err := resp.Write(clientConn); err != nil {
				fmt.Println("写入响应错误:", err)
				return
			}
			return
		}
		defaultTransport.Proxy = http.ProxyURL(fixedURL)
	}

	// 发送 HTTP 请求到目标服务器
	resp, err := defaultTransport.RoundTrip(req)
	if err != nil {
		// 创建错误响应
		resp = tools.CreateAnException("请求目标服务器失败", http.StatusServiceUnavailable, err, req)
	}
	defer resp.Body.Close()

	// 将目标服务器的响应写回给客户端
	err = resp.Write(clientConn)
	if err != nil {
		fmt.Println("写入响应错误:", err)
		return
	}
}
