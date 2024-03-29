package cmd

import (
	"encoding/json"
	"log"
	"math/big"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ethereum/go-ethereum/common/math"
	"github.com/spf13/cobra"
	"github.com/tgfukuda/test-feed/transact"
	"github.com/tgfukuda/test-feed/util"
)

type FeedOption struct {
	interval uint16
}

func newFeedCommand(opts *Options) *cobra.Command {
	subOpts := FeedOption{}
	cmd := feedCommand(opts, &subOpts)
	cmd.Flags().Uint16VarP(
		&subOpts.interval,
		"interval",
		"i",
		3600,
		"interval of each transaction",
	)

	return cmd
}

func feedCommand(opts *Options, subOpts *FeedOption) *cobra.Command {
	return &cobra.Command{
		Use:   "feed",
		Args:  cobra.ExactArgs(1),
		Short: "running feed client",
		Long:  ``,
		RunE: func(_ *cobra.Command, args []string) (err error) {
			addressPath := args[0]
			addresses, err := getAddresses(addressPath)
			if err != nil {
				return err
			}

			privKey, err := transact.GetPrivFromFile(opts.keystore, opts.password)
			if err != nil {
				return err
			}

			osm, ok := addresses[OraclePrefix+opts.name].(string)
			if !ok {
				return util.ErrCast
			}

			logger := log.Default()

			oracle, err := transact.New(opts.endpoint, privKey, osm, opts.osm, opts.median, logger)
			if err != nil {
				return err
			}

			trap := make(chan os.Signal, 1)
			signal.Notify(trap, syscall.SIGTERM, syscall.SIGINT, os.Interrupt)

			quit := make(chan bool, 1)

			feed := func() {
				tx, err := oracle.Poke(func(ts time.Time) *big.Int {
					price, _ := math.ParseBig256("100000000000000000000000")
					return price
				})
				if tx != nil {
					logger.Printf("[INFO] sent transaction %s", tx.Hash().Hex())
				}
				if err != nil {
					logger.Println(err)
				}
				s, _ := json.MarshalIndent(tx, "", "\t")
				logger.Printf("%s\n", s)
			}

			feed()

			go func() {
				defer close(quit)
				for {
					select {
					case <-trap:
						logger.Printf("closing session...\n")
						quit <- true
						return
					case <-time.After(time.Duration(subOpts.interval) * time.Second):
						feed()
					}
				}
			}()

			<-quit

			return oracle.Delete()
		},
	}
}
