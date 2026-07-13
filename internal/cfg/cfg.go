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

var generatorFns = map[string]func() any{
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

	cfg, err := GetCfg()
	if err != nil {
		return nil, err
	}

	globalValues, err := evaluateVariables(configVariables(cfg))
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

		if err := applyVariables(flowFile, &flow, globalValues); err != nil {
			return nil, err
		}

		flows = append(flows, &flow)
	}

	return flows, nil
}

func applyVariables(flowFile []byte, flow *models.Flow, globalValues map[string]any) error {
	flowVars, err := flowVariableDefinitions(flowFile, flow)
	if err != nil {
		return err
	}

	values := make(map[string]any, len(globalValues)+len(flowVars))
	for name, value := range globalValues {
		values[name] = value
	}

	flowValues, err := evaluateVariables(flowVars)
	if err != nil {
		return err
	}
	for name, value := range flowValues {
		values[name] = value
	}

	if err := replaceFlowVars(flow, values); err != nil {
		return err
	}
	return nil
}

func configVariables(cfg *Config) map[string]any {
	vars := make(map[string]any, len(cfg.Vars)+len(cfg.Variables))
	for name, value := range cfg.Vars {
		vars[name] = value
	}
	for name, value := range cfg.Variables {
		vars[name] = value
	}
	return vars
}

func flowVariableDefinitions(flowFile []byte, flow *models.Flow) (map[string]any, error) {
	vars := make(map[string]any)
	for name, value := range flow.Vars {
		vars[name] = value
	}
	for name, value := range flow.Variables {
		vars[name] = value
	}

	var rawRoot map[string]json.RawMessage
	if err := json.Unmarshal(flowFile, &rawRoot); err != nil {
		return nil, err
	}

	for name, rawValue := range rawRoot {
		if isKnownFlowField(name) {
			continue
		}

		var value any
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return nil, err
		}
		vars[name] = value
	}

	return vars, nil
}

func isKnownFlowField(name string) bool {
	switch name {
	case "name", "description", "steps", "vars", "variables":
		return true
	default:
		return false
	}
}

func evaluateVariables(vars map[string]any) (map[string]any, error) {
	values := make(map[string]any, len(vars))
	for name, value := range vars {
		evaluatedValue, err := evaluateVariable(value)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate variable %q: %w", name, err)
		}

		values[name] = evaluatedValue
	}
	return values, nil
}

func evaluateVariable(value any) (any, error) {
	expression, ok := value.(string)
	if !ok {
		return value, nil
	}

	return evaluateGenerator(expression)
}

func evaluateGenerator(expression string) (any, error) {
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

func replaceFlowVars(flow *models.Flow, values map[string]any) error {
	if len(values) == 0 {
		return nil
	}

	for stepIndex := range flow.Steps {
		step := &flow.Steps[stepIndex]
		step.PollQuery = replaceVars(step.PollQuery, values)

		for key, value := range step.Message.Headers {
			step.Message.Headers[key] = replaceVars(value, values)
		}

		body, err := replaceBodyVars(step.Message.Body, values)
		if err != nil {
			return err
		}
		step.Message.Body = body
	}

	return nil
}

func replaceVars(value string, values map[string]any) string {
	for name, generatedValue := range values {
		value = strings.ReplaceAll(value, "{{"+name+"}}", stringifyVariable(generatedValue))
	}

	return value
}

func replaceBodyVars(body json.RawMessage, values map[string]any) (json.RawMessage, error) {
	if len(body) == 0 {
		return body, nil
	}

	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}

	replaced := replaceJSONVars(parsed, values)
	replacedBody, err := json.Marshal(replaced)
	if err != nil {
		return nil, err
	}

	return json.RawMessage(replacedBody), nil
}

func replaceJSONVars(value any, values map[string]any) any {
	switch typedValue := value.(type) {
	case map[string]any:
		for key, nestedValue := range typedValue {
			typedValue[key] = replaceJSONVars(nestedValue, values)
		}
		return typedValue
	case []any:
		for index, nestedValue := range typedValue {
			typedValue[index] = replaceJSONVars(nestedValue, values)
		}
		return typedValue
	case string:
		if name, ok := variableName(typedValue); ok {
			if variableValue, exists := values[name]; exists {
				return variableValue
			}
		}
		return replaceVars(typedValue, values)
	default:
		return value
	}
}

func variableName(value string) (string, bool) {
	if !strings.HasPrefix(value, "{{") || !strings.HasSuffix(value, "}}") {
		return "", false
	}

	name := strings.TrimSuffix(strings.TrimPrefix(value, "{{"), "}}")
	return name, name != ""
}

func stringifyVariable(value any) string {
	switch typedValue := value.(type) {
	case string:
		return typedValue
	case nil:
		return ""
	default:
		return fmt.Sprint(typedValue)
	}
}

func generateUUID() any {
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
