package main

import (
	"bufio"
	"fmt"
	"net"
)

// 处理连接
func process(conn net.Conn) {
	// 关闭连接
	defer conn.Close()

	// 针对当前连接做发生和接受操作
	for {
		// 创建一个缓冲读取器
		// 直接从 net.Conn 读取数据时，每次调用 Read 方法都会触发一次系统调用
		// 读取数据到缓冲区,减少内存拷贝,提高效率
		// net.Conn 实现了 io.Reader 接口,所以可以作为参数传递给 bufio.NewReader
		reader := bufio.NewReader(conn)
		// 创建一个缓冲区
		// 128 是缓冲区的大小,如果读取的数据小于 128 字节,则不会触发系统调用
		// bufio.NewReader(conn) 是相对 conn 的缓冲读取器
		// buf 是相对于 bufio.NewReader(conn) 的缓冲区
		var buf [128]byte
		// 读取数据到缓冲区
		n, err := reader.Read(buf[:])
		if err != nil {
			fmt.Println("read from client failed, err:", err)
			break
		}
		// 打印读取到的数据
		fmt.Println("read from client:", string(buf[:n]))

		// 将接受到的数据返回给客户端
		_, err = conn.Write([]byte("ok"))
		if err != nil {
			fmt.Println("write to client failed, err:", err)
			break
		}
	}
}

func main() {
	// 监听端口
	listen, err := net.Listen("tcp", "127.0.0.1:20000")
	if err != nil {
		fmt.Printf("listen failed, err:%v\n", err)
		return
	}
	// 等待连接
	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Printf("accept failed, err:%v\n", err)
			continue
		}
		// 启动一个 goroutine 处理连接
		go process(conn)
	}
}
