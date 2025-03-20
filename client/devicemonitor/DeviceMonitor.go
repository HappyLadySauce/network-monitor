package devicemonitor

import (
	"log"

	"github.com/google/gopacket/pcap"
)

// 设备监控器
type DeviceMonitor struct {
	handle *pcap.Handle
	device string
}

// 创建一个设备监控器,如果device为空，使用默认出口设备
func NewDeviceMonitor(device string) *DeviceMonitor {
	// 如果device为空，使用默认出口设备
	if device == "" {
		device = defaultDevice()
	}
	handle, err := pcap.OpenLive(device, 65536, true, pcap.BlockForever)
	if err != nil {
		log.Fatal("打开网络设备失败:", err)
	}
	return &DeviceMonitor{
		handle: handle,
		device: device,
	}
}

// GetHandle 获取pcap句柄
func (m *DeviceMonitor) GetHandle() *pcap.Handle {
	return m.handle
}

// 关闭设备监控器
func (m *DeviceMonitor) Close() {
	if m.handle != nil {
		m.handle.Close()
		log.Printf("关闭对设备 %s 的监控\n", m.device)
	}
}
