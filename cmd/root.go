package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.bryk.io/covid-tracking/server"
	"go.bryk.io/x/ccg/did"
	xlog "go.bryk.io/x/log"
)

var log xlog.Logger
var cfgFile string

var rootCmd = &cobra.Command{
	Use:           "covid-tracking",
	Short:         "Tracking and notification platform to assist in the COVID-19 pandemic crisis",
	SilenceErrors: true,
	SilenceUsage:  true,
	Long: `COVID-19 Tracking

Open platform to assist governments and health organizations
in the voluntary and privacy-respecting tracking and notification
of individuals at potential risk of contagion for COVID-19.

For more information:
https://go.bryk.io/covid-tracking`,
}

// Execute provides the main entry point for the application
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Error(err)
		os.Exit(1)
	}
}

func init() {
	log = xlog.WithZero(true)
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is /etc/covid-tracking/config.yml)")
}

func initConfig() {
	// ENV
	viper.SetEnvPrefix("ct19")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Set configuration file
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	}

	// Read configuration file
	if err := viper.ReadInConfig(); err != nil {
		log.WithField("error", err).Error("failed to read configuration file")
	}
}

func getServerHandler() (*server.Handler, error) {
	// Get parameters
	name := viper.GetString("server.name")
	home := viper.GetString("server.home")
	port := viper.GetInt("server.port")
	store := viper.GetString("server.storage")

	// Get resolver settings
	var providers []*did.Provider
	if err := viper.UnmarshalKey("resolver", &providers); err != nil {
		return nil, err
	}

	// Prepare server handler
	return server.NewHandler(name, home, store, port, providers)
}
