package cmd

import (
	"errors"
	"fmt"
	"log"

	"github.com/spf13/cobra"
	"github.com/tgfukuda/test-feed/transact"
)

type PriceOption struct {
	direct bool
}

func newPriceCmd(opts *Options) *cobra.Command {
	subOpts := PriceOption{}
	cmd := priceCmd(opts, &subOpts)
	cmd.Flags().BoolVarP(
		&subOpts.direct,
		"direct",
		"d",
		false,
		"get price from median. need to be authorized.",
	)

	return cmd
}

func priceCmd(opts *Options, subOpts *PriceOption) *cobra.Command {
	return &cobra.Command{
		Use:   "price",
		Args:  cobra.ExactArgs(1),
		Short: "",
		Long:  ``,
		RunE: func(_ *cobra.Command, args []string) (err error) {
			addressPath := args[0]
			addresses, err := getAddresses(addressPath)
			if err != nil {
				fmt.Println("[ERROR] failed to parse addresses")
				return err
			}

			privKey, err := transact.GetPrivFromFile(opts.keystore, opts.password)
			if err != nil {
				fmt.Println("[ERROR] failed to initialize priv key")
				return err
			}

			osm, ok := addresses[OraclePrefix+opts.name].(string)
			if !ok {
				fmt.Println("[ERROR] invalid median address")
				return errors.New("failure in casting address to string")
			}

			logger := log.Default()

			oracle, err := transact.New(opts.endpoint, privKey, osm, opts.osm, opts.median, logger)
			if err != nil {
				fmt.Println("[ERROR] failed to initialize oracle")
				return err
			}

			if subOpts.direct {
				price, err := oracle.GetMedianPrice()
				if err != nil {
					fmt.Println("[ERROR] failed to get median price")
					return err
				}
				logger.Printf(`
					price: %d
				`, price)
			} else {
				curr, next, err := oracle.GetOsmPrice()
				if err != nil {
					fmt.Println("[ERROR] failed to get osm price")
					return err
				}
				logger.Printf(`
					current: %d,
					next   : %d   
				`, curr, next)
			}

			return oracle.Delete()
		},
	}
}
