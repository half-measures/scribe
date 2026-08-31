package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Scribe configuration",
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Shows current config if any",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// action to be taken, in this case account for no file returned, or return the file.
		//1. print current active config source file

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to find home directory: %w", err)
		}

		configPath := filepath.Join(home, configFileName) // full file name now at this location using const in init.go

		fmt.Printf("using config file at path:%s\n", configPath) //Print config path
		//now get open config file itself

		file, err := os.ReadFile(configPath)
		if errors.Is(err, os.ErrNotExist) {
			fmt.Fprintf(cmd.OutOrStdout(), " No config file at %s. Run 'scribe init' to create one.\n", configPath)
			return nil
		}
		if err != nil {
			return fmt.Errorf("Failed to read file at path\n%s,  %w", configPath, err)
		}
		fmt.Print("\n", string(file)) //Return config of file found at path.

		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to find home directory: %w", err)
		}

		configPath := filepath.Join(home, ".scribe.yaml")

		v := viper.New()
		v.SetConfigFile(configPath)
		v.SetConfigType("yaml")

		fileExists := false
		if _, err := os.Stat(configPath); err == nil {
			fileExists = true
			if err := v.ReadInConfig(); err != nil {
				return fmt.Errorf("failed to read existing config: %w", err)
			}
		}

		if key == "api_key" {
			value = strings.TrimSpace(value)
			if value == "" {
				return fmt.Errorf("API key cannot be empty")
			}
			if warning := apiKeyFormatWarning(v.GetString("provider"), value); warning != "" {
				fmt.Println(warning)
			}
		}

		v.Set(key, value)

		if fileExists {
			if err := v.WriteConfig(); err != nil {
				return fmt.Errorf("failed to update config file: %w", err)
			}
		} else {
			if err := v.WriteConfigAs(configPath); err != nil {
				return fmt.Errorf("failed to create config file: %w", err)
			}
		}

		fmt.Printf("✔ Updated %s = %s\n", key, value)
		return nil
	},
}

// apiKeyPrefixes maps known providers to the prefix their API keys are
// documented to use, so a mistyped or wrong-provider key can be flagged.
var apiKeyPrefixes = map[string]string{
	"openai":    "sk-",
	"claude":    "sk-ant-",
	"anthropic": "sk-ant-",
	"gemini":    "AIza",
}

// apiKeyFormatWarning returns a friendly warning if value doesn't look like a
// key for provider, or "" if the provider is unknown/unset or the key matches.
func apiKeyFormatWarning(provider, value string) string {
	prefix, ok := apiKeyPrefixes[strings.ToLower(strings.TrimSpace(provider))]
	if !ok || strings.HasPrefix(value, prefix) {
		return ""
	}
	return fmt.Sprintf("⚠ Warning: %s API keys usually start with %q — double-check this value", provider, prefix)
}

func init() {
	configCmd.AddCommand(configSetCmd)
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configShowCmd)
}
