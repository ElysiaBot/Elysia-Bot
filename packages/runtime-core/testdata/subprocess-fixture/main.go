package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	mode := "echo"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	_, _ = os.Stderr.WriteString("stderr-online\n")
	if mode == "hang-handshake" {
		time.Sleep(5 * time.Second)
		return
	}
	if mode == "bad-handshake" {
		_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"ok\",\"message\":\"wrong-handshake\"}\n")
		return
	}
	_, _ = os.Stdout.WriteString("{\"type\":\"handshake\",\"status\":\"ok\",\"message\":\"handshake-ready\"}\n")
	if mode == "crash-after-handshake" {
		os.Exit(2)
	}
	if strings.HasPrefix(mode, "crash-after-handshake-once:") {
		marker := strings.TrimPrefix(mode, "crash-after-handshake-once:")
		if _, err := os.Stat(marker); errors.Is(err, os.ErrNotExist) {
			_ = os.WriteFile(marker, []byte("crashed"), 0o644)
			os.Exit(2)
		}
	}
	if mode == "exit-after-handshake" {
		return
	}

	reader := bufio.NewScanner(os.Stdin)
	crashed := false
	for reader.Scan() {
		line := reader.Text()
		if strings.Contains(line, `"type":"health"`) {
			if mode == "hang-health" {
				time.Sleep(5 * time.Second)
				return
			}
			_, _ = os.Stdout.WriteString("{\"type\":\"health\",\"status\":\"ok\",\"message\":\"healthy\"}\n")
			continue
		}
		if strings.Contains(line, `"type":"event"`) {
			if mode == "assert-instance-config" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				prefix, ok := instanceConfig["prefix"].(string)
				if !ok || prefix != "!" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected instance_config prefix value\"}\n")
					continue
				}
			}
			if mode == "assert-default-not-injected" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if rawInstanceConfig, ok := requestEnvelope["instance_config"]; ok && len(rawInstanceConfig) > 0 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected instance_config field for manifest default-only request\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-enum-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				prefix, ok := instanceConfig["prefix"].(string)
				if !ok || prefix != "oops" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected out-of-enum instance_config prefix value\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-nested-enum-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := settings["prefix"].(string)
				if !ok || prefix != "oops" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected out-of-enum nested instance_config prefix value\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-nested-default-not-injected" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				if len(settings) != 0 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected nested default injection in instance_config payload\"}\n")
					continue
				}
				if _, exists := settings["prefix"]; exists {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected nested default member in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-nested-required-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				if _, exists := settings["prefix"]; exists {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected nested required member in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-nested-type-mismatch-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := settings["prefix"].(bool)
				if !ok || !prefix {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected nested type-mismatch instance_config prefix value\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-deeper-nested-type-mismatch-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := labels["prefix"].(bool)
				if !ok || !prefix {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected deeper nested type-mismatch instance_config prefix value\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-type-mismatch-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := naming["prefix"].(bool)
				if !ok || !prefix {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested type-mismatch instance_config prefix value\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-explicit-wrong-type-bad-value-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := naming["prefix"].(bool)
				if !ok || !prefix {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit wrong-type instance_config prefix value\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-explicit-array-valued-bad-value-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := naming["prefix"].([]any)
				if !ok || len(prefix) != 1 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit array-valued instance_config prefix\"}\n")
					continue
				}
				first, ok := prefix[0].(string)
				if !ok || first != "oops" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit array-valued member in instance_config prefix\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-explicit-object-valued-bad-value-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := naming["prefix"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit object-valued instance_config prefix\"}\n")
					continue
				}
				bad, ok := prefix["bad"].(bool)
				if !ok || !bad {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit object-valued member in instance_config prefix\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-explicit-object-node-bad-value-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(bool)
				if !ok || !naming {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit object-node bad value in instance_config naming\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-explicit-string-valued-object-node-bad-value-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(string)
				if !ok || naming != "oops" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit string-valued object-node bad value in instance_config naming\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-explicit-number-valued-object-node-bad-value-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(float64)
				if !ok || naming != 7 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit number-valued object-node bad value in instance_config naming\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-explicit-null-valued-object-node-bad-value-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, exists := labels["naming"]
				if !exists || naming != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit null-valued object-node bad value in instance_config naming\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-explicit-array-valued-object-node-bad-value-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].([]any)
				if !ok || len(naming) != 1 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit array-valued object-node bad value in instance_config naming\"}\n")
					continue
				}
				first, ok := naming[0].(string)
				if !ok || first != "oops" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit array-valued object-node member in instance_config naming\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-array-value-mismatch-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := naming["prefix"].([]any)
				if !ok || len(prefix) != 1 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested array-valued instance_config prefix\"}\n")
					continue
				}
				first, ok := prefix[0].(string)
				if !ok || first != "oops" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested array member in instance_config prefix\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-object-value-mismatch-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := naming["prefix"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested object-valued instance_config prefix\"}\n")
					continue
				}
				bad, ok := prefix["bad"].(bool)
				if !ok || !bad {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested object-valued member in instance_config prefix\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-object-node-mismatch-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(bool)
				if !ok || !naming {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested object-node mismatch instance_config naming value\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-deeper-nested-enum-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := labels["prefix"].(string)
				if !ok || prefix != "oops" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected deeper nested out-of-enum instance_config prefix value\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				if len(naming) != 0 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required injection in instance_config payload\"}\n")
					continue
				}
				if _, exists := naming["prefix"]; exists {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required member in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-enum-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := naming["prefix"].(string)
				if !ok || prefix != "oops" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested out-of-enum instance_config prefix value\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-default-not-injected" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				if len(naming) != 0 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested default injection in instance_config payload\"}\n")
					continue
				}
				if _, exists := naming["prefix"]; exists {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested default member in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-default-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				if len(naming) != 0 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+default injection in instance_config payload\"}\n")
					continue
				}
				if _, exists := naming["prefix"]; exists {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+default member in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-enum-default-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				if len(naming) != 0 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested enum+default injection in instance_config payload\"}\n")
					continue
				}
				if _, exists := naming["prefix"]; exists {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested enum+default member in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				if len(naming) != 0 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum injection in instance_config payload\"}\n")
					continue
				}
				if _, exists := naming["prefix"]; exists {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum member in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				if len(naming) != 0 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default injection in instance_config payload\"}\n")
					continue
				}
				if _, exists := naming["prefix"]; exists {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default member in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-child-omission-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing synthesized beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := naming["prefix"].(string)
				if !ok || prefix != "hello" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing synthesized beyond-supported deeper nested child omission default in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-parent-omission-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				if len(settings) != 0 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default parent omission mutation in instance_config payload\"}\n")
					continue
				}
				if _, exists := settings["labels"]; exists {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default parent omission labels key in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-root-omission-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				if _, ok := requestEnvelope["instance_config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected instance_config field for beyond-supported deeper nested required+enum+default root omission payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-required-enum-default-explicit-bad-value-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := naming["prefix"].(string)
				if !ok || prefix != "oops" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected beyond-supported deeper nested required+enum+default explicit bad value in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-beyond-supported-deeper-nested-default-merged" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				naming, ok := labels["naming"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing beyond-supported naming object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := naming["prefix"].(string)
				if !ok || prefix != "hello" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"expected merged beyond-supported deeper nested default in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-deeper-nested-array-value-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				prefix, ok := labels["prefix"].([]any)
				if !ok || len(prefix) != 1 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected deeper nested array-valued instance_config prefix\"}\n")
					continue
				}
				first, ok := prefix[0].(string)
				if !ok || first != "oops" {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected deeper nested array member in instance_config prefix\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-deeper-nested-required-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				if _, exists := labels["prefix"]; exists {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected deeper nested required member in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "assert-instance-config-deeper-nested-default-not-injected" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected config field in host request\"}\n")
					continue
				}
				rawInstanceConfig, ok := requestEnvelope["instance_config"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing instance_config field in host request\"}\n")
					continue
				}
				var instanceConfig map[string]any
				if err := json.Unmarshal(rawInstanceConfig, &instanceConfig); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"failed to decode instance_config payload\"}\n")
					continue
				}
				settings, ok := instanceConfig["settings"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing nested settings object in instance_config payload\"}\n")
					continue
				}
				labels, ok := settings["labels"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"missing deeper nested labels object in instance_config payload\"}\n")
					continue
				}
				if len(labels) != 0 {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected deeper nested default injection in instance_config payload\"}\n")
					continue
				}
				if _, exists := labels["prefix"]; exists {
					_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"error\",\"error\":\"unexpected deeper nested default member in instance_config payload\"}\n")
					continue
				}
			}
			if mode == "hang-event" {
				time.Sleep(5 * time.Second)
				return
			}
			if mode == "flood-stderr" {
				for i := range 5 {
					_, _ = fmt.Fprintf(os.Stderr, "flood-%d\n", i)
				}
			}
			if strings.HasPrefix(mode, "crash-once:") && !crashed {
				marker := strings.TrimPrefix(mode, "crash-once:")
				if _, err := os.Stat(marker); errors.Is(err, os.ErrNotExist) {
					_ = os.WriteFile(marker, []byte("crashed"), 0o644)
					crashed = true
					os.Exit(2)
				}
			}
			_, _ = os.Stdout.WriteString("{\"type\":\"event\",\"status\":\"ok\",\"message\":\"event-ok\"}\n")
			continue
		}
		if strings.Contains(line, `"type":"command"`) {
			if mode == "assert-command-instance-config-beyond-supported-deeper-nested-required-enum-default-root-omission-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"command\",\"status\":\"error\",\"error\":\"failed to decode command host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"command\",\"status\":\"error\",\"error\":\"unexpected config field in command host request\"}\n")
					continue
				}
				if _, ok := requestEnvelope["instance_config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"command\",\"status\":\"error\",\"error\":\"unexpected instance_config field for command root omission payload\"}\n")
					continue
				}
				rawCommand, ok := requestEnvelope["command"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"command\",\"status\":\"error\",\"error\":\"missing command payload in host request\"}\n")
					continue
				}
				var commandEnvelope map[string]any
				if err := json.Unmarshal(rawCommand, &commandEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"command\",\"status\":\"error\",\"error\":\"failed to decode command payload\"}\n")
					continue
				}
				commandInvocation, ok := commandEnvelope["command"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"command\",\"status\":\"error\",\"error\":\"missing nested command invocation payload\"}\n")
					continue
				}
				name, ok := commandInvocation["name"].(string)
				if !ok || name != "admin" {
					_, _ = os.Stdout.WriteString("{\"type\":\"command\",\"status\":\"error\",\"error\":\"unexpected command invocation name in payload\"}\n")
					continue
				}
			}
			_, _ = os.Stdout.WriteString("{\"type\":\"command\",\"status\":\"ok\",\"message\":\"command-ok\"}\n")
			continue
		}
		if strings.Contains(line, `"type":"job"`) {
			if mode == "assert-job-instance-config-beyond-supported-deeper-nested-required-enum-default-root-omission-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"job\",\"status\":\"error\",\"error\":\"failed to decode job host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"job\",\"status\":\"error\",\"error\":\"unexpected config field in job host request\"}\n")
					continue
				}
				if _, ok := requestEnvelope["instance_config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"job\",\"status\":\"error\",\"error\":\"unexpected instance_config field for job root omission payload\"}\n")
					continue
				}
				rawJob, ok := requestEnvelope["job"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"job\",\"status\":\"error\",\"error\":\"missing job payload in host request\"}\n")
					continue
				}
				var jobEnvelope map[string]any
				if err := json.Unmarshal(rawJob, &jobEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"job\",\"status\":\"error\",\"error\":\"failed to decode job payload\"}\n")
					continue
				}
				jobInvocation, ok := jobEnvelope["job"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"job\",\"status\":\"error\",\"error\":\"missing nested job invocation payload\"}\n")
					continue
				}
				jobType, ok := jobInvocation["type"].(string)
				if !ok || jobType != "ai.chat" {
					_, _ = os.Stdout.WriteString("{\"type\":\"job\",\"status\":\"error\",\"error\":\"unexpected job invocation type in payload\"}\n")
					continue
				}
			}
			_, _ = os.Stdout.WriteString("{\"type\":\"job\",\"status\":\"ok\",\"message\":\"job-ok\"}\n")
			continue
		}
		if strings.Contains(line, `"type":"schedule"`) {
			if mode == "assert-schedule-instance-config-beyond-supported-deeper-nested-required-enum-default-root-omission-not-enforced" {
				var requestEnvelope map[string]json.RawMessage
				if err := json.Unmarshal([]byte(line), &requestEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"schedule\",\"status\":\"error\",\"error\":\"failed to decode schedule host request envelope\"}\n")
					continue
				}
				if _, ok := requestEnvelope["config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"schedule\",\"status\":\"error\",\"error\":\"unexpected config field in schedule host request\"}\n")
					continue
				}
				if _, ok := requestEnvelope["instance_config"]; ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"schedule\",\"status\":\"error\",\"error\":\"unexpected instance_config field for schedule root omission payload\"}\n")
					continue
				}
				rawSchedule, ok := requestEnvelope["schedule_trigger"]
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"schedule\",\"status\":\"error\",\"error\":\"missing schedule payload in host request\"}\n")
					continue
				}
				var scheduleEnvelope map[string]any
				if err := json.Unmarshal(rawSchedule, &scheduleEnvelope); err != nil {
					_, _ = os.Stdout.WriteString("{\"type\":\"schedule\",\"status\":\"error\",\"error\":\"failed to decode schedule payload\"}\n")
					continue
				}
				scheduleInvocation, ok := scheduleEnvelope["schedule_trigger"].(map[string]any)
				if !ok {
					_, _ = os.Stdout.WriteString("{\"type\":\"schedule\",\"status\":\"error\",\"error\":\"missing nested schedule trigger payload\"}\n")
					continue
				}
				scheduleType, ok := scheduleInvocation["type"].(string)
				if !ok || scheduleType != "cron" {
					_, _ = os.Stdout.WriteString("{\"type\":\"schedule\",\"status\":\"error\",\"error\":\"unexpected schedule trigger type in payload\"}\n")
					continue
				}
			}
			_, _ = os.Stdout.WriteString("{\"type\":\"schedule\",\"status\":\"ok\",\"message\":\"schedule-ok\"}\n")
			continue
		}
	}
}
