/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package main

import (
	"os"

	"github.com/tgfukuda/test-feed/cmd"
)

func main() {
	opts := cmd.Options{}
	rootCmd := cmd.NewRootCommand(&opts)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
