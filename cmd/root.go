package cmd

import "github.com/spf13/cobra"

var rootCmd = &cobra.Command{
	Use: "flash",
	Short: "High speed, long distance file transfer using reliable UDP",
	Long: `A fast and reliable file transfer application built with Go.
Uses a custom RUDP protocol with NACK-based retransmission for optimal performance
over long-distance, high-latency networks.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init(){
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}