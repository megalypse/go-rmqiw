package cfg

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/megalypse/go/rmqiw/internal/domain/models"
)

type cfgFlowsPaths struct {
	cfgPath   string
	flowsPath string
}

var GetCfg = sync.OnceValues(loadCfg)
var GetFlows = sync.OnceValues(loadFlows)
var getPaths = sync.OnceValues(loadPaths)

func loadFlows() ([]*models.Flow, error) {
	paths, err := getPaths()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(paths.flowsPath)
	if err != nil {
		return nil, err
	}

	var flows []*models.Flow
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		flowFile, err := os.ReadFile(path.Join(paths.flowsPath, entry.Name()))
		if err != nil {
			return nil, err
		}

		var flow models.Flow
		err = json.Unmarshal(flowFile, &flow)
		if err != nil {
			return nil, err
		}

		flows = append(flows, &flow)
	}

	return flows, nil
}

func loadCfg() (*Config, error) {
	cfgPaths, err := getPaths()
	if err != nil {
		return nil, err
	}

	cfgFile, err := os.ReadFile(cfgPaths.cfgPath)
	if err != nil {
		return nil, err
	}

	var cfg Config
	err = json.Unmarshal(cfgFile, &cfg)
	if err != nil {
		return nil, err
	}

	applyEnvOverrides(&cfg)

	return &cfg, nil
}

func applyEnvOverrides(cfg *Config) {
	applyStringEnv("RMQIW_POSTGRES_HOST", &cfg.Postgres.Host)
	applyIntEnv("RMQIW_POSTGRES_PORT", &cfg.Postgres.Port)
	applyStringEnv("RMQIW_POSTGRES_USER", &cfg.Postgres.User)
	applyStringEnv("RMQIW_POSTGRES_PASSWORD", &cfg.Postgres.Password)
	applyStringEnv("RMQIW_POSTGRES_DATABASE", &cfg.Postgres.Database)
	applyStringEnv("RMQIW_POSTGRES_SSL_MODE", &cfg.Postgres.SSLMode)

	applyStringEnv("RMQIW_RABBITMQ_HOST", &cfg.RabbitMQ.Host)
	applyIntEnv("RMQIW_RABBITMQ_PORT", &cfg.RabbitMQ.Port)
	applyStringEnv("RMQIW_RABBITMQ_USER", &cfg.RabbitMQ.User)
	applyStringEnv("RMQIW_RABBITMQ_PASSWORD", &cfg.RabbitMQ.Password)
	applyStringEnv("RMQIW_RABBITMQ_VHOST", &cfg.RabbitMQ.VHost)
}

func applyStringEnv(name string, value *string) {
	envValue := os.Getenv(name)
	if envValue == "" {
		return
	}

	*value = envValue
}

func applyIntEnv(name string, value *int) {
	envValue := os.Getenv(name)
	if envValue == "" {
		return
	}

	var parsed int
	if _, err := fmt.Sscanf(envValue, "%d", &parsed); err != nil {
		return
	}

	*value = parsed
}

func loadPaths() (*cfgFlowsPaths, error) {
	configPath := os.Getenv("RMQIW_PATH")
	if configPath == "" {
		return nil, os.ErrNotExist
	}

	entries, err := os.ReadDir(configPath)
	if err != nil {
		return nil, err
	}

	var configFilePath string
	var flowsPath string
	for _, entry := range entries {
		if entry.Name() == "config.json" {
			configFilePath = path.Join(configPath, entry.Name())
			continue
		}

		if strings.HasPrefix(entry.Name(), "flows") && entry.IsDir() {
			flowsPath = path.Join(configPath, entry.Name())
			continue
		}
	}

	return &cfgFlowsPaths{
		cfgPath:   configFilePath,
		flowsPath: flowsPath,
	}, nil
}
