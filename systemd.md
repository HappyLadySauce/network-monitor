

# systemd 配置

## Server 服务器端

创建程序目录

```bash
sudo mkdir /opt/network-monitor && cd /opt/network-monitor
```

通过 `wget` 下载程序

```bash
sudo wget https://github.com/HappyLadySauce/network-monitor/releases/download/v1.0.0/server \
     https://github.com/HappyLadySauce/network-monitor/releases/download/v1.0.0/config_server.yaml
```

给予可执行权限

```bash
sudo chmod +x server
```

创建 `/etc/systemd/system/network-monitor.service` 并写入 server 端配置

```shell
[Unit]
Description=network monitor Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/network-monitor
ExecStart=/opt/network-monitor/server
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

重启 `systemd`

```bash
systemctl daemon-reload
```

启动 **network-monitor** 服务器端

```bash
systemctl start network-monitor
```

设置开机自启

```bash
systemctl enable network-monitor
```

------

## Client 客户端

创建程序目录

```bash
sudo mkdir /opt/network-monitor && cd /opt/network-monitor
```

通过 `wget` 下载程序

```bash
sudo wget https://github.com/HappyLadySauce/network-monitor/releases/download/v1.0.0/client \
     https://github.com/HappyLadySauce/network-monitor/releases/download/v1.0.0/config_client.yaml
```

给予可执行权限

```bash
sudo chmod +x client
```

创建 `/etc/systemd/system/network-monitor.service` 并写入 client 端配置

```shell
[Unit]
Description=network monitor Client
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/network-monitor
ExecStart=/opt/network-monitor/client
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

重启 `systemd`

```bash
systemctl daemon-reload
```

启动 **network-monitor** 客户端

```bash
systemctl start network-monitor
```

设置开机自启

```bash
systemctl enable network-monitor
```