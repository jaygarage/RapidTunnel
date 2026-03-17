/**
 * @author: Jay
 * @date: 2025/3/13
 * @file: install_redis_test.go
 * @description:
 */

package install_redis

import (
	"RapidTunnel/utils/logrus"
	"RapidTunnel/utils/settings"
	"testing"
)

func TestInstallRedis(t *testing.T) {
	// redis 主从
	//replicaof 47.115.144.160 6379
	//masterauth Alsdawo*Z96@J!Nddawaw4k#R1$hjJ8v!1daw1reach

	// 卸载 redis
	// sudo systemctl stop redis-server  # 停止 Redis 服务
	// sudo apt-get purge --auto-remove -y redis-server
	// 打包：$env:GOARCH="amd64";$env:GOOS="linux";go build -o install_redis ./mian3.go

	err := SetupRedis(true, settings.RedisPassword)
	if err != nil {
		logrus.Warnf("redis 初始化安装异常 --> %s", err.Error())
		return
	}
}
