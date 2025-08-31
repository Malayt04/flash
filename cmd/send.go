package cmd

import (
	"fmt"
	"os"

	"github.com/Malayt04/flash/pkg/transfer"
	"github.com/spf13/cobra"
)
var sendCmd = &cobra.Command{
	Use:   "send [file] [server_address]",
	Short: "Send a file to a remote server",
	Long: `Send a file to a remote server using the reliable UDP protocol.
The server address should be in the format host:port (e.g., 192.168.1.100:8080)`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string){
		filePath := args[0]
		serverAddress := args[1]

		if _, err := os.Stat(filePath); os.IsNotExist(err){
			fmt.Printf("File %s does not exist\n", filePath)
			os.Exit(1)
		}

		client, err := transfer.NewClient(serverAddress)

		if err != nil{
			fmt.Printf("Error creating client: %v\n", err)
			os.Exit(1)
		}

		defer client.Close()
		
				if err := client.SendFile(filePath); err != nil {
			fmt.Printf("Error sending file: %v\n", err)
			os.Exit(1)
		}
	
	},
}


func init(){
	rootCmd.AddCommand(sendCmd)
}