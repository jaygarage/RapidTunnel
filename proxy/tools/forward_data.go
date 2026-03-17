/**
 * @author: Jay
 * @date: 2025/3/12
 * @file: forward_data.go
 * @description: 双向隧道转发
 */

package tools

import (
	"RapidTunnel/utils/logrus"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var connID int64

// 判断是否可以忽略的网络错误
func isIgnorableErr(err error) bool {
	if err == nil || err == io.EOF {
		return true
	}

	errString := err.Error()
	if strings.Contains(errString, "use of closed network connection") {
		return true
	}
	if strings.Contains(errString, "forcibly closed") {
		return true
	}
	if strings.Contains(errString, "i/o timeout") {
		return true
	}
	return false
}

// TCP优化
func optimizeTCP(conn net.Conn) {
	if tcp, ok := conn.(*net.TCPConn); ok {
		_ = tcp.SetNoDelay(true)
		_ = tcp.SetKeepAlive(true)
		_ = tcp.SetKeepAlivePeriod(60 * time.Second)
	}
}

// transfer
func transfer(src, dst net.Conn, cancel context.CancelFunc, wg *sync.WaitGroup) {
	defer wg.Done()

	// 尝试零拷贝
	if tcpSrc, ok1 := src.(*net.TCPConn); ok1 {
		if tcpDst, ok2 := dst.(*net.TCPConn); ok2 {
			if _, err := tcpDst.ReadFrom(tcpSrc); err != nil && !isIgnorableErr(err) {
				logrus.Warnf("ReadFrom error (%v -> %v): %v", src.RemoteAddr(), dst.RemoteAddr(), err)
			}
			cancel()
			return
		}
	}

	// fallback
	if _, err := io.Copy(dst, src); err != nil && !isIgnorableErr(err) {
		logrus.Warnf("io.Copy error (%v -> %v): %v", src.RemoteAddr(), dst.RemoteAddr(), err)
	}

	cancel()
}

// 日志：耗时打印
func logForward(c1, c2 net.Conn) func() {
	id := atomic.AddInt64(&connID, 1) % 1_000_000
	start := time.Now()

	src := c1.RemoteAddr().String()
	dst := c2.RemoteAddr().String()

	logrus.Debugf("🔄 [%d] %s ⇄ %s 开始", id, src, dst)
	return func() {
		logrus.Debugf("✅ [%d] %s ⇄ %s 结束 | ⏱ %.2fms", id, src, dst, float64(time.Since(start).Milliseconds()))
	}
}

// ForwardData 双向转发
func ForwardData(c1, c2 net.Conn) {
	defer logForward(c1, c2)()

	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	optimizeTCP(c1)
	optimizeTCP(c2)

	wg.Add(2)
	go transfer(c1, c2, cancel, &wg)
	go transfer(c2, c1, cancel, &wg)

	// 任意方向结束
	<-ctx.Done()

	// 强制打断所有 IO
	_ = c1.SetDeadline(time.Now())
	_ = c2.SetDeadline(time.Now())

	// 关闭连接（释放 fd）
	_ = c1.Close()
	_ = c2.Close()

	// 等 goroutine 完全退出
	wg.Wait()
}

// CreateAnException 创建隧道异常答复
func CreateAnException(body string, StatusCode int, err error, req *http.Request) *http.Response {
	errMsg := strings.TrimSpace(fmt.Sprintf("%s ⇆ %s", body, err.Error()))
	resp := &http.Response{
		StatusCode:    StatusCode,
		Status:        fmt.Sprintf("%d %s", StatusCode, http.StatusText(StatusCode)),
		Proto:         req.Proto,
		ProtoMajor:    req.ProtoMajor,
		ProtoMinor:    req.ProtoMinor,
		Request:       req,
		Header:        make(http.Header),
		ContentLength: int64(len(errMsg)),
		Body:          ioutil.NopCloser(bytes.NewReader([]byte(errMsg))),
	}
	resp.Header.Set("Proxy-Authenticate", `Basic realm="MyProxy"`)
	resp.Header.Set("Content-Type", "text/plain; charset=utf-8")
	resp.Header.Set("Connection", "close")
	return resp
}
