package main

import (
	"fmt"
	"os"

	"github.com/Malayt04/flash/cmd"
)

func  main(){
	if err := cmd.Execute(); err != nil{
		fmt.Println(err)
		os.Exit(1)
	}	
}

