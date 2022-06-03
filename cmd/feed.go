package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/tgfukuda/test-feed/transact"
)

const OraclePrefix = "PIP_"

func getAddresses(addressPath string) (map[string]interface{}, error) {
	addressFile, err := os.Open(addressPath)
	if err != nil {
		fmt.Println("[Error] failed to open address file")
		return nil, err
	}
	defer addressFile.Close()

	bytes, err := ioutil.ReadAll(addressFile)
	if err != nil {
		fmt.Println("[Error] failed to read address file")
		return nil, err
	}

	var addresses map[string]interface{}
	if err := json.Unmarshal([]byte(bytes), &addresses); err != nil {
		fmt.Println("[Error] failed to parse json")
		return nil, err
	}

	return addresses, nil
}

func newFeedCommand(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "feed",
		Args:  cobra.ExactArgs(1),
		Short: "",
		Long:  ``,
		RunE: func(_ *cobra.Command, args []string) (err error) {
			addressPath := args[0]
			addresses, err := getAddresses(addressPath)

			privKey, err := transact.GetPrivFromFile(opts.keystore, opts.password)
			if err != nil {
				fmt.Println("[Error] failed to initialize priv key")
				return err
			}

			osm, ok := addresses[OraclePrefix+opts.name].(string)
			if !ok {
				fmt.Println("[Error] invalid median address")
				return err
			}

			logger := log.Default()

			oracle, err := transact.New(opts.endpoint, privKey, osm, opts.osm, opts.median, logger)
			if err != nil {
				fmt.Println("[Error] failed to initialize oracle")
				return err
			}

			for {
				tx, err := oracle.Poke(func(ts time.Time) int64 { return 100 })
				if err != nil {
					logger.Println(err)
				} else {
					logger.Printf("[INFO] successfully sent transaction %s\n", tx.Hash().Hex())
				}
				time.Sleep(time.Duration(opts.interval) * time.Second)
			}
		},
	}
}
