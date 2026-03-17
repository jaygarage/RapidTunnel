/**
 * @author: Jay (改进)
 * @date: 2025/10/22
 * @file: https.go
 * @description: https 隧道协议（支持二级代理，代理非200时把代理响应完整返回给客户端）
 */

package httpx

import (
	"RapidTunnel/proxy"
	"RapidTunnel/proxy/tools"
	"RapidTunnel/utils/settings"
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// HandleHTTPSProxy 处理 HTTPS CONNECT 请求，建立 TCP 隧道
func HandleHTTPSProxy(srcConn net.Conn, req *http.Request, params *proxy.QueryParams) {
	var dstConn net.Conn
	var resp *http.Response

	if !settings.TunneledOrNot {
		// 普通直连模式
		dstConn, resp = handleDirectHTTPS(srcConn, req)
	} else {
		// 链式代理模式（会把 srcConn 传入以便在隧道建立成功时向客户端返回 200）
		dstConn, resp = handleChainedHTTPS(srcConn, req, params)
	}

	// 如果出现异常响应（非 nil），统一写回客户端并返回
	if resp != nil {
		_ = resp.Write(srcConn)
		_ = srcConn.Close()
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
		return
	}

	// 启动双向数据转发（阻塞直到任一端关闭）
	tools.ForwardData(srcConn, dstConn)
}

// handleDirectHTTPS：直接连接目标主机（直连隧道）
// 返回 dstConn（已连接到目标的 net.Conn），或在失败时返回用于写回客户端的 *http.Response
func handleDirectHTTPS(srcConn net.Conn, req *http.Request) (net.Conn, *http.Response) {
	targetAddr := req.Host
	// 使用超时拨号器
	dialer := &net.Dialer{Timeout: settings.Timeout}
	dstConn, err := dialer.Dial("tcp", targetAddr)
	if err != nil {
		return nil, tools.CreateAnException("请求目标服务器失败", http.StatusServiceUnavailable, err, req)
	}

	// 动态生成协议版本，不写死 HTTP/1.1
	proto := fmt.Sprintf("HTTP/%d.%d", req.ProtoMajor, req.ProtoMinor)

	// 写回客户端 200，表示隧道建立成功
	if _, err = srcConn.Write([]byte(fmt.Sprintf("%s 200 Connection Established\r\n\r\n", proto))); err != nil {
		_ = dstConn.Close()
		return nil, tools.CreateAnException("隧道建立失败（写回客户端失败）", http.StatusServiceUnavailable, err, req)
	}

	return dstConn, nil
}

// handleChainedHTTPS：通过二级代理建立隧道（并在二级代理返回非 200 时把其响应体完整返回给客户端）
func handleChainedHTTPS(srcConn net.Conn, req *http.Request, params *proxy.QueryParams) (net.Conn, *http.Response) {
	targetHost := req.Host
	protocol := "http"

	// 获取二级代理配置（假设 GetProxy 返回 *url.URL, error, extra）
	secondProxyURL, proxyErr, _ := proxy.GetProxy(&protocol, params)
	if proxyErr != nil {
		return nil, tools.CreateAnException("无法获取二级代理配置", http.StatusServiceUnavailable, proxyErr, req)
	}

	// 确保 proxyHost 有端口（默认 http:80）
	proxyHost := secondProxyURL.Host
	if _, _, err := net.SplitHostPort(proxyHost); err != nil {
		// 没有端口，默认 80
		proxyHost = net.JoinHostPort(proxyHost, "80")
	}

	// 连接到二级代理
	proxyConn, err := net.DialTimeout("tcp", proxyHost, settings.Timeout)
	if err != nil {
		return nil, tools.CreateAnException("无法连接二级代理", http.StatusServiceUnavailable, err, req)
	}

	// 构造 CONNECT 请求（使用动态协议版本）
	connectReq := buildConnectRequest(req, secondProxyURL, targetHost)

	// 发送 CONNECT 请求到二级代理
	if _, err = proxyConn.Write([]byte(connectReq)); err != nil {
		_ = proxyConn.Close()
		return nil, tools.CreateAnException("发送 CONNECT 请求到二级代理失败", http.StatusServiceUnavailable, err, req)
	}

	// 读取二级代理响应（使用 bufio.Reader，以供 http.ReadResponse 使用）
	br := bufio.NewReader(proxyConn)
	proxyResp, err := http.ReadResponse(br, req)
	if err != nil {
		_ = proxyConn.Close()
		return nil, tools.CreateAnException("读取二级代理响应失败", http.StatusServiceUnavailable, err, req)
	}

	// 若代理返回非 200，则把代理的完整响应（包括 body）读出并返回给客户端
	if proxyResp.StatusCode != http.StatusOK {
		// 读取代理返回的 body 完整内容（缓冲到内存）
		bodyBuf, readErr := io.ReadAll(proxyResp.Body)
		_ = proxyResp.Body.Close()
		if readErr != nil {
			_ = proxyConn.Close()
			return nil, tools.CreateAnException("读取二级代理响应体失败", http.StatusServiceUnavailable, readErr, req)
		}
		// 更新响应 body 与 Content-Length、移除 chunked encoding（以便安全写回）
		proxyResp.Body = io.NopCloser(bytes.NewReader(bodyBuf))
		proxyResp.ContentLength = int64(len(bodyBuf))
		proxyResp.Header.Del("Transfer-Encoding")

		// 关闭到二级代理的连接（不作为隧道）
		_ = proxyConn.Close()

		// 返回该响应给上层，HandleHTTPSProxy 会写回客户端
		return nil, proxyResp
	}

	// 二级代理返回 200，表示代理已经建立到目标的隧道（或代理本身作为隧道）
	// 向最初的客户端写回 200 表示隧道建立成功（与直连模式一致）
	proto := fmt.Sprintf("HTTP/%d.%d", req.ProtoMajor, req.ProtoMinor)
	if _, err = srcConn.Write([]byte(fmt.Sprintf("%s 200 Connection Established\r\n\r\n", proto))); err != nil {
		_ = proxyConn.Close()
		return nil, tools.CreateAnException("向客户端返回 200 失败", http.StatusServiceUnavailable, err, req)
	}

	// 注意：http.ReadResponse 用 br 读完响应头后停止，br 可能已经从 proxyConn 缓存了部分数据，
	// 但剩余的数据（如果有）仍会由 proxyConn 提供给后续的 io.Copy，因此直接返回 proxyConn 即可作为隧道。
	return proxyConn, nil
}

// buildConnectRequest 构造 CONNECT 请求字符串，支持 Basic 认证，使用请求的协议版本（动态）
func buildConnectRequest(req *http.Request, proxyURL *url.URL, targetHost string) string {
	proto := fmt.Sprintf("HTTP/%d.%d", req.ProtoMajor, req.ProtoMinor)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("CONNECT %s %s\r\n", targetHost, proto))
	sb.WriteString(fmt.Sprintf("Host: %s\r\n", targetHost))

	// 如果二级代理 URL 带有认证信息，写 Proxy-Authorization：Basic ...
	if proxyURL != nil && proxyURL.User != nil {
		username := proxyURL.User.Username()
		password, _ := proxyURL.User.Password()
		auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
		sb.WriteString(fmt.Sprintf("Proxy-Authorization: Basic %s\r\n", auth))
	}
	sb.WriteString("\r\n")
	return sb.String()
}
