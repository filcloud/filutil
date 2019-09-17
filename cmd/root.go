package cmd

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var repoDir string
var filutilDir string

const filutilDirEnvVar = "FILUTIL_DIR"
const defaultFilutilDir = "~/.filutil"

func getFilutilDir() string {
	var dir string
	if filutilDir != "" {
		dir = filutilDir // command line flag
	} else {
		dir = os.Getenv(filutilDirEnvVar) // environment variable
		if dir == "" {
			dir = defaultFilutilDir // default
		}
	}
	dir, err := homedir.Expand(dir)
	if err != nil {
		panic(err)
	}
	return dir
}

func init() {
	rootCmd.PersistentFlags().StringVar(&repoDir, "repodir", "", "The directory of the filecoin repo")
	rootCmd.PersistentFlags().StringVar(&filutilDir, "filutildir", "", "The directory of the filutil metadata")
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "filutil",
	Short: "filutil - Command line utility tool for Filecoin/IPFS",
	Long:  ``,
}

// Execute adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var black = color.New(color.FgBlack).SprintFunc()
var red = color.New(color.FgRed).SprintFunc()
var green = color.New(color.FgGreen).SprintFunc()
var yellow = color.New(color.FgYellow).SprintFunc()
var blue = color.New(color.FgBlue).SprintFunc()
var magenta = color.New(color.FgMagenta).SprintFunc()
var cyan = color.New(color.FgCyan).SprintFunc()
var white = color.New(color.FgWhite).SprintFunc()
