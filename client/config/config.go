package config

import (
	"log"
	"time"

	"github.com/spf13/viper"
)

type config struct {
	Server struct {
		Host string `mapstructure:"host"`
		Port string `mapstructure:"port"`
	} `mapstructure:"server"`
	Monitor struct {
		SampleInterval time.Duration `mapstructure:"sample_interval"`
		ReportInterval time.Duration `mapstructure:"report_interval"`
	} `mapstructure:"monitor"`
}

var Config *config

// 加载配置文件
func InitConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("读取配置文件失败: %v", err)
	}

	var config config
	if err := viper.Unmarshal(&config); err != nil {
		log.Fatalf("解析配置文件失败: %v", err)
	}

	// 设置默认值
	if config.Monitor.SampleInterval == 0 {
		config.Monitor.SampleInterval = 500 * time.Millisecond
	}
	if config.Monitor.ReportInterval == 0 {
		config.Monitor.ReportInterval = 1000 * time.Millisecond
	}

	Config = &config
}
