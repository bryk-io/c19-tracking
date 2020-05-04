package cmd

import (
	"os"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.bryk.io/x/cli"
	"go.bryk.io/x/net/rpc"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start a new API server instance",
	RunE:  runServer,
}

func init() {
	params := []cli.Param{
		{
			Name:      "name",
			Usage:     "FQDN use as the main server address and identifier",
			FlagKey:   "server.name",
			ByDefault: "covid-tracking.test",
		},
		{
			Name:      "port",
			Usage:     "TCP port to use for the main RPC server",
			FlagKey:   "server.port",
			ByDefault: 9090,
		},
		{
			Name:      "home",
			Usage:     "Home directory for the server instance",
			FlagKey:   "server.home",
			ByDefault: "/etc/covid-tracking",
		},
		{
			Name:      "storage",
			Usage:     "Storage component endpoint",
			FlagKey:   "server.storage",
			ByDefault: "mongodb://localhost:27017",
		},
		{
			Name:      "broker",
			Usage:     "Message broker endpoint",
			FlagKey:   "server.broker",
			ByDefault: "amqp://localhost:5672",
		},
	}
	if err := cli.SetupCommandParams(serverCmd, params); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(serverCmd)
}

func runServer(_ *cobra.Command, _ []string) error {
	port := viper.GetInt("server.port")
	handler, err := getServerHandler()
	if err != nil {
		return err
	}

	// Setup RPC server
	srvOptions := []rpc.ServerOption{
		rpc.WithNetworkInterface(rpc.NetworkInterfaceAll),
		rpc.WithPort(port),
		rpc.WithInputValidation(),
		rpc.WithPanicRecovery(),
		rpc.WithService(handler.GetServiceDefinition()),
		rpc.WithTLS(handler.TLSConfig()),
		rpc.WithHTTPGateway(handler.HTTPGateway()),
		rpc.WithMonitoring(rpc.MonitoringOptions{
			IncludeHistograms:   true,
			UseGoCollector:      true,
			UseProcessCollector: true,
		}),
		rpc.WithLogger(rpc.LoggingOptions{
			Logger: log,
			FilterMethods: []string{
				"bryk.covid.proto.v1.TrackingServerAPI/Ping",
			},
		}),
	}

	// Start server
	ready := make(chan bool)
	srv, err := rpc.NewServer(srvOptions...)
	if err != nil {
		return err
	}
	go func() {
		if err := srv.Start(ready); err != nil {
			log.Error(err.Error())
		}
	}()

	// Wait for server to be ready
	<-ready
	log.Infof("waiting for requests at port: %d", port)

	// Catch interruption signals and quit
	<-cli.SignalsHandler([]os.Signal{
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		os.Interrupt,
	})
	log.Warning("server closed")
	handler.Close()
	_ = srv.Stop(true)
	return nil
}
