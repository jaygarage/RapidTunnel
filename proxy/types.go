/**
 * @author: Jay
 * @date: 2025/7/30
 * @file: types.go
 * @description:
 */

package proxy

import (
	"RapidTunnel/utils/logrus"
	"encoding/json"
	"net/url"

	"github.com/gorilla/schema"
)

var decoder = schema.NewDecoder()

func init() {
	decoder.IgnoreUnknownKeys(true)
}

/*
QueryParams 代理查询条件
FT.CREATE proxy_pools_idx ON HASH PREFIX 1 "proxy_pools:" SCHEMA \
port TAG SORTABLE \
source TAG SORTABLE \
country TAG SORTABLE \
region TAG SORTABLE \
city TAG SORTABLE \
isp TAG SORTABLE \
other_info TAG SORTABLE \
expiration_time NUMERIC SORTABLE

FT.CREATE proxy_pools_idx ON HASH PREFIX 1 "proxy_pools:" SCHEMA port TAG SORTABLE source TAG SORTABLE country TAG SORTABLE region TAG SORTABLE city TAG SORTABLE isp TAG SORTABLE other_info TAG SORTABLE expiration_time NUMERIC SORTABLE

FT.DROPINDEX proxy_pools_idx
*/
type QueryParams struct {
	Port           string `json:"port"`                                     // 端口
	Source         string `json:"source"`                                   // 代理来源：aws、qgvps、aliyun等
	Country        string `json:"country"`                                  // 国家：cn等
	Region         string `json:"region"`                                   // 区域：广东省等
	City           string `json:"city"`                                     // 城市：深圳等
	Isp            string `json:"isp"`                                      // 运营商：
	ExpirationType string `json:"expiration_type" schema:"expiration_type"` // 过期范围时间规则：1-5，2-5，等时间区间拼接
}

func (q QueryParams) ToString() string {
	data, err := json.Marshal(q)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// StructProxy 查询结果
type StructProxy struct {
	Account  string `json:"account"`  // 账号
	Password string `json:"password"` // 密码
	//Source         string `json:"source"`          // 来源：快代理、AWS、VPS等
	//MachineCode    string `json:"machine_code"`    // 机器码，唯一标识
	InternetIP string `json:"internet_ip"` // 外网：一般都是 /ip 路由返回的
	//IntranetIP     string `json:"intranet_ip"`     // 内网：一般都是隧道走的内网
	Port string `json:"port"` // 端口
	//Country        string `json:"country"`         // 国家：香港hk、韩国kr
	//Region         string `json:"region"`          // 省份/州：广东省
	//City           string `json:"city"`            // 地区：如墨西哥、北京、首尔\市区、县城等
	//ISP            string `json:"isp"`             // 运营商：aws、aliyun、ctcc电信、cmcc移动、cucc联通等
	//ExpirationTime string `json:"expiration_time"` // 代理过期时间戳
}

func (s StructProxy) ToString() string {
	data, err := json.Marshal(s)
	if err != nil {
		return "{}"
	}
	return string(data)
}

// SetFieldFromQuery 表单:解析过滤条件
func SetFieldFromQuery(params url.Values) *QueryParams {
	qp := new(QueryParams)
	if err := decoder.Decode(qp, params); err != nil {
		logrus.Errorf("解析表单失败：%v", err)
		return nil
	}

	if len(params) == 0 {
		return nil
	}

	return qp
}
