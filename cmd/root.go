package cmd

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.bryk.io/covid-tracking/api"
	xlog "go.bryk.io/x/log"
)

var log xlog.Logger
var cfgFile string

var rootCmd = &cobra.Command{
	Use:           "ct19",
	Short:         "Tracking and notification platform to assist in the COVID-19 pandemic crisis",
	SilenceErrors: true,
	SilenceUsage:  true,
	Long: `COVID-19 Contact Tracing

Open platform to assist governments and health organizations
in the voluntary and privacy-respecting tracking and notification
of individuals at potential risk of contagion for COVID-19.

For more information:
https://github.com/bryk-io/ct19`,
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is /etc/ct19/config.yml)")
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

func getServerHandler() (*api.Server, error) {
	// API server options
	opts := &api.ServerOptions{
		Name:   viper.GetString("server.name"),
		Home:   viper.GetString("server.home"),
		Store:  viper.GetString("storage"),
		Broker: viper.GetString("broker"),
		Logger: log,
	}

	// Get resolver settings
	if err := viper.UnmarshalKey("resolver", &opts.Providers); err != nil {
		return nil, err
	}

	// Prepare server handler
	return api.NewServer(opts)
}
