package config

import (
	"log"

	"github.com/spf13/viper"
)

type config struct {
	Server struct {
		Port string `mapstructure:"port"`
	} `mapstructure:"server"`
	Database struct {
		Host     string `mapstructure:"host"`
		Port     string `mapstructure:"port"`
		User     string `mapstructure:"user"`
		Password string `mapstructure:"password"`
		Name     string `mapstructure:"name"`
	} `mapstructure:"database"`
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

	Config = &config
}
