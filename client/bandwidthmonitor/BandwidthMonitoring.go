package bandwidthmonitor

import (
	"fmt"
	"log"
	"net"
	"network-monitor/devicemonitor"
	"sort"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// BandwidthStats 带宽统计结构
type BandwidthStats struct {
	// 上行数据总量（字节）
	UploadBytes uint64
	// 下行数据总量（字节）
	DownloadBytes uint64
	// 上行数据包数量
	UploadPackets uint64
	// 下行数据包数量
	DownloadPackets uint64
	// 上次计算的上行速率（字节/秒）
	UploadSpeed float64
	// 上次计算的下行速率（字节/秒）
	DownloadSpeed float64
	// 上行平均包大小（字节）
	AvgUploadPacketSize float64
	// 下行平均包大小（字节）
	AvgDownloadPacketSize float64
	// 统计开始时间
	StartTime time.Time
	// 上次更新时间
	LastUpdate time.Time
	// 本地IP地址（用于判断上下行）
	LocalIPs []net.IP
	// 本地子网（用于更精确判断本地流量）
	LocalNetworks []*net.IPNet
	// 上行带宽历史数据（用于计算滑动平均）
	UploadHistory []float64
	// 下行带宽历史数据（用于计算滑动平均）
	DownloadHistory []float64
	// 历史窗口大小（记录多少个采样点）
	HistorySize int
	// 互斥锁（保护并发访问）
	mutex sync.Mutex
	// 过滤器集合（存储五元组以去重）
	flowTracker map[string]bool
}

// NewBandwidthStats 创建新的带宽统计对象
func NewBandwidthStats(historySize int) *BandwidthStats {
	if historySize <= 0 {
		historySize = 10 // 默认保存10个历史采样点
	}

	// 获取本地IP地址和子网
	localIPs, localNetworks, err := getLocalNetworks()
	if err != nil {
		log.Printf("获取本地网络信息失败: %v", err)
	}

	return &BandwidthStats{
		StartTime:       time.Now(),
		LastUpdate:      time.Now(),
		LocalIPs:        localIPs,
		LocalNetworks:   localNetworks,
		UploadHistory:   make([]float64, 0, historySize),
		DownloadHistory: make([]float64, 0, historySize),
		HistorySize:     historySize,
		flowTracker:     make(map[string]bool),
	}
}

// 获取本地IP地址和子网列表
func getLocalNetworks() ([]net.IP, []*net.IPNet, error) {
	var ips []net.IP
	var networks []*net.IPNet

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, nil, err
	}

	for _, iface := range interfaces {
		// 忽略禁用的接口和回环接口
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				// 确保这是一个有效的IP地址
				if v.IP.To4() != nil || v.IP.To16() != nil {
					ips = append(ips, v.IP)
					networks = append(networks, &net.IPNet{
						IP:   v.IP,
						Mask: v.Mask,
					})
				}
			case *net.IPAddr:
				ips = append(ips, v.IP)
				// 对于IPAddr类型，我们使用默认掩码
				mask := net.CIDRMask(24, 32) // IPv4
				if v.IP.To4() == nil {
					mask = net.CIDRMask(64, 128) // IPv6
				}
				networks = append(networks, &net.IPNet{
					IP:   v.IP,
					Mask: mask,
				})
			}
		}
	}

	// 输出调试信息
	for i, ip := range ips {
		if i < len(networks) {
			ones, _ := networks[i].Mask.Size()
			log.Printf("本地网络: %s/%d", ip.String(), ones)
		}
	}

	return ips, networks, nil
}

// 检查IP是否是本地IP
func (bs *BandwidthStats) isLocalIP(ip net.IP) bool {
	// 检查是否是本地IP
	for _, localIP := range bs.LocalIPs {
		if localIP.Equal(ip) {
			return true
		}
	}

	// 检查是否在本地子网内
	for _, network := range bs.LocalNetworks {
		if network.Contains(ip) {
			return true
		}
	}

	return false
}

// 生成五元组标识符
func (bs *BandwidthStats) generateFlowID(srcIP, dstIP string, srcPort, dstPort uint16, protocol string) string {
	// 根据源目IP、端口、协议生成唯一标识
	return fmt.Sprintf("%s:%d-%s:%d-%s", srcIP, srcPort, dstIP, dstPort, protocol)
}

// 更新带宽统计
func (bs *BandwidthStats) Update(packet gopacket.Packet, isUpload bool) {
	bs.mutex.Lock()
	defer bs.mutex.Unlock()

	// 使用更精确的包大小计算方法（获取捕获的实际大小）
	var packetSize uint64
	if metadata := packet.Metadata(); metadata != nil {
		packetSize = uint64(metadata.CaptureInfo.Length)
	} else {
		// 如果没有元数据，使用包数据大小
		packetSize = uint64(len(packet.Data()))
	}

	if isUpload {
		bs.UploadBytes += packetSize
		bs.UploadPackets++
	} else {
		bs.DownloadBytes += packetSize
		bs.DownloadPackets++
	}
}

// 计算带宽速率并更新滑动窗口
func (bs *BandwidthStats) CalculateSpeeds() {
	bs.mutex.Lock()
	defer bs.mutex.Unlock()

	now := time.Now()
	duration := now.Sub(bs.LastUpdate).Seconds()

	if duration > 0 {
		// 计算上行速率（字节/秒）
		currentUploadSpeed := float64(bs.UploadBytes) / duration
		// 计算下行速率（字节/秒）
		currentDownloadSpeed := float64(bs.DownloadBytes) / duration

		// 添加到历史记录
		bs.UploadHistory = append(bs.UploadHistory, currentUploadSpeed)
		bs.DownloadHistory = append(bs.DownloadHistory, currentDownloadSpeed)

		// 保持历史记录不超过指定大小
		if len(bs.UploadHistory) > bs.HistorySize {
			bs.UploadHistory = bs.UploadHistory[1:]
		}
		if len(bs.DownloadHistory) > bs.HistorySize {
			bs.DownloadHistory = bs.DownloadHistory[1:]
		}

		// 计算滑动平均值
		bs.UploadSpeed = calculateMovingAverage(bs.UploadHistory)
		bs.DownloadSpeed = calculateMovingAverage(bs.DownloadHistory)

		// 计算平均包大小
		if bs.UploadPackets > 0 {
			bs.AvgUploadPacketSize = float64(bs.UploadBytes) / float64(bs.UploadPackets)
		}
		if bs.DownloadPackets > 0 {
			bs.AvgDownloadPacketSize = float64(bs.DownloadBytes) / float64(bs.DownloadPackets)
		}

		// 重置计数器
		bs.UploadBytes = 0
		bs.DownloadBytes = 0
		bs.UploadPackets = 0
		bs.DownloadPackets = 0
		bs.LastUpdate = now

		// 每隔一段时间清空流量跟踪器以防止内存泄漏
		if len(bs.flowTracker) > 10000 || now.Sub(bs.StartTime).Minutes() > 5 {
			bs.flowTracker = make(map[string]bool)
			bs.StartTime = now
		}
	}
}

// 计算滑动平均值
func calculateMovingAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// 复制一份数据用于排序
	sortedValues := make([]float64, len(values))
	copy(sortedValues, values)
	sort.Float64s(sortedValues)

	// 移除可能的离群值（去掉最高和最低的10%的值，如果有的话）
	validValuesCount := len(sortedValues)
	if validValuesCount >= 10 {
		removeCount := validValuesCount / 10
		sortedValues = sortedValues[removeCount : validValuesCount-removeCount]
		validValuesCount = len(sortedValues)
	}

	// 计算平均值
	sum := 0.0
	for _, v := range sortedValues {
		sum += v
	}

	return sum / float64(validValuesCount)
}

// 格式化带宽值（自动选择单位：B/s, KB/s, MB/s, GB/s）
func formatBandwidth(bytesPerSec float64) string {
	// 将字节/秒转换为比特/秒 (1字节 = 8比特)
	bitsPerSec := bytesPerSec * 8

	units := []string{"bps", "Kbps", "Mbps", "Gbps"}
	unitIndex := 0
	value := bitsPerSec

	for value >= 1000 && unitIndex < len(units)-1 {
		value /= 1000
		unitIndex++
	}

	return fmt.Sprintf("%.2f %s", value, units[unitIndex])
}

// BandwidthMonitor 带宽监控器结构
type BandwidthMonitor struct {
	deviceMonitor *devicemonitor.DeviceMonitor
	stats         *BandwidthStats
	stopChan      chan struct{}
	interval      time.Duration // 统计间隔
	// 新增：使用BPF过滤器
	filter string
}

// NewBandwidthMonitor 创建新的带宽监控器
func NewBandwidthMonitor(deviceMonitor *devicemonitor.DeviceMonitor, interval time.Duration) *BandwidthMonitor {
	if interval <= 0 {
		interval = 500 * time.Millisecond // 默认0.5秒统计一次
	}

	return &BandwidthMonitor{
		deviceMonitor: deviceMonitor,
		stats:         NewBandwidthStats(int(time.Second/interval) * 2), // 保存2秒的历史数据
		stopChan:      make(chan struct{}),
		interval:      interval,
		// 默认不过滤任何数据包
		filter: "",
	}
}

// SetFilter 设置BPF过滤器
func (bm *BandwidthMonitor) SetFilter(filter string) error {
	handle := bm.deviceMonitor.GetHandle()
	if err := handle.SetBPFFilter(filter); err != nil {
		return fmt.Errorf("设置BPF过滤器失败: %v", err)
	}

	bm.filter = filter
	return nil
}

// Start 开始监控带宽
func (bm *BandwidthMonitor) Start() {
	handle := bm.deviceMonitor.GetHandle()

	// 设置过滤器去除ARP等链路层流量
	if bm.filter == "" {
		// 默认只捕获IPv4和IPv6流量
		bm.SetFilter("ip or ip6")
	}

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	packetSource.NoCopy = true

	// 设置解码器选项，指定需要解码的层
	packetSource.DecodeOptions.Lazy = false
	packetSource.DecodeOptions.NoCopy = true

	// 统计协程
	go func() {
		ticker := time.NewTicker(bm.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				bm.stats.CalculateSpeeds()

				// 打印带宽使用情况
				log.Printf("上行: %s (%.2f %s) | 下行: %s (%.2f %s)",
					formatBandwidth(bm.stats.UploadSpeed),
					bm.stats.AvgUploadPacketSize, "字节/包",
					formatBandwidth(bm.stats.DownloadSpeed),
					bm.stats.AvgDownloadPacketSize, "字节/包")

			case <-bm.stopChan:
				return
			}
		}
	}()

	// 数据包处理协程
	go func() {
		for packet := range packetSource.Packets() {
			// 跳过不完整的包
			if packet.ErrorLayer() != nil {
				continue
			}

			// 识别上下行方向
			isUpload := bm.isUploadPacket(packet)

			// 更新统计数据
			bm.stats.Update(packet, isUpload)
		}
	}()
}

// Stop 停止监控
func (bm *BandwidthMonitor) Stop() {
	close(bm.stopChan)
}

// isUploadPacket 判断数据包是上行还是下行
func (bm *BandwidthMonitor) isUploadPacket(packet gopacket.Packet) bool {
	// 获取网络层（IP层）
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		ipLayer = packet.Layer(layers.LayerTypeIPv6)
	}

	if ipLayer != nil {
		var srcIP, dstIP net.IP
		var srcPort, dstPort uint16
		var protocol string

		// 根据IP版本获取源IP和目的IP
		if ipv4, ok := ipLayer.(*layers.IPv4); ok {
			srcIP = ipv4.SrcIP
			dstIP = ipv4.DstIP
			protocol = "IPv4"
		} else if ipv6, ok := ipLayer.(*layers.IPv6); ok {
			srcIP = ipv6.SrcIP
			dstIP = ipv6.DstIP
			protocol = "IPv6"
		}

		// 获取传输层信息
		if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
			tcp, _ := tcpLayer.(*layers.TCP)
			srcPort = uint16(tcp.SrcPort)
			dstPort = uint16(tcp.DstPort)
			protocol += "/TCP"
		} else if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
			udp, _ := udpLayer.(*layers.UDP)
			srcPort = uint16(udp.SrcPort)
			dstPort = uint16(udp.DstPort)
			protocol += "/UDP"
		}

		// 生成五元组
		flowID := bm.stats.generateFlowID(srcIP.String(), dstIP.String(), srcPort, dstPort, protocol)
		// 记录流量（可选，目前仅用于防止内存泄漏）
		bm.stats.flowTracker[flowID] = true

		// 检查源IP和目的IP是否在本地网络
		srcIsLocal := bm.stats.isLocalIP(srcIP)
		dstIsLocal := bm.stats.isLocalIP(dstIP)

		// 如果源IP是本地IP且目的IP不是本地IP，则是上行流量
		if srcIsLocal && !dstIsLocal {
			return true
		}

		// 如果目的IP是本地IP且源IP不是本地IP，则是下行流量
		if !srcIsLocal && dstIsLocal {
			return false
		}

		// 对于内网流量，根据端口号判断
		// 常见服务端口通常是目的地为服务端
		if srcIsLocal && dstIsLocal {
			// 如果目的端口小于1024，通常是服务器端口，认为是上行
			return dstPort < 1024 && srcPort >= 1024
		}
	}

	// 默认情况，假设IP分类不明确的包是内部通信
	return true
}

// GetStats 获取当前带宽统计
func (bm *BandwidthMonitor) GetStats() *BandwidthStats {
	return bm.stats
}

// GetInterval 获取当前采样间隔
func (bm *BandwidthMonitor) GetInterval() time.Duration {
	return bm.interval
}

// SetInterval 设置采样间隔
func (bm *BandwidthMonitor) SetInterval(interval time.Duration) {
	if interval > 0 {
		bm.interval = interval
	}
}