package cmd

import (
	"bytes"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.bryk.io/covid-tracking/api"
	protov1 "go.bryk.io/covid-tracking/proto/v1"
	"go.bryk.io/x/cli"
	"go.bryk.io/x/cli/shell"
	"go.bryk.io/x/net/rpc"
)

var clientCmd = &cobra.Command{
	Use:     "client",
	Short:   "Start an interactive CLI-based client",
	Example: "client server.com:443 --credentials ~/.covid-tracking.json",
	RunE:    runClient,
}

func init() {
	params := []cli.Param{
		{
			Name:      "credentials",
			Usage:     "Credentials file to use",
			FlagKey:   "client.credentials",
			ByDefault: "credentials.json",
		},
		{
			Name:      "insecure",
			Usage:     "Accept any certificate presented. Dangerous, for development only",
			FlagKey:   "client.insecure",
			ByDefault: false,
		},
	}
	if err := cli.SetupCommandParams(clientCmd, params); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(clientCmd)
}

func runClient(_ *cobra.Command, args []string) error {
	// Get server endpoint
	if len(args) == 0 {
		return errors.New("you must specify the server endpoint")
	}
	endpoint := args[0]

	// Open credentials file
	location := filepath.Clean(viper.GetString("client.credentials"))
	contents, err := ioutil.ReadFile(filepath.Clean(location))
	if err != nil {
		return errors.Wrap(err, "failed to open credentials file")
	}
	credentials := &protov1.CredentialsResponse{}
	if err = jsonpb.Unmarshal(bytes.NewReader(contents), credentials); err != nil {
		return errors.Wrap(err, "failed to decode credentials content")
	}

	// Client configuration
	clOpts := []rpc.ClientOption{
		rpc.WaitForReady(),
		rpc.WithTimeout(5 * time.Second),
		rpc.WithCompression(),
		rpc.WithClientTLS(rpc.ClientTLSConfig{IncludeSystemCAs: true}),
		rpc.WithUserAgent("cli-client/0.1.0"),
		rpc.WithAuthToken(credentials.AccessToken),
	}
	if viper.GetBool("client.insecure") {
		log.Warning("insecure client connection")
		clOpts = append(clOpts, rpc.WithInsecureSkipVerify())
	}

	// Open connection
	log.WithField("endpoint", endpoint).Debug("contacting server")
	conn, err := rpc.NewClientConnection(endpoint, clOpts...)
	if err != nil {
		return err
	}
	log.Info("connection ready")

	// Start interactive client
	cl := protov1.NewTrackingServerAPIClient(conn)
	sh, err := shell.New()
	if err != nil {
		return errors.Wrap(err, "failed to start shell instance")
	}
	for _, cmd := range api.GetShellCommands(sh, cl) {
		sh.AddCommand(cmd)
	}
	sh.Start()

	// Close connection
	log.Info("closing client")
	return conn.Close()
}
