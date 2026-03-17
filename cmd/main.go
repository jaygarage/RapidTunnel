/**
 * @author: Jay
 * @date: 2025/3/11
 * @file: main.go
 * @description: 代理服务器入口，使用结构体封装代理服务器逻辑，实现优雅退出和并发处理
 */

package main

import (
	"RapidTunnel/utils/install_redis"
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"

	"RapidTunnel/services"
	"RapidTunnel/utils/logrus"
	"RapidTunnel/utils/settings"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

func init() {
	logrus.Initialize()
}

// ProxyServer 封装代理服务器相关配置和运行状态
type ProxyServer struct {
	port     int                // 监听端口，默认 80
	address  string             // 监听地址，默认 0.0.0.0
	listener net.Listener       // TCP监听器
	ctx      context.Context    // 上下文，用于管理协程退出
	cancel   context.CancelFunc // 取消函数，触发退出
	wg       sync.WaitGroup     // 等待组，确保所有协程处理完毕后退出
	ne       net.Error          // 等待组，确保所有协程处理完毕后退出
}

// NewProxyServer 创建并初始化一个新的 ProxyServer 实例
func NewProxyServer(address string, port int) *ProxyServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &ProxyServer{
		port:    port,
		address: fmt.Sprintf("%s:%d", address, port),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// checkAddress 检查监听地址格式是否正确
func (ps *ProxyServer) checkAddress() bool {
	_, err := net.ResolveTCPAddr("tcp", ps.address)
	return err == nil
}

// createListener 创建 TCP 监听器
func (ps *ProxyServer) createListener() error {
	if !ps.checkAddress() {
		return fmt.Errorf("监听地址格式不正确: %s", ps.address)
	}
	listener, err := net.Listen("tcp", ps.address)
	if err != nil {
		return fmt.Errorf("无法监听地址 %s: %v", ps.address, err)
	}
	ps.listener = listener
	return nil
}

// Run 启动代理服务器，接收客户端连接，并交由 services 处理
func (ps *ProxyServer) Run() {
	defer ps.wg.Done()

	// 创建监听器
	if err := ps.createListener(); err != nil {
		logrus.Fatal(err)
	}
	logrus.Infof("代理服务启动 >>> 监听地址: %s", ps.address)

	ps.wg.Add(1)
	go ps.shutdown()

	for {
		// 检查是否收到退出信号
		select {
		case <-ps.ctx.Done():
			return
		default:
			// 设置短暂超时以便检查退出信号
			err := ps.listener.(*net.TCPListener).SetDeadline(time.Now().Add(3 * time.Second))
			if err != nil {
				return
			}

			conn, err := ps.listener.Accept()
			if err != nil {
				// 如果是超时错误则继续检查退出信号
				if errors.As(err, &ps.ne) || ps.ne.Timeout() || errors.As(err, &net.ErrClosed) {
					continue
				}
				logrus.Warnf("接受连接错误: %v", err)
				continue
			}

			ps.wg.Add(1)
			// 针对每个连接开启新的 goroutine 处理
			go func(c net.Conn) {
				defer ps.wg.Done()

				// 捕获 panic 防止程序崩溃
				defer func() {
					if r := recover(); r != nil {
						logrus.Errorf("处理连接时发生 panic: %v", r)
					}
				}()
				services.NewHandleClient(c)
			}(conn)
		}
	}
}

// Start 启动代理服务器，接收客户端连接，并交由 services 处理
func (ps *ProxyServer) Start() {
	ps.wg.Add(1)
	go ps.Run()
}

func (ps *ProxyServer) Wait() {
	ps.wg.Wait()
}

// shutdown 监听系统信号（SIGINT、SIGTERM），实现优雅的不退出
func (ps *ProxyServer) shutdown() {
	defer ps.wg.Done()
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	for range signalChan {
		logrus.Info("想要关闭我的隧道代理？好的再也不见！！！")
		ps.cancel()
		ps.listener.Close() // 关闭监听端口
		return
	}
}

// ScanlnWithValidation 封装了一个获取用户输入的函数，并通过 validate 回调校验输入内容。
// prompt: 提示信息格式化字符串（用 %s 或 %d 占位符填充默认值）
// defaultValue: 默认值（当用户输入为空时返回）
// validate: 自定义校验函数，返回 true 表示输入有效，否则提示错误后重新输入
// errMsg: 当输入不符合要求时显示的错误提示
func ScanlnWithValidation(prompt string, defaultValue any, validate func(string) bool, errMsg string) string {
	reader := bufio.NewReader(os.Stdin)
	fullPrompt := fmt.Sprintf(prompt, defaultValue)
	for {
		// 打印提示信息
		fmt.Print(fullPrompt)
		// 读取一行输入
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("读取输入错误：%v，请重新输入。\n", err.Error())
			continue
		}
		// 去掉末尾的换行符和空格
		input = strings.TrimSpace(input)
		// 如果用户没有输入，则使用默认值
		if input == "" {
			input = fmt.Sprintf("%v", defaultValue)
		}
		// 如果设置了校验函数，则校验输入
		if validate != nil && !validate(input) {
			fmt.Println(errMsg)
			continue
		}
		return input
	}
}

// isValidYesNo 校验函数，判断输入是否为 yes/y 或 no/n，不区分大小写
func isValidYesNo(input string) bool {
	input = strings.ToLower(input)
	return input == "yes" || input == "y" || input == "no" || input == "n"
}

// isValidPort 校验函数，判断端口号是否为数字且在 1~65535 范围内
func isValidPort(input string) bool {
	port, err := strconv.Atoi(input)
	if err != nil {
		return false
	}
	return port >= 1 && port <= 65535
}

// isValidIP 校验函数，判断输入是否为有效的 IP 地址
func isValidIP(input string) bool {
	return net.ParseIP(input) != nil
}

// isValidIP 校验函数，判断输入是否为有效的 IP 地址
func isValidRedisPass(input string) bool {
	if len(input) < 8 {
		return false
	}

	if strings.Contains(input, " ") {
		return false
	}

	return true
}

// 校验用户名
func isValidUsername(input string) bool {
	return len(input) >= 3 && !strings.Contains(input, " ")
}

// 校验密码
func isValidPassword(input string) bool {
	return len(input) >= 8 && !strings.Contains(input, " ")
}

// 安装redis
func installRedis() {
	if _, err := exec.LookPath("redis-server"); err == nil {
		rsPass, rsPassErr := install_redis.QueryPassword()
		if rsPassErr != nil {
			log.Println("获取 redis 密码异常程序退出 --> ", rsPassErr.Error())
			return
		}
		fmt.Printf("检测到 Redis 已安装, 密码：%s\n", rsPass)
		return
	}

	redisPassword := ScanlnWithValidation("设置 redis 密码 (默认%s): ", settings.RedisPassword, isValidRedisPass, "redis 密码 格式不正确")
	allowExternalNetworkAccess := strings.ToLower(ScanlnWithValidation("设置 redis 是否允许外网访问 (默认%s, 输入yes/no): ", "no", isValidYesNo, "redis 密码 格式不正确"))
	logrus.Infof("redisPassword --> %s", redisPassword)

	allowExternalNetworkAccessBool := false
	if allowExternalNetworkAccess == "yes" || allowExternalNetworkAccess == "y" {
		allowExternalNetworkAccessBool = true
	}

	err := install_redis.SetupRedis(allowExternalNetworkAccessBool, redisPassword)
	if err != nil {
		logrus.Warnf("redis 初始化安装异常 --> %s", err.Error())
		return
	}
}
func main() {
	logrus.Debug(`
============================================================
                   RapidTunnel - Jay的开源网络工具
                     Open Source Network Tool
------------------------------------------------------------
【中文说明】
本项目仅用于学习、研究及合法的网络测试用途。
作者不鼓励也不支持将本软件用于任何违法行为，包括但不限于：
- 绕过网络限制
- 未经授权访问
- 违反所在国家或地区法律法规
使用者应自行确保其行为符合所在司法辖区的相关法律法规。
因使用本软件所产生的任何法律责任或后果，均由使用者自行承担，与作者无关。
若您不同意上述声明，请勿使用本软件。

[English Disclaimer]
This project is intended ONLY for learning, research, and legitimate network testing.
The author does NOT encourage or support using this software for any illegal activities, including but not limited to:
- Bypassing network restrictions
- Unauthorized access
- Violating local laws or regulations
Users must ensure their actions comply with local laws and regulations. 
The author is NOT responsible for any legal liabilities or consequences resulting from the use of this software.
If you do NOT agree with this disclaimer, please DO NOT use this software.
============================================================
`)

	// 获取监听地址（无需校验，传 nil 即可）
	address := ScanlnWithValidation("请输入监听地址 (默认%s): ", settings.Address, isValidIP, "请输入有效的 IP 地址 (如：127.0.0.1)")

	// 获取监听端口号，校验输入是否为有效的端口号
	proxyPortStr := ScanlnWithValidation("请输入监听端口号 (默认%d): ", settings.Port, isValidPort, "请输入有效的端口号 (1-65535)")
	proxyPort, _ := strconv.Atoi(proxyPortStr)

	// 获取是否启用隧道转发，校验输入是否为 yes 或 no（不区分大小写）
	isTunnelStr := strings.ToLower(ScanlnWithValidation("是否启用隧道转发 (默认%s, 输入yes/no): ", "no", isValidYesNo, "请输入yes/no"))

	// 启用了隧道需要安装本地 redis,需要安装说明是隧道代理服务，否者不是隧道转发，普通http、https、socks5 代理
	if isTunnelStr == "yes" || isTunnelStr == "y" {
		settings.TunneledOrNot = true
		if runtime.GOOS != "windows" {
			installRedis()
		}
	} else {
		// 非隧道 -- 设置代理账号密码
		settings.ProxyUsername = ScanlnWithValidation("请输入代理账号 (默认%s): ", settings.ProxyUsername, isValidUsername, "用户名不能包含空格，且至少3个字符")
		settings.ProxyPassword = ScanlnWithValidation("请输入代理密码 (默认%s): ", settings.ProxyPassword, isValidPassword, "密码至少8位，不能包含空格")
		logrus.Infof("🔑 Proxy Credentials → 👤 %s | 🔒 %s", settings.ProxyUsername, settings.ProxyPassword)
	}

	// 创建代理服务器实例
	proxyServer := NewProxyServer(address, proxyPort)
	proxyServer.Start()

	//go func() {
	//	// go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
	//	// 查看函数的 CPU 使用时间（Top N 函数）：(pprof) top
	//	// 查看某个函数的详细性能数据：(pprof) list <function_name>
	//	// 查看每个 goroutine 的状态：(pprof) goroutine
	//	// 生成火焰图（需要 Graphviz）：(pprof) web  或者 (pprof) svg > cpu_flame.svg
	//	//quit
	//	logrus.Infof("启动 pprof 服务器，访问 http://localhost:6060/debug/pprof/")
	//	if err := http.ListenAndServe("localhost:6060", nil); err != nil {
	//		logrus.Fatalf("pprof 服务器启动失败: %v", err)
	//	}
	//}()

	proxyServer.Wait()
}
