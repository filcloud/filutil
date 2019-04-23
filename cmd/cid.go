package cmd

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/ipfs/go-cid"
	mbase "github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multihash"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(CidCmd)

	CidCmd.AddCommand(CidParseCmd)
}

var CidCmd = &cobra.Command{
	Use:   "cid",
	Short: "Commands for CID (Content Identifier)",
	Long:  "",
}

var CidParseCmd = &cobra.Command{
	Use:   "parse",
	Short: "",
	Long:  "",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		v := args[0]
		c, err := cid.Decode(v)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		p := c.Prefix()

		if len(v) == 46 && v[:2] == "Qm" {
			fmt.Printf("CIDv0: %s\n", v)
			fmt.Printf("  multihash: %s-%d-%s\n (implicitly: base58btc, cidv0, protobuf)", multihash.Codes[p.MhType], p.MhLength, c.Hash())
		} else {
			fmt.Printf("CIDv1: %s\n", v)
			base, _, _ := mbase.Decode(v)
			hash, _ := multihash.Decode(c.Hash())
			fmt.Printf("  multibase: %s, cid-version: cidv%d, multicodec: %s, multihash: %s-%d-%s\n", strings.ToLower(multibaseNames[base]), p.Version, cid.CodecToStr[p.Codec], multihash.Codes[p.MhType], 8*p.MhLength, hex.EncodeToString(hash.Digest))
		}
	},
}

var multibaseNames = map[mbase.Encoding]string{
	0x00: "Identity",
	'1':  "Base1",
	'0':  "Base2",
	'7':  "Base8",
	'9':  "Base10",
	'f':  "Base16",
	'F':  "Base16Upper",
	'b':  "Base32",
	'B':  "Base32Upper",
	'c':  "Base32pad",
	'C':  "Base32padUpper",
	'v':  "Base32hex",
	'V':  "Base32hexUpper",
	't':  "Base32hexPad",
	'T':  "Base32hexPadUpper",
	'Z':  "Base58Flickr",
	'z':  "Base58BTC",
	'm':  "Base64",
	'u':  "Base64url",
	'M':  "Base64pad",
	'U':  "Base64urlPad",
}
