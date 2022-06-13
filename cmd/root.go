/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
)

type Options struct {
	endpoint string
	name     string
	keystore string
	password string
	osm      string
	median   string
}

func getAddresses(addressPath string) (map[string]interface{}, error) {
	addressFile, err := os.Open(addressPath)
	if err != nil {
		fmt.Println("[ERROR] failed to open address file")
		return nil, err
	}
	defer addressFile.Close()

	bytes, err := ioutil.ReadAll(addressFile)
	if err != nil {
		fmt.Println("[ERROR] failed to read address file")
		return nil, err
	}

	var addresses map[string]interface{}
	if err := json.Unmarshal([]byte(bytes), &addresses); err != nil {
		fmt.Println("[ERROR] failed to parse json")
		return nil, err
	}

	return addresses, nil
}

func NewRootCommand(opts *Options) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "test-feed",
		Short:         "feeder for testing",
		Long:          "this client is for testing. DO NOT USE in production.",
		SilenceErrors: false,
		SilenceUsage:  true,
	}

	rootCmd.PersistentFlags().StringVarP(
		&opts.name,
		"token",
		"t",
		"JPX",
		"target token name",
	)
	rootCmd.PersistentFlags().StringVar(
		&opts.endpoint,
		"endpoint",
		"http://127.0.0.1:8545",
		"rpc server",
	)
	rootCmd.PersistentFlags().StringVarP(
		&opts.keystore,
		"keystore",
		"k",
		"~/.dapp/testnet/8545/keystore",
		"keystore json path",
	)
	rootCmd.MarkPersistentFlagFilename("keystore")
	rootCmd.PersistentFlags().StringVarP(
		&opts.password,
		"password",
		"p",
		"/dev/null",
		"password file",
	)
	rootCmd.PersistentFlags().StringVar(
		&opts.median,
		"median",
		"./median/out/MedianJPXJPY.abi",
		"Median ABI file",
	)
	rootCmd.MarkPersistentFlagRequired("median")
	rootCmd.PersistentFlags().StringVar(
		&opts.osm,
		"osm",
		"./osm/out/OSM.abi",
		"OSM ABI file",
	)
	rootCmd.MarkPersistentFlagRequired("osm")

	rootCmd.AddCommand(
		newFeedCommand(opts),
		newPriceCmd(opts),
	)

	return rootCmd
}
