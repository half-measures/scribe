package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeHome points os.UserHomeDir at a temp dir and returns the config path
// inside it. t.Setenv works on mac/linux/windows.
func fakeHome(t *testing.T) string {
	t.Helper()

	tmp := t.TempDir()
	t.Setenv("HOME", tmp)        // unix
	t.Setenv("USERPROFILE", tmp) // windows

	return filepath.Join(tmp, configFileName)
}

// writeConfig drops contents at path, failing the test if it can't.
func writeConfig(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}
}

// runConfigShow invokes the show command with its output captured.
func runConfigShow(t *testing.T) (string, error) {
	t.Helper()

	var buf bytes.Buffer
	configShowCmd.SetOut(&buf)
	configShowCmd.SetErr(&buf)
	t.Cleanup(func() {
		configShowCmd.SetOut(nil)
		configShowCmd.SetErr(nil)
	})

	err := configShowCmd.RunE(configShowCmd, nil)

	return buf.String(), err
}

func TestMaskSecret(t *testing.T) {
	//test the masking feature for API key hiding.
	tests := []struct {
		name   string
		secret string
		want   string
	}{
		{
			name:   "Anthropic key keeps its prefix and last four",
			secret: "sk-ant-api03-abcdefghijklmnop1234",
			want:   "sk-ant-**********************1234",
		},
		{
			name:   "OpenAI key matches the shorter sk- prefix",
			secret: "sk-proj-abcd1234",
			want:   "sk-*********1234",
		},
		{
			name:   "Gemini key",
			secret: "AIzaSyABC123456789",
			want:   "AIza**********6789",
		},
		{
			name:   "Unknown prefix reveals only the last four",
			secret: "randomkey12345",
			want:   "**********2345",
		},
		{
			name:   "Prefix is dropped when it would leave nothing masked",
			secret: "sk-ant-1234",
			want:   "*******1234",
		},
		{
			name:   "Short secret is fully masked",
			secret: "abc",
			want:   "***",
		},
		{
			name:   "Secret exactly as long as the kept suffix is fully masked",
			secret: "abcd",
			want:   "****",
		},
		{
			name:   "Empty secret",
			secret: "",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maskSecret(tt.secret)
			if got != tt.want {
				t.Errorf("maskSecret(%q) = %q, want %q", tt.secret, got, tt.want)
			}
			if tt.secret != "" && got == tt.secret {
				t.Errorf("maskSecret(%q) returned the secret unchanged", tt.secret)
			}
		})
	}
}

func TestConfigFilePath(t *testing.T) {
	want := fakeHome(t)

	got, err := configFilePath()
	if err != nil {
		t.Fatalf("configFilePath() returned error: %v", err)
	}
	if got != want {
		t.Errorf("configFilePath() = %q, want %q", got, want)
	}
}

func TestLoadRawConfig(t *testing.T) {
	t.Run("Missing file returns a nil map and no error", func(t *testing.T) {
		path := fakeHome(t)

		raw, err := loadRawConfig(path)
		if err != nil {
			t.Fatalf("loadRawConfig() on missing file returned error: %v", err)
		}
		if raw != nil {
			t.Errorf("loadRawConfig() on missing file = %v, want nil map", raw)
		}
	})

	t.Run("Existing file is parsed", func(t *testing.T) {
		path := fakeHome(t)
		writeConfig(t, path, "provider: claude\nmodel: claude-opus-4\n")

		raw, err := loadRawConfig(path)
		if err != nil {
			t.Fatalf("loadRawConfig() returned error: %v", err)
		}
		if got := raw["provider"]; got != "claude" {
			t.Errorf("raw[\"provider\"] = %v, want %q", got, "claude")
		}
		if got := raw["model"]; got != "claude-opus-4" {
			t.Errorf("raw[\"model\"] = %v, want %q", got, "claude-opus-4")
		}
	})

	t.Run("Malformed YAML returns an error naming the path", func(t *testing.T) {
		path := fakeHome(t)
		writeConfig(t, path, "provider: [unclosed\n")

		raw, err := loadRawConfig(path)
		if err == nil {
			t.Fatalf("loadRawConfig() on malformed YAML = %v, want error", raw)
		}
		if !strings.Contains(err.Error(), path) {
			t.Errorf("error %q does not mention the config path %q", err, path)
		}
	})
}

// TestConfigShowMasksAPIKey is the regression guard: `config show` must never
// print the raw key, since it lands in scrollback, screenshares and CI logs.
func TestConfigShowMasksAPIKey(t *testing.T) {
	const apiKey = "sk-ant-api03-abcdefghijklmnop1234"

	path := fakeHome(t)
	writeConfig(t, path, "provider: claude\napi_key: "+apiKey+"\nmodel: claude-opus-4\n")

	out, err := runConfigShow(t)
	if err != nil {
		t.Fatalf("config show returned error: %v", err)
	}

	if strings.Contains(out, apiKey) {
		t.Errorf("config show leaked the raw API key:\n%s", out)
	}
	if want := maskSecret(apiKey); !strings.Contains(out, want) {
		t.Errorf("config show output does not contain masked key %q:\n%s", want, out)
	}
}

func TestConfigShowRendersYAML(t *testing.T) {
	path := fakeHome(t)
	writeConfig(t, path, "provider: claude\nmodel: claude-opus-4\n")

	out, err := runConfigShow(t)
	if err != nil {
		t.Fatalf("config show returned error: %v", err)
	}

	// A Go map printed with %v would render as "map[...]" rather than YAML.
	if strings.Contains(out, "map[") {
		t.Errorf("config show printed a Go map instead of YAML:\n%s", out)
	}
	for _, want := range []string{"provider: claude", "model: claude-opus-4", path} {
		if !strings.Contains(out, want) {
			t.Errorf("config show output missing %q:\n%s", want, out)
		}
	}
}

func TestConfigShowWithoutAPIKey(t *testing.T) {
	path := fakeHome(t)
	writeConfig(t, path, "provider: gemini\nmodel: gemini-1.5-flash\n")

	out, err := runConfigShow(t)
	if err != nil {
		t.Fatalf("config show returned error: %v", err)
	}

	if strings.Contains(out, "api_key") {
		t.Errorf("config show invented an api_key field:\n%s", out)
	}
}

func TestConfigShowMissingFile(t *testing.T) {
	path := fakeHome(t)

	out, err := runConfigShow(t)
	if err != nil {
		t.Fatalf("config show on missing file returned error: %v, want nil", err)
	}

	if !strings.Contains(out, "scribe init") {
		t.Errorf("config show on missing file should point at 'scribe init':\n%s", out)
	}
	if !strings.Contains(out, path) {
		t.Errorf("config show on missing file should name the path %q:\n%s", path, out)
	}
	if strings.Contains(out, "map[") {
		t.Errorf("config show on missing file printed an empty Go map:\n%s", out)
	}
}
