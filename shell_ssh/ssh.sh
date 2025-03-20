#!/bin/bash

# 配置文件路径
CONFIG_FILE="/home/config.json"

# 获取命令行参数，默认为空
ALIAS_FLAG=$1

# 如果没有传入别名参数，进入交互式选择
select_server() {
  echo "Please select a server to connect to:"
  jq -r '.servers[] | "\(.alias) (\(.address))"' "$CONFIG_FILE"

  read -p "Enter the alias or address of the server: " CHOICE

  # 从配置文件中获取选择的服务器信息
  SERVER=$(jq -r ".servers[] | select(.alias == \"$CHOICE\" or .address == \"$CHOICE\")" "$CONFIG_FILE")

  if [ -z "$SERVER" ]; then
    echo "Invalid choice, exiting."
    exit 1
  fi
}

# 如果指定了服务器别名，通过命令行参数直接选择
if [ -n "$ALIAS_FLAG" ]; then
  SERVER=$(jq -r ".servers[] | select(.alias == \"$ALIAS_FLAG\")" "$CONFIG_FILE")
  if [ -z "$SERVER" ]; then
    echo "No server found with alias \"$ALIAS_FLAG\", exiting."
    exit 1
  fi
else
  select_server
fi

# 从服务器配置中提取信息
ALIAS=$(echo "$SERVER" | jq -r '.alias')
ADDRESS=$(echo "$SERVER" | jq -r '.address')
PORT=$(echo "$SERVER" | jq -r '.port')
USER=$(echo "$SERVER" | jq -r '.user')
USE_KEY=$(echo "$SERVER" | jq -r '.use_key')
PRIVATE_KEY=$(echo "$SERVER" | jq -r '.private_key')
PASSWORD=$(echo "$SERVER" | jq -r '.password')

# 设置 SSH 连接字符串
SSH_CMD="ssh $USER@$ADDRESS -p $PORT"

# 如果使用密钥认证，添加 `-i` 参数
if [ "$USE_KEY" == "true" ]; then
  HOME_DIR=$(eval echo ~${USER})
  KEY_PATH="${HOME_DIR}/${PRIVATE_KEY}"
  SSH_CMD="$SSH_CMD -i $KEY_PATH"
fi

# 如果有密码，使用 `sshpass` 工具来自动输入密码
if [ -n "$PASSWORD" ]; then
  SSH_CMD="sshpass -p $PASSWORD $SSH_CMD"
fi

# 打印连接信息并连接
echo "Connecting to $ALIAS ($ADDRESS:$PORT)..."
$SSH_CMD

# 如果 SSH 连接失败，提示错误信息
if [ $? -ne 0 ]; then
  echo "Failed to connect to server $ALIAS, exiting."
  exit 1
fi
