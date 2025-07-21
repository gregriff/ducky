package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

//go:embed ducky.toml
var defaultConfigFile []byte

func InitConfig(file string) {
	viper.SetConfigName("ducky")
	viper.SetConfigType("toml")
	viper.AddConfigPath(getConfigDir()) // $XDG_HOME_CONFIG takes precedence over config in repo dir
	viper.AddConfigPath("./config")     // in the repo

	if file != "" {
		viper.SetConfigFile(file)
		return
	}

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// create config file from embedded default file
			viper.ReadConfig(bytes.NewBuffer(defaultConfigFile))
			configPath := filepath.Join(getConfigDir(), "ducky.toml")
			if err := os.WriteFile(configPath, defaultConfigFile, 0644); err != nil {
				fmt.Printf("Error writing default config: %v", err)
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
	os.MkdirAll(appConfigDir, 0755)
	return appConfigDir
}
