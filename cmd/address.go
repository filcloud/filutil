package cmd

import (
	"encoding/hex"
	"fmt"
	"github.com/filecoin-project/go-leb128"
	"os"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(AddressCmd)

	AddressCmd.AddCommand(AddressParseCmd)
}

var AddressCmd = &cobra.Command{
	Use:   "address",
	Short: "Commands for filecoin address",
	Long:  "",
}

var AddressParseCmd = &cobra.Command{
	Use:   "parse",
	Short: "Parse and show parts of filecoin address",
	Long:  "",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		addr, err := address.NewFromString(args[0])
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		var network string
		if string(args[0][0]) == address.MainnetPrefix {
			network = "Mainnet"
		} else {
			network = "Testnet"
		}

		var protocol string
		switch addr.Protocol() {
		case address.ID:
			protocol = "ID"
		case address.SECP256K1:
			protocol = "SECP256K1"
		case address.Actor:
			protocol = "Actor"
		case address.BLS:
			protocol = "BLS"
		}

		payload := hex.EncodeToString(addr.Payload())

		fmt.Printf("Address: %s\n", args[0])
		fmt.Printf("  network: %s, protocol: %s, payload: %s", network, protocol, payload)
		var addrStr string
		if addr.Protocol() != address.ID {
			checksum := address.Checksum(append([]byte{addr.Protocol()}, addr.Payload()...))
			fmt.Printf(", checksum: %s", hex.EncodeToString(checksum))

			addrStr = string(args[0][0]) + fmt.Sprintf("%d", addr.Protocol()) + address.AddressEncoding.WithPadding(-1).EncodeToString(append(addr.Payload(), checksum[:]...))
		} else {
			id := leb128.ToUInt64(addr.Payload())
			fmt.Printf("\n  ID: %d", id)

			addrStr = string(args[0][0]) + fmt.Sprintf("%d", addr.Protocol()) + fmt.Sprintf("%d", leb128.ToUInt64(addr.Payload()))
		}
		if addrStr != args[0] {
			panic("invalid address")
		}
		fmt.Println()
	},
}
