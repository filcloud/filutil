package cmd

import "github.com/spf13/cobra"

var IpldCmd = &cobra.Command{
	Use:   "ipld",
	Short: "Commands for IPLD (InterPlanetary Linked Data)",
	Long:  "IPLD is a set of standards and implementations for creating decentralized data-structures that are universally addressable and linkable.",
}

func init() {
	rootCmd.AddCommand(IpldCmd)
}
