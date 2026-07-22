package cfg

import (
	"context"
	"crypto/rand"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/megalypse/go/rmqiw/internal/domain/models"
)

type cfgFlowsPaths struct {
	cfgPath   string
	flowsPath string
}

var GetFlows = sync.OnceValues(loadFlows)
var getPaths = sync.OnceValues(loadPaths)
var getProfiles = sync.OnceValues(loadProfiles)

var profileSelection struct {
	sync.RWMutex
	index int
}

var generatorFns = map[string]func() any{
	"uuid": generateUUID,
}

var querySQLValue = querySQL

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

		rootVariables, err := flowRootVariables(flowFile)
		if err != nil {
			return nil, err
		}
		flow.RootVariables = rootVariables

		flows = append(flows, &flow)
	}

	return flows, nil
}

func ResolveFlow(flow *models.Flow) (*models.Flow, error) {
	cfg, err := GetCfg()
	if err != nil {
		return nil, err
	}
	return ResolveFlowWithConfig(flow, cfg)
}

// ResolveFlowWithConfig resolves a flow using a fixed profile snapshot. This
// keeps an in-progress journey on the profile with which it was started.
func ResolveFlowWithConfig(flow *models.Flow, cfg *Config) (*models.Flow, error) {
	resolvedFlow, err := cloneFlow(flow)
	if err != nil {
		return nil, err
	}

	globalValues, err := evaluateVariables(configVariables(cfg))
	if err != nil {
		return nil, err
	}

	if err := applyVariables(resolvedFlow, globalValues); err != nil {
		return nil, err
	}

	return resolvedFlow, nil
}

// GetCfg returns the currently selected configuration profile.
func GetCfg() (*Config, error) {
	profiles, err := getProfiles()
	if err != nil {
		return nil, err
	}

	profileSelection.RLock()
	defer profileSelection.RUnlock()

	return profiles[profileSelection.index], nil
}

// NextProfile selects and returns the next configuration profile, wrapping at
// the end of the list.
func NextProfile() (*Config, error) {
	profiles, err := getProfiles()
	if err != nil {
		return nil, err
	}

	profileSelection.Lock()
	defer profileSelection.Unlock()

	profileSelection.index = (profileSelection.index + 1) % len(profiles)
	return profiles[profileSelection.index], nil
}

func cloneFlow(flow *models.Flow) (*models.Flow, error) {
	flowBytes, err := json.Marshal(flow)
	if err != nil {
		return nil, err
	}

	var cloned models.Flow
	if err := json.Unmarshal(flowBytes, &cloned); err != nil {
		return nil, err
	}

	cloned.RootVariables = cloneMap(flow.RootVariables)
	return &cloned, nil
}

func cloneMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}

	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func applyVariables(flow *models.Flow, globalValues map[string]any) error {
	flowVars := flowVariableDefinitions(flow)

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

func flowRootVariables(flowFile []byte) (map[string]any, error) {
	var rawRoot map[string]json.RawMessage
	if err := json.Unmarshal(flowFile, &rawRoot); err != nil {
		return nil, err
	}

	rootVariables := make(map[string]any)
	for name, rawValue := range rawRoot {
		if isKnownFlowField(name) {
			continue
		}

		var value any
		if err := json.Unmarshal(rawValue, &value); err != nil {
			return nil, err
		}
		rootVariables[name] = value
	}

	return rootVariables, nil
}

func applyVariablesForTest(flowFile []byte, flow *models.Flow, globalValues map[string]any) error {
	rootVariables, err := flowRootVariables(flowFile)
	if err != nil {
		return err
	}
	flow.RootVariables = rootVariables

	return applyVariables(flow, globalValues)
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

func flowVariableDefinitions(flow *models.Flow) map[string]any {
	vars := make(map[string]any)
	for name, value := range flow.Vars {
		vars[name] = value
	}
	for name, value := range flow.Variables {
		vars[name] = value
	}
	for name, value := range flow.RootVariables {
		vars[name] = value
	}

	return vars
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
	if ok {
		return evaluateGenerator(expression)
	}

	query, ok := sqlQuery(value)
	if ok {
		return querySQLValue(context.Background(), query)
	}

	return value, nil
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

func sqlQuery(value any) (string, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return "", false
	}

	query, ok := object["sql"].(string)
	return query, ok && query != ""
}

func querySQL(ctx context.Context, query string) (any, error) {
	query = normalizeSQLVariableQuery(query)

	config, err := GetCfg()
	if err != nil {
		return nil, err
	}

	conn, err := pgx.Connect(ctx, postgresURL(config.Postgres))
	if err != nil {
		return nil, err
	}
	defer conn.Close(ctx)

	rows, err := conn.Query(ctx, query, pgx.QueryExecModeSimpleProtocol)
	if err != nil {
		return nil, fmt.Errorf("failed to execute SQL variable query %q: %w", query, err)
	}
	defer rows.Close()

	if len(rows.FieldDescriptions()) != 1 {
		return nil, fmt.Errorf("sql variable query must return exactly 1 column: %q", query)
	}

	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("failed to read SQL variable query %q: %w", query, err)
		}
		return nil, fmt.Errorf("sql variable query returned no rows: %q: %w", query, pgx.ErrNoRows)
	}

	values, err := rows.Values()
	if err != nil {
		return nil, fmt.Errorf("failed to read SQL variable query %q: %w", query, err)
	}

	if rows.Next() {
		return nil, fmt.Errorf("sql variable query must return exactly 1 row: %q", query)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to read SQL variable query %q: %w", query, err)
	}

	value, err := normalizeSQLValue(values[0])
	if err != nil {
		return nil, fmt.Errorf("failed to normalize SQL variable query %q: %w", query, err)
	}

	return value, nil
}

func normalizeSQLVariableQuery(query string) string {
	return strings.TrimRight(strings.TrimSpace(query), "; \t\r\n")
}

func normalizeSQLValue(value any) (any, error) {
	if numeric, ok := value.(pgtype.Numeric); ok {
		return normalizeSQLNumeric(numeric)
	}

	if valuer, ok := value.(driver.Valuer); ok {
		driverValue, err := valuer.Value()
		if err != nil {
			return nil, err
		}
		return normalizeSQLValue(driverValue)
	}

	switch typedValue := value.(type) {
	case []byte:
		return string(typedValue), nil
	default:
		return value, nil
	}
}

func normalizeSQLNumeric(value pgtype.Numeric) (any, error) {
	if !value.Valid {
		return nil, nil
	}

	intValue, err := value.Int64Value()
	if err == nil && intValue.Valid {
		return intValue.Int64, nil
	}

	floatValue, err := value.Float64Value()
	if err != nil {
		return nil, err
	}
	if !floatValue.Valid {
		return nil, nil
	}

	return floatValue.Float64, nil
}

func postgresURL(config PostgresConfig) string {
	port := config.Port
	if port == 0 {
		port = 5432
	}

	sslMode := config.SSLMode
	if sslMode == "" {
		sslMode = "disable"
	}

	uri := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(config.User, config.Password),
		Host:   net.JoinHostPort(config.Host, strconv.Itoa(port)),
	}

	if config.Database != "" {
		uri.Path = config.Database
	}

	query := uri.Query()
	query.Set("sslmode", sslMode)
	query.Set("default_transaction_read_only", "on")
	uri.RawQuery = query.Encode()

	return uri.String()
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

func loadProfiles() ([]*Config, error) {
	cfgPaths, err := getPaths()
	if err != nil {
		return nil, err
	}

	cfgFile, err := os.ReadFile(cfgPaths.cfgPath)
	if err != nil {
		return nil, err
	}

	profiles, err := parseProfiles(cfgFile)
	if err != nil {
		return nil, err
	}

	for _, profile := range profiles {
		applyEnvOverrides(profile)
	}

	return profiles, nil
}

func parseProfiles(cfgFile []byte) ([]*Config, error) {
	var profiles []*Config
	if err := json.Unmarshal(cfgFile, &profiles); err != nil {
		return nil, fmt.Errorf("config must be an array of profiles: %w", err)
	}

	if len(profiles) == 0 {
		return nil, fmt.Errorf("config must contain at least one profile")
	}

	names := make(map[string]struct{}, len(profiles))
	for i, profile := range profiles {
		if profile == nil {
			return nil, fmt.Errorf("config profile at index %d must be an object", i)
		}

		profile.Name = strings.TrimSpace(profile.Name)
		if profile.Name == "" {
			return nil, fmt.Errorf("config profile at index %d must have a name", i)
		}
		if _, exists := names[profile.Name]; exists {
			return nil, fmt.Errorf("config profile name %q is duplicated", profile.Name)
		}
		names[profile.Name] = struct{}{}
	}

	return profiles, nil
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
	applyBoolEnv("RMQIW_RABBITMQ_TLS", &cfg.RabbitMQ.TLS)
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

func applyBoolEnv(name string, value *bool) {
	envValue := os.Getenv(name)
	if envValue == "" {
		return
	}

	parsed, err := strconv.ParseBool(envValue)
	if err != nil {
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
