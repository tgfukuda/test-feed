/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/tgfukuda/test-feed/util"
)

type Options struct {
	endpoint string
	name     string
	keystore string
	password string
	osm      string
	median   string
}

var errGetAddresses = errors.New("failed to get addresses")

func getAddresses(addressPath string) (map[string]interface{}, error) {
	addressFile, err := os.Open(addressPath)
	if err != nil {
		return nil, util.ChainError(errGetAddresses, err)
	}
	defer addressFile.Close()

	bytes, err := ioutil.ReadAll(addressFile)
	if err != nil {
		return nil, util.ChainError(errGetAddresses, err)
	}

	var addresses map[string]interface{}
	if err := json.Unmarshal([]byte(bytes), &addresses); err != nil {
		return nil, util.ChainError(errGetAddresses, err)
	}

	return addresses, nil
}

const OraclePrefix = "PIP_"

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
	rootCmd.MarkPersistentFlagRequired("keystore")
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
	rootCmd.PersistentFlags().StringVar(
		&opts.osm,
		"osm",
		"./osm/out/OSM.abi",
		"OSM ABI file",
	)

	rootCmd.AddCommand(
		newFeedCommand(opts),
		newPriceCmd(opts),
		newSignCommand(opts),
	)

	return rootCmd
}
