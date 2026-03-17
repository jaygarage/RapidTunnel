#!/bin/bash

# 判断系统类型，设置包管理器
detect_system() {
  if [ -f /etc/redhat-release ]; then
    PKG_MANAGER="yum"
  elif [ -f /etc/debian_version ]; then
    PKG_MANAGER="apt"
  else
    echo "不支持的系统类型，请使用基于Debian或RedHat的系统。"
    exit 1
  fi
}

# 判断Squid是否已安装（兼容Debian和RedHat）
is_squid_installed() {
  if [ "$PKG_MANAGER" = "yum" ]; then
    if command -v rpm &>/dev/null && rpm -q squid &>/dev/null; then
      return 0 # yum系检测到已安装
    else
      return 1 # yum系未安装
    fi
  else
    if command -v dpkg &>/dev/null && dpkg -l | grep -q squid; then
      return 0 # apt系检测到已安装
    else
      return 1 # apt系未安装
    fi
  fi
}

# 卸载Squid（如已安装）
uninstall_squid() {
  if is_squid_installed; then
    echo "检测到Squid已安装，执行卸载清理..."

    sudo systemctl stop squid 2>/dev/null
    sudo systemctl disable squid 2>/dev/null

    if [ "$PKG_MANAGER" = "yum" ]; then
      if ! sudo yum remove -y squid; then
        echo "❌ Squid卸载失败，请检查！" >&2
        return 1
      fi
    else
      if ! sudo apt remove --purge -y squid; then
        echo "❌ Squid卸载失败，请检查！" >&2
        return 1
      fi
      sudo apt autoremove -y
    fi

    sudo rm -rf /etc/squid /var/spool/squid
    echo "✅ Squid已卸载并清理完成。"
  else
    echo "🔎 Squid未检测到，无需卸载。"
  fi
}

# 安装Squid及认证工具
install_squid() {
  echo "更新系统并安装 Squid 代理服务器..."
  if [ "$PKG_MANAGER" = "yum" ]; then
    sudo yum install -y squid httpd-tools
  else
    sudo apt update
    sudo apt install -y squid apache2-utils
  fi
}

# 设置认证用户
set_auth_user() {
  echo "设置账号密码 (账号:jay, 密码:16888<jyck599$+Jay->88861"
  sudo htpasswd -bc /etc/squid/passwd jay "16888<jyck599$+Jay->88861"
}

# 生成Squid配置文件
generate_squid_config() {
  cpu_cores=$(($(nproc) * 3))
  NCSA_AUTH_PATH=$(find /usr/lib* -name basic_ncsa_auth 2>/dev/null | head -n 1)
  if [ -z "$NCSA_AUTH_PATH" ]; then
    echo "错误：未找到basic_ncsa_auth模块，请确认Squid和apache2-utils/httpd-tools是否已正确安装。"
    exit 1
  fi
  echo "认证模块路径：$NCSA_AUTH_PATH"

  sudo bash -c "cat << EOF > /etc/squid/squid.conf
# =======================
# Squid代理服务器配置
# 生成时间: $(date "+%Y-%m-%d %H:%M:%S")
# =======================

# =======================
# 网络ACL定义（允许哪些网段访问）
# =======================
acl localnet src 10.0.0.0/8	# RFC1918 possible internal network
acl localnet src 172.16.0.0/12	# RFC1918 possible internal network
acl localnet src 192.168.0.0/16	# RFC1918 possible internal network
acl localnet src fc00::/7       # RFC 4193 local private network range
acl localnet src fe80::/10      # RFC 4291 link-local (directly plugged) machines

# =======================
# 端口ACL定义（允许哪些端口）
# =======================
acl SSL_ports port 443
acl Safe_ports port 80		# http
acl Safe_ports port 21		# ftp
acl Safe_ports port 443		# https
acl Safe_ports port 210		# wais
acl Safe_ports port 1025-65535	# unregistered ports
acl Safe_ports port 280		# http-mgmt
acl Safe_ports port 488		# gss-http
acl Safe_ports port 591		# filemaker
acl Safe_ports port 777		# multiling http
acl CONNECT method CONNECT

# =======================
# 禁止访问非安全端口（Access Control Rules）
# =======================
# Deny requests to certain unsafe ports
http_access deny !Safe_ports

# Deny CONNECT to other than secure SSL ports
http_access deny CONNECT !SSL_ports

# Only allow cachemgr access from localhost
http_access allow localhost manager
http_access deny manager

# =======================
# 身份鉴权配置
# =======================
auth_param basic program $NCSA_AUTH_PATH /etc/squid/passwd
auth_param basic realm \"Proxy Authentication\"  # 身份认证提示信息
auth_param basic credentialsttl 2 hours        # 身份认证有效期2小时
auth_param basic casesensitive off              # 用户名不区分大小写

# 认证用户ACL
acl authenticated proxy_auth REQUIRED
http_access allow authenticated

# 禁止所有其他请求
http_access deny all

# 监听端口配置
http_port 16888

# 设置请求超时时间
request_timeout 60 seconds

# 设置连接超时时间
connect_timeout 60 seconds

# 设置读取数据超时时间
read_timeout 60 seconds

# =======================
# CoreDump配置（故障转储目录）
# =======================
coredump_dir /var/spool/squid

# =======================
# 缓存刷新规则（Cache Refresh Patterns）
# =======================
refresh_pattern ^ftp:                   1440    20%     10080
refresh_pattern -i (/cgi-bin/|\?)       0       0%      0
refresh_pattern \/(Packages|Sources)(|\.bz2|\.gz|\.xz)$ 0 0% 0 refresh-ims
refresh_pattern \/Release(|\.gpg)$      0       0%      0 refresh-ims
refresh_pattern \/InRelease$            0       0%      0 refresh-ims
refresh_pattern \/(Translation-.*)(|\.bz2|\.gz|\.xz)$ 0 0% 0 refresh-ims
refresh_pattern .                       0       20%     4320

# =======================
# 代理身份与隐私（高匿名设置）
# =======================
via off                                 # 关闭Via头信息
httpd_suppress_version_string on        # 隐藏Squid版本信息
forwarded_for delete                    # 删除X-Forwarded-For头，防止泄露客户端IP

# =======================
# 高并发配置
# =======================
max_filedescriptors 1024              # 增加文件描述符限制，避免高并发连接失败
cache_mem 512 MB                      # 设置内存缓存大小（根据服务器内存调整）
maximum_object_size 128 MB            # 设置缓存中最大对象的大小
memory_pools off                      # 关闭内存池，优化高并发性能

# 工作进程优化
workers $cpu_cores
EOF"
}

# 启动并设置自启
start_and_enable_squid() {
  echo "设置开机自启，并重启Squid服务..."
  sudo systemctl enable squid
  sudo systemctl restart squid
}

# 查看Squid状态
show_squid_status() {
  echo "查看Squid服务状态"
  sudo systemctl status squid --no-pager
}

# 主流程
main() {
  detect_system
  uninstall_squid
  install_squid
  set_auth_user
  generate_squid_config
  start_and_enable_squid
  show_squid_status
  echo "Squid 代理服务器安装和配置完成！"
}
main

# 安装 dos2unix 工具来转换文件格式
#sudo apt install dos2unix
#dos2unix install_squid.sh
