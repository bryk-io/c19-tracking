package api

import (
	"context"
	"fmt"

	"github.com/gogo/protobuf/types"
	protov1 "go.bryk.io/covid-tracking/proto/v1"
	"go.bryk.io/x/cli/shell"
)

// GetShellCommands return the shell commands available when using a
// CLI client to interact with a server handler instance.
func GetShellCommands(sh *shell.Instance, cl protov1.TrackingServerAPIClient) []*shell.Command {
	var commands []*shell.Command

	// Clear
	commands = append(commands, &shell.Command{
		Name:        "clear",
		Description: "Clear screen",
		Run: func(_ string) string {
			sh.Clear()
			return ""
		},
	})

	// Ping
	commands = append(commands, &shell.Command{
		Name:        "ping",
		Description: "Send a reachability test to the server",
		Run: func(_ string) string {
			r, err := cl.Ping(context.TODO(), &types.Empty{})
			if err != nil {
				return fmt.Sprintf("error: %s", err)
			}
			return fmt.Sprintf("ping status: %v", r.Ok)
		},
	})

	return commands
}
