package cfg

import (
	"encoding/json"
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

	return &cfg, nil
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
