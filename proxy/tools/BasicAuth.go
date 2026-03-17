/**
 * @author: Jay
 * @date: 2025/3/7
 * @file: BasicAuth.go
 * @description: 代理鉴权
 */

package tools

import (
	"RapidTunnel/utils/redisclient"
	"RapidTunnel/utils/settings"
	"fmt"
	"net/url"
	"strings"
)

var (
	redisUserKey = "proxy_auths:%s"
)

// BasicAuth 通过redis鉴权用户身份
func BasicAuth(user *url.Userinfo) bool {
	password, err := user.Password()
	if !err {
		return false
	}
	// 隧道代理账号密码判断
	if settings.TunneledOrNot {
		rk := fmt.Sprintf(redisUserKey, user.Username())

		passwordDecodedValue, passwordDecodedValueErr := url.QueryUnescape(strings.ReplaceAll(password, "+", "%2B"))
		if passwordDecodedValueErr != nil {
			return false
		}

		if redisclient.Get(rk).Val() != passwordDecodedValue {
			return false
		}
		return true
	}

	// 非隧道代理转发的判断
	if user.Username() != settings.ProxyUsername {
		return false
	}

	if password != settings.ProxyPassword {
		return false
	}

	return true
}
