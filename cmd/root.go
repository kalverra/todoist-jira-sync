// Package cmd contains command execution logic.
package cmd

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "go-cli-template",
	Short: "Template CLI application in Go",
	Long:  `Template CLI application in Go`,
}

func init() {

}

// Execute runs the root command.
func Execute() {
	if err := fang.Execute(context.Background(), rootCmd); err != nil {
		os.Exit(1)
	}
}
