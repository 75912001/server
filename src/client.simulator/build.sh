#!/bin/bash

# 获取当前脚本文件所在路径的绝对路径
currentPath=$(realpath "$(dirname "$0")")
echo "currentPath:${currentPath}"

# 模拟器目录
simulatorPath="${currentPath}"
echo "simulatorPath:${simulatorPath}"

# server 仓库根目录
serverPath=$(dirname "$(dirname "${simulatorPath}")")
echo "serverPath:${serverPath}"

cd "${serverPath}" || exit 1

# 编译客户端模拟器
go build -o "${simulatorPath}/bin/client.simulator.exe" ./src/client.simulator/main
if [ $? -ne 0 ]; then
    echo -e "\e[91m build client.simulator failed.\e[0m"
    exit 1
fi

echo -e "\e[92m build client.simulator successfully: ${simulatorPath}/bin/client.simulator.exe\e[0m"
