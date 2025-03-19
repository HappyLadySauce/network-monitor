package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
)

func main() {
	// 连接服务器
	conn, err := net.Dial("tcp", "127.0.0.1:20000")
	if err != nil {
		fmt.Println("dial failed, err:", err)
	}
	// 关闭连接
	defer conn.Close()

	inputReader := bufio.NewReader(os.Stdin)
	for {
		// 读取用户输入
		s, _ := inputReader.ReadString('\n')
		// 去除字符串两端的空白字符
		s = strings.TrimSpace(s)
		// 如果输入 q 则退出
		if strings.ToUpper(s) == "Q" {
			return
		}

		// 发送数据
		_, err = conn.Write([]byte(s))
		if err != nil {
			fmt.Println("write to server failed, err:", err)
			return
		}

		// 读取服务器返回的数据
		var buf [1024]byte
		// 读取数据到缓冲区
		n, err := conn.Read(buf[:])
		if err != nil {
			fmt.Println("read from server failed, err:", err)
			return
		}
		// 打印读取到的数据
		fmt.Println("read from server:", string(buf[:n]))
	}
}