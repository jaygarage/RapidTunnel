/**
 * @author: Jay
 * @date: 2025/3/10
 * @file: AgentPools.go
 * @description: IP 代理池
 */

package proxy

import (
	"RapidTunnel/utils/redisclient"
	"encoding/json"
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"

	"github.com/asaskevich/govalidator"
)

// ISPMap 运营商
var ISPMap = map[string]string{
	"cmcc":    "移动",
	"ctcc":    "电信",
	"cucc":    "联通",
	"aliyun":  "阿里云",
	"tencent": "腾讯云",
	"aws":     "AWS",
}

// 生成查询字符串的方法（使用反射）
func generateQueryFromParams(qp *QueryParams) string {
	var queryParts []string

	// 反射遍历 QueryParams 结构体字段
	val := reflect.ValueOf(qp).Elem()
	typ := val.Type()

	// 遍历每个字段并根据字段的值生成查询条件
	for i := 0; i < val.NumField(); i++ {
		fieldValue := val.Field(i).String()
		fieldName := typ.Field(i).Tag.Get("json") // 获取 json 标签名

		if fieldValue == "" || fieldName == "" {
			continue
		}

		if fieldName == "expiration_type" {
			// ⚠️ 这里仅拼接规则字符串，真正时间戳替换还是在 Lua 里做
			if strings.Contains(fieldValue, "-") {
				parts := strings.Split(fieldValue, "-")
				if len(parts) == 2 {
					startInt, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
					endInt, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
					if startInt > endInt {
						startInt, endInt = endInt, startInt
					}
					// 拼成一个占位符，Lua 里再替换 current_time_s
					queryParts = append(queryParts, fmt.Sprintf("@expiration_time:[${now+%d} ${now+%d}]", startInt, endInt))
					continue
				}
			}
		}
		queryParts = append(queryParts, fmt.Sprintf("@%s:{%s}", fieldName, strings.ToLower(fieldValue)))
	}

	// 拼接为查询字符串，多个条件用空格分隔
	return strings.Join(queryParts, " ")
}

// GetProxy socks5、http
func GetProxy(protocol *string, params *QueryParams) (*url.URL, error, *StructProxy) {
	query := ""
	if params != nil {
		query = generateQueryFromParams(params)
	}
	proxy, err := redisclient.SRandMemberProxy(query)

	if err != nil {
		if params == nil {
			return nil, fmt.Errorf("代理为空 %v", proxy), nil
		}
		return nil, fmt.Errorf("代理为空 %s %v", params.ToString(), proxy), nil
	}

	// 反序列化到 Proxy 结构体
	var sp StructProxy
	err = json.Unmarshal([]byte(strings.TrimSpace(*proxy)), &sp)
	if err != nil {
		return nil, fmt.Errorf("反序列化失败 %v %v", *proxy, err), nil
	}

	var (
		proxyURL *url.URL
		parseErr error
	)

	// /ip 代理的
	if protocol == nil {
		//proxyURL, parseErr = url.Parse(fmt.Sprintf("%s:%s@%s:%s", ps.Account, ps.Password, ps.IntranetIP, ps.Port))
		return nil, parseErr, &sp
	}

	proxyURL, parseErr = url.Parse(fmt.Sprintf("%s://%s:%s@%s:%s", *protocol, sp.Account, sp.Password, sp.InternetIP, sp.Port))
	//proxyURL, _ = url.Parse(fmt.Sprintf("%s://Q8wBTL:cDjifiEr3Ao9Ve7vquKhIrkzKFRl4eCB@18.163.210.228:16888", protocol))
	//proxyURL, parseErr = url.Parse("https://test:test123456TEST@47.119.153.252:80")
	if parseErr != nil {
		return nil, parseErr, nil
	}

	if !govalidator.IsURL(proxyURL.String()) {
		return nil, fmt.Errorf("代理异常，无法进行格式化 %s", sp.ToString()), nil
	}

	return proxyURL, nil, nil
}
