/**
 * @author: Jay
 * @date: 2025/3/12
 * @file: auth.go
 * @description:
 */

package socks

import (
	"RapidTunnel/proxy"
	"RapidTunnel/proxy/tools"
	"bufio"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
)

type UserAuthentication struct {
	conn   net.Conn
	reader *bufio.Reader
	params *proxy.QueryParams
}

// 读取验证方式
func (ua *UserAuthentication) readMethods() ([]byte, error) {
	//认证头
	header, _, err := ua.reader.ReadRune()
	if err != nil {
		return nil, err
	}

	// 认证类型
	methods := make([]byte, int(header))
	_, err = ua.reader.Read(methods)
	return methods, err
}

// Validation 实现了基本的账号密码认证流程。
func (ua *UserAuthentication) Validation() (bool, error) {
	methods, methodsErr := ua.readMethods()
	if methodsErr != nil {
		return false, methodsErr
	}
	//logrus.Infof("methods %v %d", methods, len(methods))

	// 仅接受 账号&&密码 验证
	if !func() bool {
		for _, m := range methods {
			if m == UsernamePasswordMethod {
				return true
			}
		}
		return false
	}() {
		//logrus.Warnf("客户端不支持基本认证方法, methods=%v", methods)
		return false, fmt.Errorf("客户端不支持基本认证方法")
	}

	//logrus.Info("验证方式存在开始验证.")

	// 发送认证方法选择应答，选择基本认证方法
	// 应答格式：第一个字节为 SOCKS 版本号(通常为 0x05)，第二个字节为选择的认证方法（0x02表示基本认证）
	if _, err := ua.conn.Write([]byte{socksVersion5, UsernamePasswordMethod}); err != nil {
		//logrus.Warn(err.Error())
		return false, err
	}

	// 按照 SOCKS5 用户名密码认证协议，客户端将发送以下格式的数据：
	// +-----+------+----------+------+----------+
	// | VER | ULEN |  UNAME   | PLEN |  PASSWD  |
	// +-----+------+----------+------+----------+
	// |  1  |  1   | 1 to 255 |  1   | 1 to 255 |
	// +-----+------+----------+------+----------+
	// 服务端返回数据格式：
	// +-----+--------+
	// | VER | STATUS |
	// +-----+--------+
	// |  1  |   1    |
	// +-----+--------+
	// 读取认证请求版本号（期望为 0x01）
	version := make([]byte, 1)
	if _, err := io.ReadFull(ua.conn, version); err != nil {
		return false, err
	}
	//logrus.Infof("version：%v", version)

	if version[0] != 0x01 {
		return false, fmt.Errorf("不支持的认证协议版本: %d", version[0])
	}

	user := ua.User()
	//logrus.Info(user.Username())
	//logrus.Info(user.Password())

	// 账号密码校验
	if !tools.BasicAuth(user) {
		// 认证失败，返回失败应答：版本号 0x01，状态码 0x01（非0表示失败）
		return false, fmt.Errorf("认证失败: 用户名或密码错误")
	}

	// 认证成功，发送认证成功应答：版本号 0x01，状态码 0x00（0 表示成功）
	if _, err := ua.conn.Write([]byte{CommandConnect, StatusSuccess}); err != nil {
		return false, err
	}

	//logrus.Info("认证成功...")
	return true, nil
}

func (ua *UserAuthentication) User() *url.Userinfo {
	username, err := ua.user()
	if err != nil {
		return &url.Userinfo{}
	}

	// 再用 | 分割用户名和参数
	up := strings.SplitN(username, "|", 2)
	//logrus.Info("客户端提交的用户名：", up[0])

	paramsStr := ""
	if len(up) > 1 {
		paramsStr = up[1]
	}
	params, err := url.ParseQuery(paramsStr)
	if err != nil {
		return &url.Userinfo{}
	}
	ua.params = proxy.SetFieldFromQuery(params)
	//logrus.Info("客户端提交的表单：", params)

	password, err := ua.user()
	if err != nil {
		return &url.Userinfo{}
	}
	//logrus.Info("客户端提交的密码：", password)

	return url.UserPassword(up[0], password)
}

func (ua *UserAuthentication) user() (string, error) {
	// 读取用户名长度
	ul := make([]byte, 1)
	if _, err := ua.conn.Read(ul); err != nil {
		return "", err
	}

	// 根据用户名长度读取用户名
	us := make([]byte, ul[0])
	if _, err := ua.conn.Read(us); err != nil {
		return "", err
	}

	return string(us), nil
}

func NewUserAuthentication(conn net.Conn, reader *bufio.Reader) *UserAuthentication {
	return &UserAuthentication{conn: conn, reader: reader}
}
