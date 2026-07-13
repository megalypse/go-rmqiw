package cfg

import (
	"crypto/rand"
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

var generatorFns = map[string]func() string{
	"uuid": generateUUID,
}

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

		if err := applyGeneratedVariables(flowFile, &flow); err != nil {
			return nil, err
		}

		flows = append(flows, &flow)
	}

	return flows, nil
}

func applyGeneratedVariables(flowFile []byte, flow *models.Flow) error {
	generators, err := flowGeneratorDefinitions(flowFile, flow)
	if err != nil {
		return err
	}

	values := make(map[string]string, len(generators))
	for name, expression := range generators {
		value, err := evaluateGenerator(expression)
		if err != nil {
			return fmt.Errorf("failed to generate %q: %w", name, err)
		}

		values[name] = value
	}

	replaceFlowVars(flow, values)
	return nil
}

func flowGeneratorDefinitions(flowFile []byte, flow *models.Flow) (map[string]string, error) {
	generators := make(map[string]string)
	for name, expression := range flow.Vars {
		generators[name] = expression
	}
	for name, expression := range flow.Variables {
		generators[name] = expression
	}

	var rawRoot map[string]json.RawMessage
	if err := json.Unmarshal(flowFile, &rawRoot); err != nil {
		return nil, err
	}

	for name, rawValue := range rawRoot {
		if isKnownFlowField(name) {
			continue
		}

		var expression string
		if err := json.Unmarshal(rawValue, &expression); err != nil {
			continue
		}
		if isGeneratorCall(expression) {
			generators[name] = expression
		}
	}

	return generators, nil
}

func isKnownFlowField(name string) bool {
	switch name {
	case "name", "description", "steps", "vars", "variables":
		return true
	default:
		return false
	}
}

func evaluateGenerator(expression string) (string, error) {
	if !isGeneratorCall(expression) {
		return expression, nil
	}

	fnName := strings.TrimSuffix(expression, "()")
	fn, ok := generatorFns[fnName]
	if !ok {
		return "", fmt.Errorf("unknown generator function %q", fnName)
	}

	return fn(), nil
}

func isGeneratorCall(expression string) bool {
	return strings.HasSuffix(expression, "()") && len(expression) > len("()")
}

func replaceFlowVars(flow *models.Flow, values map[string]string) {
	if len(values) == 0 {
		return
	}

	for stepIndex := range flow.Steps {
		step := &flow.Steps[stepIndex]
		step.PollQuery = replaceVars(step.PollQuery, values)

		for key, value := range step.Message.Headers {
			step.Message.Headers[key] = replaceVars(value, values)
		}

		step.Message.Body = json.RawMessage(replaceVars(string(step.Message.Body), values))
	}
}

func replaceVars(value string, values map[string]string) string {
	for name, generatedValue := range values {
		value = strings.ReplaceAll(value, "{{"+name+"}}", generatedValue)
	}

	return value
}

func generateUUID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		panic(fmt.Errorf("failed to generate uuid: %w", err))
	}

	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
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
