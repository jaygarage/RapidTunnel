package redisclient

import (
	"RapidTunnel/utils/settings"
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	client    *redis.Client
	ctx       = context.Background()
	luaScript = redis.NewScript(`
math.randomseed(tonumber(redis.call('TIME')[2]))

local query = KEYS[1]
local current_time_s = tonumber(redis.call('TIME')[1])

-- ${now+x} 替换
local new_query = string.gsub(query, "%${now%+(%d+)}", function(mins)
    return tostring(current_time_s + (tonumber(mins) * 60))
end)

-- 默认过期过滤
if not string.find(new_query, "@expiration_time") then
    new_query = new_query .. ' @expiration_time:[' .. (current_time_s + 6) .. ' +inf]'
end

-- 获取数量
local quantity = redis.call(
    'FT.SEARCH',
    'proxy_pools_idx',
    new_query,
    'LIMIT',0,0
)[1]

if quantity == 0 then
    return nil
end

local attempts = 0

while attempts < 3 do
    local offset = math.random(0, quantity - 1)
    local result = redis.call(
        'FT.SEARCH',
        'proxy_pools_idx',
        new_query,
        'LIMIT', offset, 1,
        'RETURN', 4,
        'account',
        'password',
        'internet_ip',
        'port'
    )

	if result[1] ~= 0 and result[3] then
		local obj = {}
		for i = 1, #result[3], 2 do
			obj[result[3][i]] = result[3][i+1]
		end
		return cjson.encode(obj)
	end

    attempts = attempts + 1
end

return nil
	`)

	// 弃用过于复杂不考虑这些情况，让使用者重新访问
	//	luaScript = redis.NewScript(`
	//local query = KEYS[1]
	//
	//-- 获取当前时间戳（秒）
	//local current_time_s = tonumber(redis.call('TIME')[1])
	//
	//-- 替换 query 中的占位符，比如 ${now+5} -> (current_time_s + 5*60)
	//local new_query = string.gsub(query, "%${now%+(%d+)}", function(mins)
	//    return tostring(current_time_s + (tonumber(mins) * 60))
	//end)
	//
	//-- 如果没有 expiration_time 过滤规则，则走默认逻辑（最近 6 秒内）
	//if not string.find(new_query, "@expiration_time") then
	//    new_query = new_query .. ' @expiration_time:[' .. (current_time_s + 6) .. ' +inf]'
	//end
	//
	//-- 获取查询结果的数量
	//local quantity = redis.call('FT.SEARCH', 'proxy_pools_idx', new_query, 'LIMIT', 0, 0)[1]
	//if quantity == 0 then
	//    return nil
	//end
	//
	//local attempts = 0
	//local doc_fields_map = {}
	//local offset = math.random(0, quantity - 1)
	//
	//while attempts < 100 do
	//    local result = redis.call('FT.SEARCH', 'proxy_pools_idx', new_query, 'LIMIT', offset, 1)
	//
	//    -- 如果 result 为零，表示没有更多结果，直接返回 nil
	//    if result[1] == 0 then
	//		return nil
	//    end
	//
	//    -- 检查 result[3] 是否有值（即有返回的文档）
	//	local doc_fields = result[3]
	//    if doc_fields then
	//		for i = 1, #doc_fields, 2 do
	//		    local key = doc_fields[i]
	//		    local value = doc_fields[i + 1]
	//		    doc_fields_map[key] = value
	//		end
	//        return cjson.encode(doc_fields_map)
	//    end
	//
	//    -- 如果 result 为零，表示没有更多结果，直接返回 nil
	//    if result[1] == 0 then
	//		return nil
	//    end
	//
	//    -- 增加尝试次数
	//    attempts = attempts + 1
	//	offset = math.random(0, result[1] - 1)
	//end
	//
	//return nil
	//`)
)

// init 函数在包初始化时会自动运行
func init() {
	// 从配置中获取地址、密码和 DB 信息
	addr := fmt.Sprintf("%s:%d", settings.RedisAddr, settings.RedisPort)

	// 初始化 Redis 客户端
	client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: settings.RedisPassword,
		DB:       settings.RedisDB,

		// 连接池配置
		PoolSize:     512, // 推荐 CPU*20
		MinIdleConns: 32,

		// 超时控制
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolTimeout:  3 * time.Second,

		// 连接生命周期
		IdleTimeout:        60 * time.Second,
		MaxConnAge:         30 * time.Minute,
		IdleCheckFrequency: 30 * time.Second,

		// 重试策略
		MaxRetries:      2,
		MinRetryBackoff: 10 * time.Millisecond,
		MaxRetryBackoff: 200 * time.Millisecond,

		OnConnect: func(ctx context.Context, cn *redis.Conn) error {
			return nil
		},
	})
}

// Close 关闭 Redis 连接
func Close() error {
	return client.Close()
}

// Get Redis `GET key` 命令，获取指定 key 的值
// 如果 key 不存在，返回 redis.Nil 错误
func Get(key string) *redis.StringCmd {
	return client.Get(ctx, key)
}

// SRandMemberProxy 获取随机代理IP
func SRandMemberProxy(query string) (*string, error) {
	searchResults, err := luaScript.Run(ctx, client, []string{query}).Result()
	if searchResults == nil {
		return nil, fmt.Errorf("无法获取代理 -> %s", err)
	}

	if resultStr, ok := searchResults.(string); ok {
		return &resultStr, nil
	}
	return nil, fmt.Errorf("unexpected type, expected string but got %T", searchResults)
}
