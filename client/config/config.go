package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Configuration struct {
	Server struct {
		Host          string        `mapstructure:"host"`
		Port          string        `mapstructure:"port"`
		MaxRetry      int           `mapstructure:"max_retry"`
		RetryInterval time.Duration `mapstructure:"retry_interval"`
	} `mapstructure:"server"`
	Client struct {
		ID    string `mapstructure:"id"`
		Alias string `mapstructure:"alias"`
	}
	Monitor struct {
		SampleInterval time.Duration `mapstructure:"sample_interval"`
		ReportInterval time.Duration `mapstructure:"report_interval"`
	} `mapstructure:"monitor"`
}

var Config Configuration

// 加载配置文件
func Init() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("config/")

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("读取配置文件失败: %v", err)
	}

	if err := viper.Unmarshal(&Config); err != nil {
		return fmt.Errorf("解析配置文件失败: %v", err)
	}

	// 设置默认值
	if Config.Monitor.SampleInterval == 0 {
		Config.Monitor.SampleInterval = 500 * time.Millisecond
	}
	if Config.Monitor.ReportInterval == 0 {
		Config.Monitor.ReportInterval = 1000 * time.Millisecond
	}
	if Config.Server.MaxRetry == 0 {
		Config.Server.MaxRetry = 5
	}
	if Config.Server.RetryInterval == 0 {
		Config.Server.RetryInterval = 5 * time.Minute
	}

	return nil
}
