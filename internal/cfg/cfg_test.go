package cfg

import (
	"encoding/json"
	"regexp"
	"testing"

	"github.com/megalypse/go/rmqiw/internal/domain/models"
)

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

	if err := applyGeneratedVariables(flowFile, &flow); err != nil {
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

	if err := applyGeneratedVariables(flowFile, &flow); err != nil {
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

	if err := applyGeneratedVariables(flowFile, &flow); err == nil {
		t.Fatal("expected unknown generator error")
	}
}

func isUUID(value string) bool {
	return regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`).MatchString(value)
}
