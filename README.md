# Network Monitor

Network Monitor 是一个用于监控远端服务器网络数据的工具，包含服务端和客户端两个组件。该工具使用 QUIC 协议进行通信，支持多客户端同时监控，并提供实时的网络带宽数据统计。

## 功能特性

### 客户端功能
- 实时监控网络带宽使用情况
- 支持上行/下行速率监控
- 计算平均数据包大小
- 自动重连机制
- 优雅的启动和关闭处理
- 支持配置采样间隔和上报间隔

### 服务端功能
- 使用 QUIC 协议提供高效的数据传输
- PostgreSQL 数据库存储监控数据
- 支持多客户端同时连接
- 自动清理过期数据（默认保留一周）
- 优雅的启动和关闭机制

## 项目结构
```
.
├── build                           # 构建输出目录
│   ├── client                      # 客户端构建输出
│   │   ├── client                  # 客户端可执行文件
│   │   └── config                  # 客户端配置目录
│   │       └── config.yaml         # 客户端配置文件
│   └── server                      # 服务端构建输出
│       ├── config                  # 服务端配置目录
│       │   └── config.yaml         # 服务端配置文件
│       └── server                  # 服务端可执行文件
├── build.sh                        # 项目构建脚本
├── client                          # 客户端源代码目录
│   ├── bandwidthmonitor           # 带宽监控模块
│   │   ├── BandwidthMonitoring.go # 带宽监控核心逻辑
│   │   └── Utils.go               # 带宽监控工具函数
│   ├── client                     # QUIC客户端模块
│   │   └── quic.go                # QUIC通信实现
│   ├── config                     # 客户端配置模块
│   │   ├── config.go              # 配置加载和管理
│   │   └── config.yaml            # 配置文件模板
│   ├── devicemonitor             # 设备监控模块
│   │   ├── DeviceMonitor.go      # 网络设备监控实现
│   │   └── Devices.go            # 设备管理工具函数
│   ├── go.mod                    # Go模块依赖定义
│   ├── go.sum                    # Go模块依赖校验
│   └── main.go                   # 客户端入口文件
├── README.md                     # 项目说明文档
└── server                        # 服务端源代码目录
    ├── config                    # 服务端配置模块
    │   ├── config.go             # 配置加载和管理
    │   └── config.yaml           # 配置文件模板
    ├── database                  # 数据库操作模块
    │   └── database.go           # 数据库连接和操作实现
    ├── go.mod                    # Go模块依赖定义
    ├── go.sum                    # Go模块依赖校验
    ├── main.go                   # 服务端入口文件
    └── server                    # QUIC服务器模块
        └── quic.go               # QUIC服务器实现
```

### 主要模块说明

#### 客户端模块
- **bandwidthmonitor**: 负责网络带宽监控的核心模块
  - 实时捕获网络数据包
  - 计算上下行带宽
  - 统计平均包大小
  - 处理数据采样和异常值过滤

- **devicemonitor**: 负责网络设备管理的模块
  - 获取网络接口信息
  - 设置数据包捕获
  - 管理网络设备状态
  - 处理设备异常

- **client/quic**: QUIC协议通信模块
  - 实现与服务器的QUIC连接
  - 管理数据传输
  - 处理连接重试和错误恢复

#### 服务端模块
- **database**: 数据持久化模块
  - 管理数据库连接
  - 处理数据存储和查询
  - 实现数据自动清理
  - 管理客户端信息

- **server/quic**: QUIC服务器实现
  - 处理客户端连接
  - 接收带宽数据
  - 管理客户端会话
  - 实现多客户端支持

## 源码安装说明

### 前置要求
- Go 1.18 或更高版本
- PostgreSQL 数据库
- libpcap-dev（用于网络数据包捕获）

### 安装步骤

1. 安装依赖：
```bash
# Ubuntu/Debian
sudo apt-get update
sudo apt-get install libpcap-dev postgresql postgresql-contrib

# CentOS/RHEL
sudo yum install libpcap-devel postgresql postgresql-server
```

2. 克隆代码：
```bash
git clone <repository_url>
cd network-monitoring
```

3. 编译项目：
```bash
./build.sh
```

### 启动服务端
```bash
cd build/server
./server
```

### 启动客户端
```bash
cd build/client
sudo ./client  # 需要root权限以捕获网络数据包
```

## 配置说明

### 客户端配置 (client/config/config.yaml)
```yaml
server:
  host: 127.0.0.1        # 服务器地址
  port: 8082             # 服务器端口
  max_retry: 5           # 最大重试次数
  retry_interval: 5m     # 重试间隔

client:
  id: "client_001"       # 客户端ID
  alias: "测试客户端"      # 客户端别名

monitor:
  sample_interval: 500ms # 采样间隔
  report_interval: 1s    # 上报间隔
```

### 服务端配置 (server/config/config.yaml)
```yaml
server:
  host: 0.0.0.0         # 监听地址
  port: 8082            # 监听端口

database:
  host: localhost       # 数据库地址
  port: 5432           # 数据库端口
  name: networkmonitor # 数据库名
  user: postgres       # 数据库用户
  password: postgres   # 数据库密码
```

## 系统服务配置

为了使程序能够作为系统服务运行，我们提供了systemd配置方案。

### 服务端配置

1. 创建程序目录并下载程序：
```bash
sudo mkdir /opt/network-moniter && cd /opt/network-moniter
sudo wget https://github.com/HappyLadySauce/network-monitor/releases/download/v1.0.0/server \
     https://github.com/HappyLadySauce/network-monitor/releases/download/v1.0.0/config_server.yaml
sudo chmod +x server
```

2. 创建systemd服务配置文件 `/etc/systemd/system/network-monitor.service`：
```ini
[Unit]
Description=network monitor Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/network-moniter
ExecStart=/opt/network-monitor/server
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

3. 启动服务：
```bash
systemctl daemon-reload
systemctl start network-monitor
systemctl enable network-monitor  # 设置开机自启
```

### 客户端配置

1. 创建程序目录并下载程序：
```bash
sudo mkdir /opt/network-moniter && cd /opt/network-moniter
sudo wget https://github.com/HappyLadySauce/network-monitor/releases/download/v1.0.0/client \
     https://github.com/HappyLadySauce/network-monitor/releases/download/v1.0.0/config_client.yaml
sudo chmod +x client
```

2. 创建systemd服务配置文件 `/etc/systemd/system/network-monitor.service`：
```ini
[Unit]
Description=network monitor Client
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/network-moniter
ExecStart=/opt/network-monitor/client
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

3. 启动服务：
```bash
systemctl daemon-reload
systemctl start network-monitor
systemctl enable network-monitor  # 设置开机自启
```

## 数据存储

服务器将自动创建以下数据表：
- `clients`: 存储客户端信息
- `bandwidth_stats`: 存储带宽统计数据

数据保留策略：
- 带宽数据保留时间为7天
- 超过7天的数据会自动清理

## 注意事项

1. 客户端需要以root权限运行以捕获网络数据包
2. 确保服务器和客户端的时间同步，以保证数据统计的准确性
3. 如果遇到权限问题，请检查数据库用户权限设置
4. 建议在生产环境中配置 SSL 证书

## 许可证

[License Name] - 详见 LICENSE 文件