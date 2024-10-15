package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cometbft/cometbft/p2p/nodekey"
)

// ShowNodeIDCmd dumps node's ID to the standard output.
var ShowNodeIDCmd = &cobra.Command{
	Use:     "show-node-id",
	Aliases: []string{"show_node_id"},
	Short:   "Show this node's ID",
	RunE:    showNodeID,
}

func showNodeID(*cobra.Command, []string) error {
	nk, err := nodekey.LoadNodeKey(config.NodeKeyFile())
	if err != nil {
		return err
	}

	fmt.Println(nk.ID())
	return nil
}
