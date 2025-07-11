package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"rag-cli/pkg/version"
)

var cfgFile string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "rag-cli",
	Short: "A CLI tool with RAG capabilities using local models",
	Long: `RAG CLI is a command-line tool that provides AI-powered assistance
using Retrieval-Augmented Generation (RAG) with local language models.
It can process documents, generate embeddings, and interact with vector databases.`,
	Run: func(cmd *cobra.Command, args []string) {
		if versionFlag, _ := cmd.Flags().GetBool("version"); versionFlag {
			fmt.Println(version.GetBuildInfo().String())
			return
		}
		cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.rag-cli.yaml)")
	rootCmd.PersistentFlags().Bool("debug", false, "Enable debug mode")
	rootCmd.Flags().BoolP("version", "v", false, "Print version information")
	
	// Bind flags to viper
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error getting home directory: %v\n", err)
			os.Exit(1)
		}

		// Search config in home directory with name ".rag-cli" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".rag-cli")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
