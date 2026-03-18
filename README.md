# 🌐 RapidTunnel - 高性能代理池系统
- 一个支持 高并发/多条件过滤/随机调度 的隧道代理 IP 池实现。

🚀 项目特性

⚡ 高并发代理获取（随机 / 条件）

🔍 多维度筛选（国家 / ISP / 城市 / 端口）

🎯 支持随机代理调度

🧠 可扩展索引（类似 MySQL）

🔄 支持百万 ~ 千万级数据扩展

## 🚀 RapidTunnel 快速开始

### 一、环境要求

- Go 版本：`go 1.26.1`
- Redis（隧道模式必需）

### 二、依赖初始化

```bash
Linux：
go mod init RapidTunnel && go mod tidy

Windows：
go mod init RapidTunnel; go mod tidy
```

### 三、编译项目
```
进入项目根目录执行：make build
编译完成后，进入 RapidTunnel/dist 目录，根据系统执行对应的可执行文件
```

### 四、代理类型

#### 1. 隧道模式

- 依赖 Redis，已集成 Ubuntu 内置脚本安装
- Ubuntu 可使用项目内脚本安装 Redis（其他系统需自行安装）参考修改：utils/install_redis/install_redis.go

#### 2.架构流程
![mermaid-diagram.png](doc/mermaid-diagram.png)
```
flowchart LR
Client[客户端] --> API[API服务]

API --> Filter[条件过滤]
API --> Random[随机获取]

Filter --> Redis[(Redis + RediSearch)]
Random --> Redis

Filter --> ES[(Elasticsearch)]
Random --> ES

Producer[代理采集] --> Redis
Producer --> ES

Redis --> Expire[过期检测]
ES --> Expire

Expire --> Cleanup[清理任务:自行实现]
```

### 五、📊 数据结构设计
#### 🔍 RedisSearch 索引
```FT.CREATE proxy_pools_idx ON HASH PREFIX 1 "proxy_pools:" SCHEMA
port TAG SORTABLE
source TAG SORTABLE
country TAG SORTABLE
region TAG SORTABLE
city TAG SORTABLE
isp TAG SORTABLE
other_info TAG SORTABLE
expiration_time NUMERIC SORTABLE
```

#### 🗑 删除索引：FT.DROPINDEX proxy_pools_idx

- 创建索引：FT.CREATE proxy_pools_idx ON HASH PREFIX 1 "proxy_pools:" SCHEMA port TAG SORTABLE source TAG SORTABLE country TAG
  SORTABLE region TAG SORTABLE city TAG SORTABLE isp TAG SORTABLE other_info TAG SORTABLE expiration_time NUMERIC SORTABLE

#### ⚙️ 可扩展点
- 与MySQL索引相似，自行调整索引并修改
- 查询逻辑：proxy/AgentPools.go
- proxy/AgentPools.go(代理获取)
- Redis Lua：utils/redisclient/redisclient.go(luaScript过滤查询条件)

#### 📦 代理结构：见proxy/types.go中的StructProxy 

```
type StructProxy struct {
	Account        string `json:"account"`
	Password       string `json:"password"`
	Source         string `json:"source"`
	MachineCode    string `json:"machine_code"`
	InternetIP     string `json:"internet_ip"`
	IntranetIP     string `json:"intranet_ip"`
	Port           string `json:"port"`
	Country        string `json:"country"`
	Region         string `json:"region"`
	City           string `json:"city"`
	ISP            string `json:"isp"`
	ExpirationTime string `json:"expiration_time"`
}
```

| 场景   | QPS     | 延迟  |
| ---- | ------- | --- |
| 随机获取 | 50,000+ | 2ms |
| 条件查询 | 30,000+ | 5ms |

| 场景   | QPS    | 延迟   |
| ---- | ------ | ---- |
| 条件查询 | 8,000+ | 15ms |
📈 结论

Redis：极致性能（推荐默认）

ES：海量数据 + 复杂查询

✅ 推荐：混合架构（热数据 Redis + 冷数据 ES）自行修改

# 六、📘 API 文档
```
type QueryParams struct {
    Port           string `json:"port"`                                     // 端口
    Source         string `json:"source"`                                   // 代理来源：aws、qgvps、aliyun等
    Country        string `json:"country"`                                  // 国家：cn等
    Region         string `json:"region"`                                   // 区域：广东省等
    City           string `json:"city"`                                     // 城市：深圳等
    Isp            string `json:"isp"`                                      // 运营商：
    ExpirationType string `json:"expiration_type" schema:"expiration_type"` // 过期范围时间规则：1-5，2-5，等时间区间拼接
}
```
```py
proxy_params = {
# "port":"",
# "source": "xxx",
# "country": "xxx",
# "region": "xxx",
# "city": "桂林市",
# "isp": "ctcc",
# "expiration_type": "1-3"
}
# 获取单个代理IP
resp = requests.get(ip, headers={
"Proxy-Authorization": "Basic 自己的账密"
}, params=proxy_params).json()

# 或者直接作为 （隧道 or 直连） 代理
proxy = f"http://test|{quote(urlencode(proxy_params))}:12345678@127.0.0.1"
proxies = {"http": proxy, "https": proxy}
r = requests.get("https://www.xxxxx", proxies=proxies, timeout=10)
print("状态码:", r.status_code)
```

---

### 系统信息

```bash
# 查看系统版本
lsb_release -a

# 查看系统架构
uname -m

# window 打包
$env:GOARCH="amd64";$env:GOOS="linux";go build -o RapidTunnel ./main.go
$env:GOARCH="arm64";$env:GOOS="linux";go build -o RapidTunnel ./main.go

# linux 打包
make build

# GOARCH的常见值
amd64: 64 位 x86 架构
386: 32 位 x86 架构
arm: 32 位 ARM 架构
arm64: 64 位 ARM 架构
ppc64: 64 位 PowerPC 架构
ppc64le: 64 位小端 PowerPC 架构
mips64: 64 位 MIPS 架构
mips64le: 64 位小端 MIPS 架构
s390x: 64 位 IBM z/Architecture
```

# 技术选型

#### 🟥 Redis + RediSearch
优点
高性能：Redis 是一个内存数据库，读写性能非常高，适合高并发访问场景。
简单高效：通过 RediSearch 可以实现简单的多条件过滤（如国家、ISP、端口等）。
轻量级：适合中等规模的数据量（例如几百万条记录）。Redis 提供了高性能和低延迟。
随机选择：可以利用 Redis 的 ZSET 或 List 数据结构随机选择一个代理。

缺点
内存消耗：Redis 是内存数据库，存储大量数据（几百万条代理）会消耗较多的内存。
水平扩展困难：虽然 Redis 支持集群，但当数据量达到极大时，可能会遇到内存瓶颈，水平扩展较为复杂。

适用场景
小到中等规模的数据量：如果代理池的数据量不大，且需要非常高的查询性能和高并发访问，Redis + RediSearch 是非常合适的选择。

推荐操作
使用 RediSearch 支持复杂查询（如按国家、ISP、端口等过滤）。

高并发查询时，可以使用 Redis Pipeline 批量获取代理 IP，提高效率。

#### 🟨 Elasticsearch
优点
分布式架构：Elasticsearch 是为分布式环境设计的，适合海量数据的场景，能够轻松扩展。
丰富的查询功能：支持更复杂的查询，条件过滤、聚合等非常强大。
随机查询支持：Elasticsearch 原生支持 random_score，可以基于某些字段或随机种子进行随机查询。

缺点
性能稍逊：与 Redis 相比，Elasticsearch 的写入性能稍低，尤其在高并发写入场景下。
硬件消耗高：Elasticsearch 需要更多的硬件资源（如内存、CPU、存储），尤其是在高可用和扩展性方面。

适用场景
大规模数据量：如果你的代理池数据量很大（上千万条记录）且需要支持复杂查询，Elasticsearch 更适合。


#### ⚖️ 对比

| **特性**    | **Redis + RediSearch** | **Elasticsearch**   |
|-----------|------------------------|---------------------|
| **写入性能**  | 极快（内存存储）               | 较慢（基于磁盘存储）          |
| **读取性能**  | 极快（内存存储）               | 稍逊，但支持大规模查询         |
| **查询复杂度** | 支持基本的多条件过滤             | 支持复杂查询、聚合和过滤        |
| **数据量**   | 适合小到中等规模的数据量（百万级）      | 适合大规模数据量（千万级以上）     |
| **水平扩展性** | 受限于内存，水平扩展较困难          | 本身设计为分布式，水平扩展非常容易   |
| **硬件要求**  | 较低，但需要足够的内存            | 高，需要较多的内存、CPU 和存储资源 |

缺点：
性能略逊： 相较于 Redis，Elasticsearch 在读写性能上稍微逊色，尤其是对于写入操作。
硬件成本： Elasticsearch 的硬件资源消耗相对较高，尤其是需要高性能存储和计算资源。

适用场景：
如果你的代理池数据量较大（上千万级），且需要支持 复杂查询 和 高并发查询，并且对分布式扩展有需求，Elasticsearch 更加合适。

#### 🎯 选型建议
| 场景        | 推荐    |
| --------- | ----- |
| 高并发 + 低延迟 | Redis |
| 海量数据      | ES    |
| 简单过滤      | Redis |
| 复杂查询      | ES    |

小到中等规模（百万条记录以内）的代理池，且对性能要求极高时，建议选用 Redis + RediSearch。它能提供极快的查询响应时间和高并发的支持，同时也能通过
RediSearch 做条件过滤。
如果需要支持海量数据和复杂的查询条件（千万条记录以上），且具备扩展需求，可以选择
Elasticsearch。它的分布式设计和强大的查询能力，特别适合海量数据场景下的代理池管理。

### ⭐ 如果对你有帮助 - 欢迎 Star & PR 🚀

