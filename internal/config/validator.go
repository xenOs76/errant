package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
	"gopkg.in/yaml.v3"
)

//go:embed schema.json
var embeddedSchema []byte

// ValidateYAML validates raw YAML bytes against the embedded JSON schema.
func ValidateYAML(yamlBytes []byte) error {
	var rawData any
	if err := yaml.Unmarshal(yamlBytes, &rawData); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Convert YAML map[any]any structure to clean JSON-compatible types
	jsonBytes, err := json.Marshal(rawData)
	if err != nil {
		return fmt.Errorf("failed to convert YAML to JSON: %w", err)
	}

	var jsonObject any
	if err := json.Unmarshal(jsonBytes, &jsonObject); err != nil {
		return fmt.Errorf("failed to decode JSON: %w", err)
	}

	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("errantfile.schema.json", strings.NewReader(string(embeddedSchema))); err != nil {
		return fmt.Errorf("failed to register schema: %w", err)
	}

	sch, err := compiler.Compile("errantfile.schema.json")
	if err != nil {
		return fmt.Errorf("failed to compile schema: %w", err)
	}

	if err := sch.Validate(jsonObject); err != nil {
		return fmt.Errorf("configuration schema validation error:\n%w", err)
	}

	return nil
}
