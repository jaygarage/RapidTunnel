/**
 * @author: Jay
 * @date: 2025/3/13
 * @file: install_redis.go
 * @description: 安装并配置 Redis，实现自动安装、配置允许外网访问、设置快照、修改端口、密码及开机自启
 */

package install_redis

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
)

const (
	redisConfig  = "/etc/redis/redis.conf"
	backupConfig = "/etc/redis/redis.conf.bck"
)

// redisStruct 表示 Redis 安装与配置相关信息
type redisStruct struct {
	address  string // 绑定地址（例如 "127.0.0.1" 或 "0.0.0.0"）
	password string // Redis 访问密码
}

// runCommand 是一个辅助函数，用于执行 shell 命令，并将命令输出重定向到标准输出和错误输出，便于调试
func (rs *redisStruct) runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("命令 [%s %v] 执行失败：%v", name, args, err)
	}
	return nil
}

// installRedis 检测并安装 Redis（适用于 Ubuntu/Debian 系统）
func (rs *redisStruct) installRedis() error {
	// 安装必要的依赖
	fmt.Println("安装依赖: lsb-release, curl, gpg ...")
	if err := rs.runCommand("sudo", "apt-get", "install", "-y", "lsb-release", "curl", "gpg"); err != nil {
		return fmt.Errorf("安装依赖失败：%v", err)
	}

	// 添加 Redis GPG 密钥
	fmt.Println("添加 Redis GPG 密钥 ...")
	gpgCmd := "curl -fsSL https://packages.redis.io/gpg | sudo gpg --dearmor -o /usr/share/keyrings/redis-archive-keyring.gpg"
	if err := rs.runCommand("bash", "-c", gpgCmd); err != nil {
		return fmt.Errorf("下载 Redis GPG 密钥失败：%v", err)
	}

	// 设置 GPG 文件权限
	if err := rs.runCommand("sudo", "chmod", "644", "/usr/share/keyrings/redis-archive-keyring.gpg"); err != nil {
		return fmt.Errorf("设置 GPG 文件权限失败：%v", err)
	}

	// 添加 Redis 软件源
	fmt.Println("添加 Redis 软件源 ...")
	repoCmd := `echo "deb [signed-by=/usr/share/keyrings/redis-archive-keyring.gpg] https://packages.redis.io/deb $(lsb_release -cs) main" | sudo tee /etc/apt/sources.list.d/redis.list`
	if err := rs.runCommand("bash", "-c", repoCmd); err != nil {
		return fmt.Errorf("添加 Redis 软件源失败：%v", err)
	}

	// 更新软件包列表
	fmt.Println("更新软件包列表 ...")
	if err := rs.runCommand("sudo", "apt-get", "update"); err != nil {
		return fmt.Errorf("更新 apt-get 失败：%v", err)
	}

	// 安装 Redis
	fmt.Println("安装 Redis ...")
	if err := rs.runCommand("sudo", "apt-get", "-y", "install", "redis"); err != nil {
		return fmt.Errorf("安装 Redis 失败：%v", err)
	}

	// 检查安装是否成功
	if err := rs.runCommand("redis-server", "--version"); err != nil {
		return fmt.Errorf("redis 安装后检测失败，请检查安装情况：%v", err)
	}

	fmt.Println("Redis 安装完成。")
	return nil
}

// configureRedis 配置 Redis，设置密码、允许外网访问以及自定义端口
func (rs *redisStruct) configureRedis() error {
	// 检查 redis.conf 是否存在
	if _, err := os.Stat(redisConfig); os.IsNotExist(err) {
		return fmt.Errorf("redis 配置文件不存在: %v", err)
	} else if err != nil {
		return fmt.Errorf("无法访问 redis 配置文件: %v", err)
	}

	// 备份原始配置文件: sudo cp /etc/redis/redis.conf /etc/redis/redis.conf.bck
	if err := rs.runCommand("sudo", "cp", redisConfig, backupConfig); err != nil {
		return fmt.Errorf("备份 Redis 配置文件失败：%v", err)
	}
	log.Println("备份原始配置文件完成 /etc/redis/redis.conf  >>> /etc/redis/redis.conf.bck")

	// 构建 sed 命令参数，对配置文件进行修改：
	// 1. 将 daemonize 设置为 yes，使 Redis 后台运行
	// 2. 修改 requirepass 为指定密码（同时支持注释或未注释的情况）
	// 3. 将 bind 修改为 0.0.0.0，允许外部访问
	// 4. 修改 port 为自定义端口
	sedCmd := []string{
		"-i",                                         // 修改文件
		"-e", "s/^daemonize\\s\\+no$/daemonize yes/", // 修改 daemonize 为 yes
		"-e", fmt.Sprintf("s|^#\\s*requirepass\\s\\+foobared$|requirepass %s|", rs.password), // 替换密码
		"-e", fmt.Sprintf("s/^\\s*bind\\s\\+.*::1.*$/bind %s ::1/", rs.address), // 替换 bind 地址
		redisConfig, // 配置文件路径
	}

	// 执行 sed 命令更新配置文件：sed -i -e 's/^daemonize\s\+no$/daemonize yes/' -e 's|^#\s*requirepass\s\+foobared$|requirepass Addlso*Z96@J!N4k#R1hjJ8v!11reach|' /etc/redis/redis.conf
	if err := rs.runCommand("sudo", append([]string{"sed"}, sedCmd...)...); err != nil {
		return fmt.Errorf("更新 Redis 配置失败: %v", err)
	}
	log.Println("更新 Redis 配置文件成功")

	return nil
}

// installRediSearch 安装 RediSearch 模块
func (rs *redisStruct) installRediSearch() error {
	fmt.Println("开始安装 RediSearch 模块...")

	// 安装必要依赖
	if err := rs.runCommand("sudo", "apt-get", "install", "-y", "build-essential", "tcl", "pkg-config", "libjemalloc-dev"); err != nil {
		return fmt.Errorf("安装 RediSearch 依赖失败：%v", err)
	}

	// 固定版本号，可根据需要调整或自动检测Redis版本后选择匹配版本
	rediSearchVersion := "2.10.20"

	// 下载 RediSearch 模块so文件
	downloadURL := fmt.Sprintf("https://github.com/RediSearch/RediSearch/releases/download/v%s/redisearch.so", rediSearchVersion)
	tmpPath := "/tmp/redisearch.so"

	fmt.Println("下载 RediSearch 模块...")
	curlCmd := []string{"curl", "-L", "-o", tmpPath, downloadURL}
	if err := rs.runCommand(curlCmd[0], curlCmd[1:]...); err != nil {
		return fmt.Errorf("下载 RediSearch 模块失败：%v", err)
	}

	// 确保 Redis 模块目录存在，不存在则创建
	moduleDir := "/etc/redis/modules"
	if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
		if err := rs.runCommand("sudo", "mkdir", "-p", moduleDir); err != nil {
			return fmt.Errorf("创建 Redis 模块目录失败：%v", err)
		}
	}

	// 移动so文件到模块目录
	destPath := moduleDir + "/redisearch.so"
	if err := rs.runCommand("sudo", "mv", tmpPath, destPath); err != nil {
		return fmt.Errorf("移动 RediSearch 模块失败：%v", err)
	}

	// 修改模块文件权限
	if err := rs.runCommand("sudo", "chmod", "644", destPath); err != nil {
		return fmt.Errorf("设置 RediSearch 模块权限失败：%v", err)
	}

	// 备份 Redis 配置文件，变量redisConfig应已定义为配置文件路径
	backupModuleConf := "/etc/redis/redis.conf.bck2"
	if err := rs.runCommand("sudo", "cp", redisConfig, backupModuleConf); err != nil {
		return fmt.Errorf("备份 redis.conf 失败：%v", err)
	}

	// 删除配置中已存在的 loadmodule redisearch.so 行，避免重复加载
	if err := rs.runCommand("sudo", "sed", "-i", "/^loadmodule .*redisearch.so$/d", redisConfig); err != nil {
		return fmt.Errorf("删除旧的 loadmodule 配置失败：%v", err)
	}

	// 追加 loadmodule 配置行到 redis.conf
	appendLoadModuleCmd := fmt.Sprintf("echo 'loadmodule %s' | sudo tee -a %s", destPath, redisConfig)
	if err := rs.runCommand("bash", "-c", appendLoadModuleCmd); err != nil {
		return fmt.Errorf("追加 loadmodule 配置失败：%v", err)
	}

	fmt.Println("RediSearch 模块安装并配置完成。请重启 Redis 服务以生效。")
	return nil
}

// enableRedisAutostart 设置 Redis 开机自启，并重启 Redis 服务使配置生效
func (rs *redisStruct) enableRedisAutostart() error {
	// 启用 Redis 服务的开机自启
	if err := rs.runCommand("sudo", "systemctl", "enable", "redis-server"); err != nil {
		return fmt.Errorf("设置 Redis 开机自启失败：%v", err)
	}

	// 重启 Redis 服务
	if err := rs.runCommand("sudo", "systemctl", "restart", "redis-server"); err != nil {
		return fmt.Errorf("启动 Redis 服务失败：%v", err)
	}
	if rs.address == "0.0.0.0" {
		fmt.Println("Redis 已设置为开机自启并成功启动（外网访问已开启）。")
	} else {
		fmt.Println("Redis 已设置为开机自启并成功启动（外网访问未开启）。")
	}
	return nil
}

// QueryPassword 查询当前 Redis 配置中的密码
func QueryPassword() (string, error) {
	// 使用 grep 查找配置文件中以 requirepass 开头的行
	out, err := exec.Command("grep", "^requirepass", redisConfig).Output()
	if err != nil {
		return "", fmt.Errorf("查询密码失败：%v", err)
	}

	// 输出格式示例：requirepass yourpassword
	fields := strings.Fields(string(out))
	if len(fields) < 2 {
		return "", fmt.Errorf("未在配置中找到密码设置")
	}
	return fields[1], nil
}

// SetupRedis 一键安装、配置并设置开机自启 Redis
// allowExternalNetworkAccess 为 true 时，绑定地址设置为 "0.0.0.0"，否则为 "127.0.0.1"
// port 为 Redis 监听端口（建议 6379 或其他自定义端口）
// password 为 Redis 访问密码
func SetupRedis(allowExternalNetworkAccess bool, password string) error {
	address := "127.0.0.1"
	if allowExternalNetworkAccess {
		address = "0.0.0.0"
	}

	if _, err := exec.LookPath("redis-server"); err == nil {
		rsPass, rsPassErr := QueryPassword()
		if rsPassErr != nil {
			return rsPassErr
		}
		fmt.Printf("检测到 Redis 已安装, 密码：%s\n", rsPass)
		return nil
	}

	rs := redisStruct{
		address:  address,
		password: password,
	}

	// 安装redis
	if err := rs.installRedis(); err != nil {
		return err
	}

	// 设置密码
	if err := rs.configureRedis(); err != nil {
		return err
	}

	// 开机自启
	if err := rs.enableRedisAutostart(); err != nil {
		return err
	}
	fmt.Println("Redis 安装与配置完成！")

	return nil
}
