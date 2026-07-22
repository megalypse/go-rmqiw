package cfg

import (
	"context"
	"encoding/json"
	"math/big"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/megalypse/go/rmqiw/internal/domain/models"
)

func TestParseProfiles(t *testing.T) {
	profiles, err := parseProfiles([]byte(`[
		{"name":" local ","postgres":{"host":"localhost"}},
		{"name":"staging","rabbitmq":{"host":"rabbitmq.staging"}}
	]`))
	if err != nil {
		t.Fatal(err)
	}

	if len(profiles) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(profiles))
	}
	if profiles[0].Name != "local" {
		t.Fatalf("expected trimmed profile name, got %q", profiles[0].Name)
	}
	if profiles[1].RabbitMQ.Host != "rabbitmq.staging" {
		t.Fatalf("expected staging RabbitMQ host, got %q", profiles[1].RabbitMQ.Host)
	}
}

func TestParseProfilesRejectsInvalidConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		wantErr string
	}{
		{name: "object", config: `{}`, wantErr: "array of profiles"},
		{name: "empty", config: `[]`, wantErr: "at least one profile"},
		{name: "missing name", config: `[{"postgres":{}}]`, wantErr: "must have a name"},
		{name: "null profile", config: `[null]`, wantErr: "must be an object"},
		{name: "duplicate name", config: `[{"name":"local"},{"name":"local"}]`, wantErr: "is duplicated"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseProfiles([]byte(tt.config))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestNextProfileCyclesThroughProfiles(t *testing.T) {
	originalGetProfiles := getProfiles
	defer func() {
		getProfiles = originalGetProfiles
		profileSelection.Lock()
		profileSelection.index = 0
		profileSelection.Unlock()
	}()

	profiles := []*Config{{Name: "local"}, {Name: "staging"}}
	getProfiles = sync.OnceValues(func() ([]*Config, error) {
		return profiles, nil
	})
	profileSelection.Lock()
	profileSelection.index = 0
	profileSelection.Unlock()

	profile, err := NextProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != "staging" {
		t.Fatalf("expected staging profile, got %q", profile.Name)
	}

	profile, err = NextProfile()
	if err != nil {
		t.Fatal(err)
	}
	if profile.Name != "local" {
		t.Fatalf("expected profile selection to wrap to local, got %q", profile.Name)
	}
}

func TestApplyGeneratedVariablesFromVars(t *testing.T) {
	flowFile := []byte(`{
		"name": "Generated vars",
		"vars": {
			"requestId": "uuid()"
		},
		"steps": [
			{
				"poll_query": "SELECT '{{requestId}}'",
				"message": {
					"headers": {
						"x-request-id": "{{requestId}}"
					},
					"body": {
						"id": "{{requestId}}",
						"nested": "prefix-{{requestId}}"
					}
				}
			}
		]
	}`)

	var flow models.Flow
	if err := json.Unmarshal(flowFile, &flow); err != nil {
		t.Fatal(err)
	}

	if err := applyVariablesForTest(flowFile, &flow, nil); err != nil {
		t.Fatal(err)
	}

	requestID := flow.Steps[0].Message.Headers["x-request-id"]
	if !isUUID(requestID) {
		t.Fatalf("expected uuid header, got %q", requestID)
	}

	if flow.Steps[0].PollQuery != "SELECT '"+requestID+"'" {
		t.Fatalf("expected poll query to use generated value, got %q", flow.Steps[0].PollQuery)
	}

	var body map[string]string
	if err := json.Unmarshal(flow.Steps[0].Message.Body, &body); err != nil {
		t.Fatal(err)
	}

	if body["id"] != requestID {
		t.Fatalf("expected body id %q, got %q", requestID, body["id"])
	}
	if body["nested"] != "prefix-"+requestID {
		t.Fatalf("expected nested generated value, got %q", body["nested"])
	}
}

func TestApplyGeneratedVariablesFromRootField(t *testing.T) {
	flowFile := []byte(`{
		"name": "Root generated vars",
		"requestId": "uuid()",
		"steps": [
			{
				"message": {
					"headers": {
						"x-request-id": "{{requestId}}"
					},
					"body": {"id": "{{requestId}}"}
				}
			}
		]
	}`)

	var flow models.Flow
	if err := json.Unmarshal(flowFile, &flow); err != nil {
		t.Fatal(err)
	}

	if err := applyVariablesForTest(flowFile, &flow, nil); err != nil {
		t.Fatal(err)
	}

	if requestID := flow.Steps[0].Message.Headers["x-request-id"]; !isUUID(requestID) {
		t.Fatalf("expected uuid header, got %q", requestID)
	}
}

func TestApplyGeneratedVariablesRejectsUnknownFunction(t *testing.T) {
	flowFile := []byte(`{
		"name": "Unknown generated vars",
		"vars": {
			"requestId": "unknown()"
		},
		"steps": []
	}`)

	var flow models.Flow
	if err := json.Unmarshal(flowFile, &flow); err != nil {
		t.Fatal(err)
	}

	if err := applyVariablesForTest(flowFile, &flow, nil); err == nil {
		t.Fatal("expected unknown generator error")
	}
}

func TestApplyVariablesPreservesBodyValueTypes(t *testing.T) {
	flowFile := []byte(`{
		"name": "Typed vars",
		"vars": {
			"retryCount": 3,
			"enabled": true,
			"metadata": {"source": "flow"}
		},
		"steps": [
			{
				"message": {
					"headers": {
						"x-retry-count": "{{retryCount}}"
					},
					"body": {
						"retry_count": "{{retryCount}}",
						"enabled": "{{enabled}}",
						"metadata": "{{metadata}}",
						"label": "retry-{{retryCount}}"
					}
				}
			}
		]
	}`)

	var flow models.Flow
	if err := json.Unmarshal(flowFile, &flow); err != nil {
		t.Fatal(err)
	}

	if err := applyVariablesForTest(flowFile, &flow, nil); err != nil {
		t.Fatal(err)
	}

	if flow.Steps[0].Message.Headers["x-retry-count"] != "3" {
		t.Fatalf("expected stringified header, got %q", flow.Steps[0].Message.Headers["x-retry-count"])
	}

	var body map[string]any
	if err := json.Unmarshal(flow.Steps[0].Message.Body, &body); err != nil {
		t.Fatal(err)
	}

	if body["retry_count"] != float64(3) {
		t.Fatalf("expected numeric retry_count, got %#v", body["retry_count"])
	}
	if body["enabled"] != true {
		t.Fatalf("expected boolean enabled, got %#v", body["enabled"])
	}
	if body["label"] != "retry-3" {
		t.Fatalf("expected interpolated label, got %#v", body["label"])
	}
	metadata, ok := body["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected object metadata, got %#v", body["metadata"])
	}
	if metadata["source"] != "flow" {
		t.Fatalf("expected metadata source, got %#v", metadata["source"])
	}
}

func TestApplyVariablesUsesGlobalVariables(t *testing.T) {
	flowFile := []byte(`{
		"name": "Global vars",
		"steps": [
			{
				"poll_query": "SELECT '{{tenantId}}'",
				"message": {
					"headers": {
						"x-tenant-id": "{{tenantId}}"
					},
					"body": {"tenant_id": "{{tenantId}}"}
				}
			}
		]
	}`)

	var flow models.Flow
	if err := json.Unmarshal(flowFile, &flow); err != nil {
		t.Fatal(err)
	}

	if err := applyVariablesForTest(flowFile, &flow, map[string]any{"tenantId": "tenant-1"}); err != nil {
		t.Fatal(err)
	}

	if flow.Steps[0].PollQuery != "SELECT 'tenant-1'" {
		t.Fatalf("expected global value in poll query, got %q", flow.Steps[0].PollQuery)
	}
	if flow.Steps[0].Message.Headers["x-tenant-id"] != "tenant-1" {
		t.Fatalf("expected global value in header, got %q", flow.Steps[0].Message.Headers["x-tenant-id"])
	}
}

func TestFlowVariablesOverrideGlobalVariables(t *testing.T) {
	flowFile := []byte(`{
		"name": "Override vars",
		"vars": {
			"tenantId": "tenant-flow"
		},
		"steps": [
			{
				"message": {
					"body": {"tenant_id": "{{tenantId}}"}
				}
			}
		]
	}`)

	var flow models.Flow
	if err := json.Unmarshal(flowFile, &flow); err != nil {
		t.Fatal(err)
	}

	if err := applyVariablesForTest(flowFile, &flow, map[string]any{"tenantId": "tenant-global"}); err != nil {
		t.Fatal(err)
	}

	var body map[string]string
	if err := json.Unmarshal(flow.Steps[0].Message.Body, &body); err != nil {
		t.Fatal(err)
	}

	if body["tenant_id"] != "tenant-flow" {
		t.Fatalf("expected flow variable to override global variable, got %q", body["tenant_id"])
	}
}

func TestApplyVariablesUsesSQLQueryResult(t *testing.T) {
	flowFile := []byte(`{
		"name": "SQL vars",
		"vars": {
			"merchantId": {"sql": "SELECT merchant_id FROM merchants LIMIT 1"}
		},
		"steps": [
			{
				"poll_query": "SELECT '{{merchantId}}'",
				"message": {
					"headers": {
						"x-merchant-id": "{{merchantId}}"
					},
					"body": {"merchant_id": "{{merchantId}}"}
				}
			}
		]
	}`)

	var flow models.Flow
	if err := json.Unmarshal(flowFile, &flow); err != nil {
		t.Fatal(err)
	}

	querySQLValue = func(ctx context.Context, query string) (any, error) {
		if query != "SELECT merchant_id FROM merchants LIMIT 1" {
			t.Fatalf("unexpected query %q", query)
		}
		return int64(42), nil
	}
	defer func() {
		querySQLValue = querySQL
	}()

	if err := applyVariablesForTest(flowFile, &flow, nil); err != nil {
		t.Fatal(err)
	}

	if flow.Steps[0].PollQuery != "SELECT '42'" {
		t.Fatalf("expected SQL value in poll query, got %q", flow.Steps[0].PollQuery)
	}
	if flow.Steps[0].Message.Headers["x-merchant-id"] != "42" {
		t.Fatalf("expected SQL value in header, got %q", flow.Steps[0].Message.Headers["x-merchant-id"])
	}

	var body map[string]any
	if err := json.Unmarshal(flow.Steps[0].Message.Body, &body); err != nil {
		t.Fatal(err)
	}
	if body["merchant_id"] != float64(42) {
		t.Fatalf("expected numeric SQL value in body, got %#v", body["merchant_id"])
	}
}

func TestApplyVariablesKeepsNonSQLObjectsHardcoded(t *testing.T) {
	flowFile := []byte(`{
		"name": "Object vars",
		"vars": {
			"metadata": {"source": "flow"}
		},
		"steps": [
			{
				"message": {
					"body": {"metadata": "{{metadata}}"}
				}
			}
		]
	}`)

	var flow models.Flow
	if err := json.Unmarshal(flowFile, &flow); err != nil {
		t.Fatal(err)
	}

	if err := applyVariablesForTest(flowFile, &flow, nil); err != nil {
		t.Fatal(err)
	}

	var body map[string]any
	if err := json.Unmarshal(flow.Steps[0].Message.Body, &body); err != nil {
		t.Fatal(err)
	}
	metadata, ok := body["metadata"].(map[string]any)
	if !ok {
		t.Fatalf("expected hardcoded object, got %#v", body["metadata"])
	}
	if metadata["source"] != "flow" {
		t.Fatalf("expected metadata source, got %#v", metadata["source"])
	}
}

func TestNormalizeSQLVariableQuery(t *testing.T) {
	query := "  WITH test AS (SELECT 1) SELECT * FROM test; \n"
	want := "WITH test AS (SELECT 1) SELECT * FROM test"

	if got := normalizeSQLVariableQuery(query); got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestNormalizeSQLValueConvertsNumericToInt64(t *testing.T) {
	value, err := normalizeSQLValue(pgtype.Numeric{
		Int:   big.NewInt(42),
		Valid: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	if value != int64(42) {
		t.Fatalf("expected normalized numeric value, got %#v", value)
	}
}

func TestLoadFlowsDoesNotEvaluateVariables(t *testing.T) {
	flowFile := []byte(`{
		"name": "Lazy vars",
		"vars": {
			"requestId": "uuid()",
			"merchantId": {"sql": "SELECT merchant_id FROM merchants LIMIT 1"}
		},
		"steps": []
	}`)

	var flow models.Flow
	if err := json.Unmarshal(flowFile, &flow); err != nil {
		t.Fatal(err)
	}

	rootVariables, err := flowRootVariables(flowFile)
	if err != nil {
		t.Fatal(err)
	}
	flow.RootVariables = rootVariables

	if flow.Vars["requestId"] != "uuid()" {
		t.Fatalf("expected unevaluated uuid, got %#v", flow.Vars["requestId"])
	}
	merchantID, ok := flow.Vars["merchantId"].(map[string]any)
	if !ok {
		t.Fatalf("expected unevaluated SQL variable, got %#v", flow.Vars["merchantId"])
	}
	if merchantID["sql"] != "SELECT merchant_id FROM merchants LIMIT 1" {
		t.Fatalf("expected unevaluated SQL query, got %#v", merchantID["sql"])
	}
}

func isUUID(value string) bool {
	return regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(value)
}
