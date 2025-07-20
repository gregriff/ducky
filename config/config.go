package config

import (
	"bytes"
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

//go:embed gpt-cli-go.toml
var defaultConfigFile []byte

func InitConfig(file string) {
	viper.SetConfigName("gpt-cli-go")
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
			configPath := filepath.Join(getConfigDir(), "gpt-cli-go.toml")
			if err := os.WriteFile(configPath, defaultConfigFile, 0644); err != nil {
				fmt.Printf("Error writing default config: %v", err)
			}
		} else {
			fmt.Println("Error reading config file: ", err)
			os.Exit(1)
		}
	} else {
		log.Println("CONFIG FILE USED: ", viper.ConfigFileUsed())
	}

}

func getConfigDir() string {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		homeDir, _ := os.UserHomeDir()
		configHome = filepath.Join(homeDir, ".config")
	}
	appConfigDir := filepath.Join(configHome, "gpt-cli-go")
	os.MkdirAll(appConfigDir, 0755)
	return appConfigDir
}
