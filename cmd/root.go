package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:           "covid-tracking",
	Short:         "Tracking and notification platform to assist in the COVID-19 pandemic crisis",
	SilenceErrors: true,
	SilenceUsage:  true,
	Long: `COVID-19 Tracking

Provides an open source and privacy-respecting platform to assist governments and health
institutions in the voluntary tracking and notification of individuals at potential risk
of contagion in the COVID-19 pandemic.

For more information:
https://go.bryk.io/covid-tracking`,
}

// Execute provides the main entry point for the application
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is /etc/covid-tracking/config.yml)")
}

func initConfig() {
	// ENV
	viper.SetEnvPrefix("covid")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set configuration file
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}

	// Read configuration file
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
