package cmd

import (
    "fmt"
	"github.com/Malayt04/flash/pkg/transfer"
    "os"

    "github.com/spf13/cobra"
)

var (
    listenPort string
)

var receiveCmd = &cobra.Command{
    Use:   "receive",
    Short: "Start server to receive files",
    Long: `Start a server that listens for incoming file transfers.
The server will save received files in the current directory.`,
    Run: func(cmd *cobra.Command, args []string) {
        listenAddr := fmt.Sprintf(":%s", listenPort)
        
        // Create server
        server, err := transfer.NewServer(listenAddr)
        if err != nil {
            fmt.Printf("Error creating server: %v\n", err)
            os.Exit(1)
        }
        defer server.Close()
        
        // Start listening
        if err := server.Listen(); err != nil {
            fmt.Printf("Server error: %v\n", err)
            os.Exit(1)
        }
    },
}

func init() {
    receiveCmd.Flags().StringVarP(&listenPort, "port", "p", "8080", "Port to listen on")
    rootCmd.AddCommand(receiveCmd)
}