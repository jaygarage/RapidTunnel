/**
 * @author: Jay
 * @date: 2025/3/11
 * @file: settings.go
 * @description:
 */

package settings

import "time"

var (
	Port    = 80
	Address = "0.0.0.0"

	// ProjectName 项目名，必须与打包的名称一样
	ProjectName   = "RapidTunnel"
	TunneledOrNot = false // 默认 false

	// ProxyPassword ProxyUsername 普通代理的默认账号密码
	ProxyPassword = "test123456TEST"
	ProxyUsername = "test"

	Timeout = 30 * time.Second // CONNECT 隧道超时时间

	IdleConnTimeout       = 30 * time.Second // 连接超时
	TLSHandshakeTimeout   = 20 * time.Second // 握手超时时间
	ExpectContinueTimeout = 10 * time.Second // 预期继续超时时间

	RedisAddr     = "127.0.0.1"
	RedisPort     = 6379
	RedisPassword = "Alsdawo*Z96@J!Nddawaw4k#R1$hjJ8v!1daw1reach"
	RedisDB       = 0
)
