package cmd

import (
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.bryk.io/covid-tracking/api"
	"go.bryk.io/x/cli"
)

var workerCmd = &cobra.Command{
	Use:   "worker",
	Short: "Start a new worker instance",
	RunE:  runWorker,
	Long: `Worker Process

Workers are independent components responsible for handling potentially
time consuming tasks asynchronously. Workers are stateless and allow
for easy horizontal scaling of the platform.`,
}

func init() {
	params := []cli.Param{
		{
			Name:      "storage",
			Usage:     "Storage component endpoint",
			FlagKey:   "storage",
			ByDefault: "mongodb://localhost:27017",
		},
		{
			Name:      "broker",
			Usage:     "Message broker endpoint",
			FlagKey:   "broker",
			ByDefault: "amqp://localhost:5672",
		},
	}
	if err := cli.SetupCommandParams(workerCmd, params); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(workerCmd)
}

func runWorker(_ *cobra.Command, _ []string) error {
	// Get worker settings
	opts := &api.WorkerOptions{
		Store:  viper.GetString("storage"),
		Broker: viper.GetString("broker"),
		Logger: log,
	}
	if err := viper.UnmarshalKey("resolver", &opts.Providers); err != nil {
		return err
	}

	// Create new worker instance
	worker, err := api.NewWorker(opts)
	if err != nil {
		return nil
	}

	// Catch interruption signals and quit
	<-cli.SignalsHandler([]os.Signal{
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		os.Interrupt,
	})
	log.Warning("worker closed")
	worker.Close()
	return nil
}
