#!/bin/bash

# 设置版本号
VERSION="1.0.0"

# 设置输出目录
OUTPUT_DIR="build"
rm -rf "${OUTPUT_DIR}"
mkdir -p "${OUTPUT_DIR}"

# 创建目录结构
mkdir -p "${OUTPUT_DIR}/client/config"
mkdir -p "${OUTPUT_DIR}/server/config"

# 构建client
echo "正在构建client..."
cd client
go build -o "../${OUTPUT_DIR}/client/client" main.go
if [ $? -ne 0 ]; then
    echo "client构建失败"
    exit 1
fi
cd ..

# 复制client配置文件
cp client/config/config.yaml "${OUTPUT_DIR}/client/config/"

# 构建server
echo "正在构建server..."
cd server
go build -o "../${OUTPUT_DIR}/server/server" main.go
if [ $? -ne 0 ]; then
    echo "server构建失败"
    exit 1
fi
cd ..

# 复制server配置文件
cp server/config/config.yaml "${OUTPUT_DIR}/server/config/"

echo "构建完成！"
echo "可执行文件和配置文件已放置在 ${OUTPUT_DIR} 目录下"
echo "目录结构："
tree "${OUTPUT_DIR}" 