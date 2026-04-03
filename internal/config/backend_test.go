package config

import (
	"reflect"
	"testing"
	"time"
)

func TestProtectedAPI_HasExpectedFields(t *testing.T) {
	// Verify fields that should exist
	rt := reflect.TypeOf(ProtectedAPI{})

	expectedFields := []struct {
		name     string
		typeName string
	}{
		{"URL", "string"},
		{"RequestHostHeader", "string"},
		{"InsecureConnection", "bool"},
		{"RootCA", "string"},
		{"MaxConnsPerHost", "int"},
		{"ReadTimeout", "Duration"},
		{"WriteTimeout", "Duration"},
		{"DialTimeout", "Duration"},
		{"ReadBufferSize", "int"},
		{"WriteBufferSize", "int"},
		{"MaxResponseBodySize", "int"},
		{"DeleteAcceptEncoding", "bool"},
		{"HealthCheckInterval", "Duration"},
		{"MaxIdleConnDuration", "Duration"},
	}

	for _, ef := range expectedFields {
		field, ok := rt.FieldByName(ef.name)
		if !ok {
			t.Errorf("expected field %q to exist on ProtectedAPI", ef.name)
			continue
		}
		if field.Type.Name() != ef.typeName {
			t.Errorf("field %q: expected type %s, got %s", ef.name, ef.typeName, field.Type.Name())
		}
	}
}

func TestProtectedAPI_RemovedFields(t *testing.T) {
	// Verify fields that were removed no longer exist
	rt := reflect.TypeOf(ProtectedAPI{})

	removedFields := []string{
		"ClientPoolCapacity",
		"UsePoolV2",
		"PoolV2HealthCheckPeriod",
	}

	for _, name := range removedFields {
		if _, ok := rt.FieldByName(name); ok {
			t.Errorf("field %q should have been removed from ProtectedAPI", name)
		}
	}
}

func TestProtectedAPI_Defaults(t *testing.T) {
	// Verify zero-value struct has expected zero values
	// (actual defaults are applied by the conf library at parse time,
	// but we can verify the struct tags are present)
	rt := reflect.TypeOf(ProtectedAPI{})

	expectedDefaults := map[string]string{
		"MaxConnsPerHost":     "default:512",
		"ReadTimeout":         "default:5s",
		"WriteTimeout":        "default:5s",
		"DialTimeout":         "default:200ms",
		"ReadBufferSize":      "default:8192",
		"WriteBufferSize":     "default:8192",
		"MaxResponseBodySize": "default:0",
		"HealthCheckInterval": "default:30s",
		"MaxIdleConnDuration": "default:10s",
		"InsecureConnection":  "default:false",
	}

	for fieldName, expectedTag := range expectedDefaults {
		field, ok := rt.FieldByName(fieldName)
		if !ok {
			t.Errorf("field %q not found", fieldName)
			continue
		}
		confTag := field.Tag.Get("conf")
		if confTag != expectedTag {
			t.Errorf("field %q: expected conf tag %q, got %q", fieldName, expectedTag, confTag)
		}
	}
}

func TestProtectedAPI_ZeroValue(t *testing.T) {
	var cfg ProtectedAPI

	// Zero value should have zero durations
	if cfg.HealthCheckInterval != 0 {
		t.Errorf("expected zero HealthCheckInterval, got %v", cfg.HealthCheckInterval)
	}
	if cfg.MaxIdleConnDuration != 0 {
		t.Errorf("expected zero MaxIdleConnDuration, got %v", cfg.MaxIdleConnDuration)
	}
	if cfg.InsecureConnection {
		t.Error("expected InsecureConnection to be false")
	}

	// Verify types work correctly when set
	cfg.HealthCheckInterval = 30 * time.Second
	cfg.MaxIdleConnDuration = 10 * time.Second
	if cfg.HealthCheckInterval != 30*time.Second {
		t.Errorf("unexpected HealthCheckInterval: %v", cfg.HealthCheckInterval)
	}
}
