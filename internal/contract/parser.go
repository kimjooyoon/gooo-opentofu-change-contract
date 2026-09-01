package contract

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func parseFields(text string) map[string]string {
	fields := make(map[string]string)
	for _, token := range strings.Fields(text) {
		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			continue
		}
		fields[parts[0]] = strings.Trim(parts[1], "\"")
	}
	return fields
}

func ParseSchema(path string) (Schema, error) {
	file, err := os.Open(path)
	if err != nil {
		return Schema{}, err
	}
	defer file.Close()

	var schema Schema
	schema.Scope = make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		switch {
		case strings.HasPrefix(line, "package "):
			schema.Package = strings.TrimSpace(strings.TrimPrefix(line, "package "))
		case strings.HasPrefix(line, "namespace "):
			schema.Namespace = strings.TrimSpace(strings.TrimPrefix(line, "namespace "))
		case strings.HasPrefix(line, "// @output "):
			fields := parseFields(strings.TrimPrefix(line, "// @output "))
			schema.Outputs = append(schema.Outputs, OutputSpec{Name: fields["name"], Kind: fields["kind"]})
		case strings.HasPrefix(line, "// @activity "):
			fields := parseFields(strings.TrimPrefix(line, "// @activity "))
			schema.Activities = append(schema.Activities, ActivitySpec{ID: fields["id"], Name: fields["name"], Stage: fields["stage"], Step: fields["step"]})
		case strings.HasPrefix(line, "// @action "):
			fields := parseFields(strings.TrimPrefix(line, "// @action "))
			schema.Actions = append(schema.Actions, ActionSpec{Name: fields["name"], Semantics: fields["semantics"]})
		case strings.HasPrefix(line, "// @case "):
			fields := parseFields(strings.TrimPrefix(line, "// @case "))
			schema.Cases = append(schema.Cases, CaseSpec{ID: fields["id"], State: fields["state"], Kind: fields["kind"], Action: fields["action"]})
		case strings.HasPrefix(line, "// @relation "):
			fields := parseFields(strings.TrimPrefix(line, "// @relation "))
			schema.Relations = append(schema.Relations, RelationSpec{From: fields["from"], To: fields["to"], Predicate: fields["predicate"]})
		case strings.HasPrefix(line, "scope "):
			for key, value := range parseFields(strings.TrimPrefix(line, "scope ")) {
				schema.Scope[key] = value
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Schema{}, err
	}
	if err := schema.Validate(); err != nil {
		return Schema{}, err
	}
	return schema, nil
}

func (s Schema) Validate() error {
	if s.Package == "" || s.Namespace == "" {
		return fmt.Errorf(".gooo package and namespace are required")
	}
	if len(s.Outputs) != len(OutputNames) {
		return fmt.Errorf(".gooo must declare exactly %d outputs", len(OutputNames))
	}
	for index, expected := range OutputNames {
		if s.Outputs[index].Name != expected || s.Outputs[index].Kind == "" {
			return fmt.Errorf(".gooo output %d is not %s", index+1, expected)
		}
	}
	if len(s.Activities) != 10 {
		return fmt.Errorf(".gooo must declare exactly 10 activities")
	}
	activityIDs := make(map[string]bool)
	activityNames := make(map[string]bool)
	for _, activity := range s.Activities {
		if activity.ID == "" || activity.Name == "" || activity.Stage == "" || activity.Step == "" {
			return fmt.Errorf("activity metadata is incomplete")
		}
		if activityIDs[activity.ID] || activityNames[activity.Name] {
			return fmt.Errorf("activity identity is not unique")
		}
		activityIDs[activity.ID] = true
		activityNames[activity.Name] = true
	}
	if len(s.Actions) != 3 {
		return fmt.Errorf(".gooo must declare create, update, and delete semantics")
	}
	actions := make(map[string]bool)
	for _, action := range s.Actions {
		if action.Name == "" || action.Semantics == "" || actions[action.Name] {
			return fmt.Errorf("action semantics are incomplete")
		}
		actions[action.Name] = true
	}
	for _, expected := range []string{"create", "update", "delete"} {
		if !actions[expected] {
			return fmt.Errorf("missing action semantics for %s", expected)
		}
	}
	if len(s.Cases) != 9 {
		return fmt.Errorf(".gooo must declare exactly 9 canonical cases")
	}
	caseIDs := make(map[string]bool)
	counts := map[string]int{Closed: 0, Unknown: 0, Refuted: 0}
	for _, item := range s.Cases {
		if item.ID == "" || item.State == "" || item.Kind == "" || caseIDs[item.ID] {
			return fmt.Errorf("canonical case metadata is incomplete or duplicated")
		}
		if _, ok := counts[item.State]; !ok {
			return fmt.Errorf("unsupported canonical case state %s", item.State)
		}
		counts[item.State]++
		caseIDs[item.ID] = true
	}
	if counts[Closed] != 3 || counts[Unknown] != 3 || counts[Refuted] != 3 {
		return fmt.Errorf("canonical case state counts must be CLOSED=3 UNKNOWN=3 REFUTED=3")
	}
	for _, key := range []string{"name", "repository_writes", "input_mutations", "opentofu_invocations", "terraform_invocations", "network_provider_invocations"} {
		if key == "name" {
			if s.Scope[key] == "" {
				return fmt.Errorf("scope name is required")
			}
			continue
		}
		if s.Scope[key] != "0" {
			return fmt.Errorf("scope %s must be zero", key)
		}
	}
	return nil
}

func ReadJSON(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.UseNumber()
	if err := decoder.Decode(destination); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

