package devicemonitor

// 多层次网络连通性检测：
// 首先使用系统ping命令测试
// 如果ping失败，尝试TCP连接测试（连接到目标IP的53或80端口）
// 多个测试IP：
// 不仅测试8.8.8.8，还加入1.1.1.1（Cloudflare DNS）和114.114.114.114（中国电信DNS）
// 增加成功率，避免因为某个特定服务器不可用导致检测失败
// 适配多操作系统：
// 针对Windows、Linux和macOS分别使用不同的ping参数
// 解决了Windows上不能直接指定网络接口的问题
// 增强的超时处理：
// 设置ping命令执行超时，防止命令卡死
// 延长ping超时时间，提高检测成功率
// 详细日志：
// 添加了完整的网络接口信息输出
// 显示所有pcap设备详情
// 输出ping命令的完整结果
// 全面的匹配方式：
// 适配中英文系统的ping输出格式
// 添加多种成功标志检测：TTL、time=、bytes from等
// 备选方案：
// 如果指定设备ping失败，尝试不指定设备直接ping
// 如果能ping通，选择第一个非环回设备

import (
	"fmt"
	"log"
	"net"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/google/gopacket/pcap"
)

// 测试设备是否能ping通指定的IP地址
func canPingIP(device, ipAddr string) bool {
	// 首先尝试使用操作系统命令ping
	if pingUsingCommand(device, ipAddr) {
		return true
	}

	// 如果命令ping失败，尝试使用TCP连接测试
	return canConnectTcp(ipAddr)
}

// 使用系统ping命令测试连通性
func pingUsingCommand(device, ipAddr string) bool {
	// 构建ping命令
	var cmd *exec.Cmd
	var pingTimeout = "2" // 更长的超时时间，更可靠

	switch runtime.GOOS {
	case "windows":
		// Windows上无法直接指定网络接口，忽略device参数
		cmd = exec.Command("ping", "-n", "1", "-w", "2000", ipAddr)
	case "linux":
		// Linux上可以指定网络接口
		if device != "" && device != "any" {
			cmd = exec.Command("ping", "-c", "1", "-W", pingTimeout, "-I", device, ipAddr)
		} else {
			cmd = exec.Command("ping", "-c", "1", "-W", pingTimeout, ipAddr)
		}
	case "darwin": // macOS
		// macOS上的ping参数略有不同
		if device != "" && device != "any" {
			cmd = exec.Command("ping", "-c", "1", "-t", pingTimeout, "-S", device, ipAddr)
		} else {
			cmd = exec.Command("ping", "-c", "1", "-t", pingTimeout, ipAddr)
		}
	default:
		// 其他操作系统使用标准参数
		cmd = exec.Command("ping", "-c", "1", ipAddr)
	}

	// 给命令执行设置超时
	cmdChan := make(chan struct{})
	var output []byte
	var err error

	go func() {
		output, err = cmd.CombinedOutput()
		close(cmdChan)
	}()

	// 等待命令完成或超时
	select {
	case <-cmdChan:
		// 命令正常完成
	case <-time.After(3 * time.Second):
		// 超时处理
		log.Printf("设备 %s ping %s 超时\n", device, ipAddr)
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		return false
	}

	if err != nil {
		log.Printf("设备 %s ping %s 失败: %v\n", device, ipAddr, err)
		return false
	}

	// 检查输出是否包含成功的提示（支持多语言）
	outputStr := string(output)
	log.Printf("Ping输出: %s\n", outputStr)

	success := strings.Contains(outputStr, "bytes from") ||
		strings.Contains(outputStr, "字节，来自") ||
		strings.Contains(outputStr, "TTL=") ||
		strings.Contains(outputStr, "time=") ||
		strings.Contains(outputStr, "时间=") ||
		!strings.Contains(outputStr, "100% packet loss") &&
			!strings.Contains(outputStr, "100% 丢包率")

	if success {
		log.Printf("设备 %s 成功ping通 %s\n", device, ipAddr)
	} else {
		log.Printf("设备 %s ping %s 失败\n", device, ipAddr)
	}

	return success
}

// 使用TCP连接测试网络连通性作为备选方案
func canConnectTcp(ipAddr string) bool {
	// 尝试连接Google DNS服务器(8.8.8.8:53)或其他常用端口
	addr := fmt.Sprintf("%s:53", ipAddr)
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		log.Printf("TCP连接到%s失败: %v\n", addr, err)

		// 尝试HTTP端口
		addr = fmt.Sprintf("%s:80", ipAddr)
		conn, err = net.DialTimeout("tcp", addr, 2*time.Second)
		if err != nil {
			log.Printf("TCP连接到%s失败: %v\n", addr, err)
			return false
		}
	}
	defer conn.Close()
	log.Printf("成功建立TCP连接到%s\n", addr)
	return true
}

// 获取系统所有网络接口的信息
func getAllInterfaces() {
	ifaces, err := net.Interfaces()
	if err != nil {
		log.Printf("获取网络接口错误: %v\n", err)
		return
	}

	log.Println("系统网络接口列表:")
	for _, iface := range ifaces {
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		// 只显示有IP地址的接口
		if len(addrs) > 0 {
			log.Printf("接口: %s (索引: %d, MAC: %s, 状态: %v)\n",
				iface.Name, iface.Index, iface.HardwareAddr, iface.Flags)

			for _, addr := range addrs {
				log.Printf("  地址: %s\n", addr.String())
			}
		}
	}
}

// 获取默认路由出口网络设备
func defaultDevice() string {
	// 首先打印所有网络接口信息，便于调试
	getAllInterfaces()

	devices, err := pcap.FindAllDevs()
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("找到%d个pcap设备\n", len(devices))
	for i, d := range devices {
		log.Printf("设备[%d]: %s (%s)\n", i, d.Name, d.Description)
		for _, addr := range d.Addresses {
			log.Printf("  IP: %v, Netmask: %v\n", addr.IP, addr.Netmask)
		}
	}

	// 首先尝试找到能ping通Google DNS的设备
	testIPs := []string{"8.8.8.8", "1.1.1.1", "114.114.114.114"}

	for _, testIP := range testIPs {
		for _, device := range devices {
			if len(device.Addresses) > 0 && device.Name != "lo" && device.Name != "localhost" {
				log.Printf("测试设备 %s 是否能连接到 %s...\n", device.Name, testIP)
				if canPingIP(device.Name, testIP) {
					log.Printf("找到能连接互联网的设备: %s (%s)\n", device.Name, device.Description)
					return device.Name
				}
			}
		}
	}

	// 如果没有设备能ping通，尝试直接ping而不指定设备
	log.Printf("尝试不指定设备ping %s\n", "8.8.8.8")
	if canPingIP("", "8.8.8.8") {
		// 如果能ping通，选择第一个非环回设备
		for _, device := range devices {
			if len(device.Addresses) > 0 && device.Name != "lo" && device.Name != "localhost" {
				log.Printf("系统能连接互联网，选择第一个非环回设备: %s\n", device.Name)
				return device.Name
			}
		}
	}

	// 如果没有设备能ping通，退回到原来的选择逻辑
	log.Printf("没有设备能ping通测试IP，将基于其他条件选择设备")

	// 遍历所有网络设备
	for _, device := range devices {
		// 检查设备是否有IP地址
		if len(device.Addresses) > 0 {
			// 优先选择非回环设备
			if device.Name != "lo" && device.Name != "localhost" {
				log.Printf("选择网络设备: %s (%s)\n", device.Name, device.Description)
				return device.Name
			}
		}
	}

	// 如果没有找到合适的设备，返回第一个设备或提示错误
	if len(devices) > 0 {
		log.Printf("未找到理想网络设备，使用第一个可用设备: %s\n", devices[0].Name)
		return devices[0].Name
	}

	log.Fatal("未找到任何网络设备")
	return ""
}
