// Package config contains the logic to obtain app configuration from a file or the environment
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "embed" // used to embed the default application config file.

	"github.com/spf13/viper"
)

//go:embed ducky.toml
var defaultConfigFile []byte

// InitConfig initializes the app config with Viper from the environment, a specified file, or a default file.
func InitConfig(file string) {
	viper.SetConfigName("ducky")
	viper.SetConfigType("toml")
	viper.AddConfigPath(getConfigDir()) // $XDG_HOME_CONFIG takes precedence over config in repo dir
	viper.AddConfigPath("./config")     // in the repo

	// allow env vars to override config file
	viper.SetEnvPrefix("ducky")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv()

	if file != "" {
		viper.SetConfigFile(file)
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// create config file from embedded default file
			if err := viper.ReadConfig(bytes.NewBuffer(defaultConfigFile)); err != nil {
				fmt.Printf("Error reading default config file at path: %s", defaultConfigFile)
				os.Exit(1)
			}
			configPath := filepath.Join(getConfigDir(), "ducky.toml")
			if err := os.WriteFile(configPath, defaultConfigFile, 0o600); err != nil {
				fmt.Printf("Error writing default config: %v", err)
				os.Exit(1)
			}
		} else {
			fmt.Println("Error reading config file: ", err)
			os.Exit(1)
		}
	}
}

func getConfigDir() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, _ := os.UserHomeDir()
		configHome = filepath.Join(homeDir, ".config")
	}
	appConfigDir := filepath.Join(configHome, "ducky")
	if err := os.MkdirAll(appConfigDir, 0o750); err != nil {
		fmt.Printf("Error creating application config file at this location: %s", appConfigDir)
		os.Exit(1)
	}
	return appConfigDir
}
