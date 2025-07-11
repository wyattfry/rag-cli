package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var execCmd = &cobra.Command{
	Use:   "exec [command]",
	Short: "Execute shell commands",
	Long:  `Execute shell commands on the local machine. Use with caution.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runExec(args)
	},
}

func init() {
	rootCmd.AddCommand(execCmd)
}

func runExec(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no command specified")
	}

	// Join all arguments into a single command
	cmdStr := strings.Join(args, " ")
	
	fmt.Printf("Executing: %s\n", cmdStr)
	
	// Execute the command
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	
	return cmd.Run()
}
