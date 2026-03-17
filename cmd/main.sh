#!/usr/bin/env bash

USER_NAME="${1:-jay}"
PASSWORD="${2:-16888<jyck599\$+Jay->88861}"
TUNNEL_PROXY_PATH="/home/RapidTunnel"

detectionExpect(){
  if dpkg -s expect >/dev/null 2>&1; then
      echo "Expect 已安装，继续执行..."
  else
      echo "Expect 未安装，正在安装..."
      sudo apt-get update
      sudo apt-get install -y expect
  fi
}


# 定义一个函数来执行 expect 脚本
run_expect() {
    expect <<EOF
        # 设置无限等待（可以根据需要设置超时）
        set timeout -1

        # 启动 RapidTunnel 程序（确保路径正确）
        spawn $TUNNEL_PROXY_PATH

        # 模拟交互并发送输入
        expect "请输入监听地址 (默认0.0.0.0):"
        send "0.0.0.0\r"

        expect "请输入监听端口号 (默认80):"
        send "16888\r"

        expect "是否启用隧道转发 (默认no, 输入yes/no):"
        send "no\r"

        expect "请输入代理账号 (默认test):"
        send -- "$USER_NAME\r"

        expect "请输入代理密码 (默认test123456TEST):"
        send -- "$PASSWORD\r"

        # 等待程序结束
        expect eof
EOF
}

main(){
  chmod +x $TUNNEL_PROXY_PATH
  while true
  do
      echo "正在启动 RapidTunnel..."
	    sudo lsof -t -i :16888 | xargs -r sudo kill -9

      # 执行 expect 脚本
      run_expect

      # 打印一条日志，表示脚本完成了（不管是否出错）
      echo "RapidTunnel 执行结束，重新启动..."
  done
}

# 检查expect
detectionExpect
main

# 清空并创建任务
# crontab -r && echo '* * * * * bash -c '\''screen -ls | grep -q "RapidTunnel_session" || screen -dmS RapidTunnel_session /home/test.sh jay "16888<jyck599\$+Jay->88861"'\''' | crontab -
