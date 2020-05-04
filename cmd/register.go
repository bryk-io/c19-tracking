package cmd

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	protov1 "go.bryk.io/covid-tracking/proto/v1"
	"go.bryk.io/covid-tracking/utils"
	"go.bryk.io/x/ccg/did"
	"go.bryk.io/x/cli"
)

var registerCmd = &cobra.Command{
	Use:     "credentials",
	RunE:    runRegister,
	Aliases: []string{"register"},
	Short:   "Create new account credentials",
	Long: `Manually create account credentials

This mechanism bypass the regular registration process available through
the public API. It should ONLY be used to generate the initial administrator
credentials for a new API server.`,
}

func init() {
	params := []cli.Param{
		{
			Name:      "role",
			Usage:     "Account role to register (admin, agent, user)",
			FlagKey:   "register.role",
			ByDefault: "admin",
		},
		{
			Name:      "did",
			Usage:     "Account's DID",
			FlagKey:   "register.did",
			ByDefault: "",
		},
		{
			Name:      "code",
			Usage:     "Activation code used to register the account",
			FlagKey:   "register.code",
			ByDefault: "",
		},
		{
			Name:      "proof",
			Usage:     "JSON LD document containing the signed activation code",
			FlagKey:   "register.proof",
			ByDefault: "",
		},
	}
	if err := cli.SetupCommandParams(registerCmd, params); err != nil {
		panic(err)
	}
	rootCmd.AddCommand(registerCmd)
}

func runRegister(_ *cobra.Command, _ []string) error {
	// Get registration parameters
	id := strings.TrimSpace(viper.GetString("register.did"))
	if id == "" {
		utils.ReadInput("Account DID", &id)
	}
	if _, err := did.Parse(id); err != nil {
		return errors.New("invalid DID")
	}
	role := strings.TrimSpace(viper.GetString("register.role"))
	if role == "" {
		utils.ReadInput("Account role", &role)
	}
	code := strings.TrimSpace(viper.GetString("register.code"))
	if code == "" {
		utils.ReadInput("Activation code", &code)
	}
	if _, err := uuid.Parse(code); err != nil {
		return errors.New("invalid activation code")
	}
	proofFile := strings.TrimSpace(viper.GetString("register.proof"))
	if proofFile == "" {
		utils.ReadInput("Proof file", &proofFile)
	}
	proof, err := ioutil.ReadFile(filepath.Clean(proofFile))
	if err != nil {
		return err
	}

	// Get service handler
	handler, err := getServerHandler()
	if err != nil {
		return err
	}
	defer handler.Close()

	// Request account credentials
	req := &protov1.CredentialsRequest{
		Did:            id,
		Role:           role,
		ActivationCode: code,
		Proof:          proof,
	}
	credentials, err := handler.AccessToken(req, false)
	if err != nil {
		return errors.Wrap(err, "get credentials")
	}

	// Save generated credentials and exit
	m := jsonpb.Marshaler{
		EnumsAsInts:  false,
		EmitDefaults: true,
		OrigName:     true,
	}
	output, err := m.MarshalToString(credentials)
	if err != nil {
		return err
	}
	fmt.Printf("%s", output)
	return nil
}
