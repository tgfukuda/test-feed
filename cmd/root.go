/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"github.com/spf13/cobra"
)

type Options struct {
	interval uint16
	endpoint string
	name     string
	keystore string
	password string
	osm      string
	median   string
}

func NewRootCommand(opts *Options) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:           "test-feed",
		Short:         "feeder for testing",
		Long:          "this client is for testing. DO NOT USE in production.",
		SilenceErrors: false,
		SilenceUsage:  true,
	}

	rootCmd.PersistentFlags().Uint16VarP(
		&opts.interval,
		"interval",
		"i",
		3600,
		"interval of each transaction",
	)
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
		"f",
		"~/.dapp/testnet/8545/keystore/",
		"keystore json path",
	)
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
	)

	return rootCmd
}
