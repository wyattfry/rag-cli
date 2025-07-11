package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"rag-cli/pkg/version"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version information",
	Long:  `Print the version information including build details.`,
	Run: func(cmd *cobra.Command, args []string) {
		outputJSON, _ := cmd.Flags().GetBool("json")
		
		buildInfo := version.GetBuildInfo()
		
		if outputJSON {
			jsonOutput, err := json.MarshalIndent(buildInfo, "", "  ")
			if err != nil {
				fmt.Printf("Error marshaling version info: %v\n", err)
				return
			}
			fmt.Println(string(jsonOutput))
		} else {
			fmt.Println(buildInfo.String())
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
	
	// Add JSON output flag
	versionCmd.Flags().BoolP("json", "j", false, "Output version information in JSON format")
}
