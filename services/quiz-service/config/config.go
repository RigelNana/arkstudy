package config

import (
	"fmt"
	"log"

	"github.com/spf13/viper"
)

type Config struct {
	Database   DatabaseConfig   `mapstructure:"database"`
	OpenAI     OpenAIConfig     `mapstructure:"openai"`
	GRPC       GRPCConfig       `mapstructure:"grpc"`
	LLMService LLMServiceConfig `mapstructure:"llm_service"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

type OpenAIConfig struct {
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
	BaseURL string `mapstructure:"base_url"`
}

type GRPCConfig struct {
	Port string `mapstructure:"port"`
}

type LLMServiceConfig struct {
	Address string `mapstructure:"address"`
}

func LoadConfig() (*Config, error) {
	config := &Config{}

	// 设置默认值
	viper.SetDefault("grpc.port", "50055")
	viper.SetDefault("database.sslmode", "disable")
	viper.SetDefault("openai.model", "gpt-3.5-turbo")
	viper.SetDefault("llm_service.address", "arkstudy-llm-service:50054")

	// 从环境变量读取配置
	viper.AutomaticEnv()

	// 绑定环境变量
	viper.BindEnv("grpc.port", "GRPC_PORT")
	viper.BindEnv("database.host", "DB_HOST")
	viper.BindEnv("database.port", "DB_PORT")
	viper.BindEnv("database.user", "DB_USER")
	viper.BindEnv("database.password", "DB_PASSWORD")
	viper.BindEnv("database.dbname", "DB_NAME")
	viper.BindEnv("openai.api_key", "OPENAI_API_KEY")
	viper.BindEnv("openai.model", "OPENAI_MODEL")
	viper.BindEnv("openai.base_url", "OPENAI_BASE_URL")
	viper.BindEnv("llm_service.address", "LLM_SERVICE_ADDR")

	if err := viper.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %v", err)
	}

	// 验证必需的配置
	if config.OpenAI.APIKey == "" {
		log.Println("Warning: OpenAI API key not configured")
	}

	return config, nil
}

func (c *DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode)
}
