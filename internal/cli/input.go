package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"redmine-cli/internal/output"
)

func readJSONInput(path string) (map[string]any, output.Envelope) {
	if path == "" {
		return map[string]any{}, output.Envelope{}
	}
	var data []byte
	var err error
	if path == "-" {
		data, err = io.ReadAll(stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if os.IsNotExist(err) {
		return nil, output.Failure("file_not_found", "Input file was not found", "Check the --input path.", map[string]any{"path": path})
	}
	if err != nil {
		return nil, output.Failure("input_parse_error", "Could not read JSON input", "Check the --input path and permissions.", map[string]any{"path": path, "error": err.Error()})
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, output.Failure("input_parse_error", "Could not parse JSON input", "Provide a JSON object or an object with an issue field.", map[string]any{"path": path, "error": err.Error()})
	}
	if issue, ok := raw["issue"].(map[string]any); ok {
		return issue, output.Envelope{}
	}
	return raw, output.Envelope{}
}

func mergeRawJSON(payload map[string]any, raw string) output.Envelope {
	if strings.TrimSpace(raw) == "" {
		return output.Envelope{}
	}
	if strings.HasPrefix(raw, "@") {
		path := strings.TrimPrefix(raw, "@")
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			return output.Failure("file_not_found", "Raw JSON file was not found", "Check the --raw-json @path.", map[string]any{"path": path})
		}
		if err != nil {
			return output.Failure("input_parse_error", "Could not read raw JSON file", "Check the --raw-json @path.", map[string]any{"path": path, "error": err.Error()})
		}
		raw = string(data)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		return output.Failure("input_parse_error", "Could not parse raw JSON", "Pass a JSON object or @path.", map[string]any{"error": err.Error()})
	}
	for key, value := range obj {
		payload[key] = value
	}
	return output.Envelope{}
}

func readTextFlag(message string, file string, flagName string) (string, output.Envelope) {
	if message != "" && file != "" {
		return "", output.Failure("validation_failed", "Conflicting message inputs", "Use either --"+flagName+" or --"+flagName+"-file, not both.", nil)
	}
	if file == "" {
		return message, output.Envelope{}
	}
	data, err := os.ReadFile(file)
	if os.IsNotExist(err) {
		return "", output.Failure("file_not_found", "Message file was not found", "Check the --"+flagName+"-file path.", map[string]any{"path": file})
	}
	if err != nil {
		return "", output.Failure("input_parse_error", "Could not read message file", "Check the --"+flagName+"-file path and permissions.", map[string]any{"path": file, "error": err.Error()})
	}
	return string(data), output.Envelope{}
}

func applySet(payload map[string]any, raw string) output.Envelope {
	key, value, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return output.Failure("validation_failed", "Invalid --set value", "Use --set field=value.", map[string]any{"value": raw})
	}
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)

	switch key {
	case "subject", "description", "start_date", "due_date", "status", "tracker", "priority", "project", "assignee", "assigned_to":
		payload[key] = value
	case "done_ratio", "estimated_hours", "parent_id":
		n, err := strconv.Atoi(value)
		if err != nil {
			return output.Failure("validation_failed", "Invalid numeric --set value", "Use an integer for "+key+".", map[string]any{"field": key, "value": value})
		}
		payload[key] = n
	case "is_private":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return output.Failure("validation_failed", "Invalid boolean --set value", "Use true or false for is_private.", map[string]any{"field": key, "value": value})
		}
		payload[key] = b
	default:
		return output.Failure("validation_failed", "Unknown --set field", "Supported fields are subject, description, start_date, due_date, done_ratio, estimated_hours, parent_id, and is_private. Use --raw-json for plugin-specific fields.", map[string]any{"field": key})
	}
	return output.Envelope{}
}

func parseKeyValue(raw string, flag string) (string, string, output.Envelope) {
	key, value, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return "", "", output.Failure("validation_failed", "Invalid "+flag+" value", "Use "+flag+" Name=Value or ID=Value.", map[string]any{"value": raw})
	}
	return strings.TrimSpace(key), strings.TrimSpace(value), output.Envelope{}
}

func parseCustomValue(raw string) any {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[") {
		var v any
		if json.Unmarshal([]byte(raw), &v) == nil {
			return v
		}
	}
	return raw
}

func appendCustomField(payload map[string]any, id int, value any) {
	field := map[string]any{"id": id, "value": value}
	var existing []any
	switch current := payload["custom_fields"].(type) {
	case []any:
		existing = append(existing, current...)
	case []map[string]any:
		for _, item := range current {
			existing = append(existing, item)
		}
	}
	existing = append(existing, field)
	payload["custom_fields"] = existing
}

func parseParam(raw string) (string, string, error) {
	key, value, ok := strings.Cut(raw, "=")
	if !ok || strings.TrimSpace(key) == "" {
		return "", "", fmt.Errorf("use --param key=value")
	}
	return strings.TrimSpace(key), value, nil
}
