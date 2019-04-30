package cmd

import (
	"encoding/hex"
	"fmt"
	"github.com/filecoin-project/go-filecoin/paths"
	keystore "github.com/ipfs/go-ipfs-keystore"
	crypto "github.com/libp2p/go-libp2p-crypto"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
)

func init() {
	rootCmd.AddCommand(KeystoreCmd)

	KeystoreCmd.AddCommand(KeystoreLsCmd)
}

var KeystoreCmd = &cobra.Command{
	Use:   "keystore",
	Short: "Commands for filecoin keystore",
	Long:  "",
}

var KeystoreLsCmd = &cobra.Command{
	Use:   "ls",
	Short: "List keys in filecoin keystore",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		defer func() {
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}()

		repoDir = paths.GetRepoPath(repoDir)
		repoDir, err = homedir.Expand(repoDir)
		if err != nil {
			return
		}

		ksp := filepath.Join(repoDir, "keystore")
		ks, err := keystore.NewFSKeystore(ksp)
		if err != nil {
			return
		}

		identifiers, err := ks.List()
		if err != nil {
			return
		}

		for _, id := range identifiers {
			var privKey crypto.PrivKey
			privKey, err = ks.Get(id)
			if err != nil {
				return
			}

			t := privKey.Type()
			var pv, pb []byte
			pv, err = privKey.Raw()
			if err != nil {
				return
			}
			pb, err = privKey.GetPublic().Raw()
			if err != nil {
				return
			}
			fmt.Printf("%s: %s %s, %s %s, %s %s\n", red(id), blue("type"), t, blue("private key"), hex.EncodeToString(pv), blue("public key"), hex.EncodeToString(pb))
		}
	},
}
