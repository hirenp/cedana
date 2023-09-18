package cmd

import (
	"fmt"
	"net/rpc"
	"os"
	"strconv"

	"github.com/cedana/cedana/api"
	"github.com/cedana/cedana/utils"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var dir string

type CLI struct {
	cfg    *utils.Config
	conn   *rpc.Client
	logger zerolog.Logger
}

func NewCLI() (*CLI, error) {
	cfg, err := utils.InitConfig()
	if err != nil {
		return nil, err
	}
	client, err := rpc.Dial("unix", "/tmp/cedana.sock")
	if err != nil {
		return nil, fmt.Errorf("could not connect to daemon at /tmp/cedana.sock, running as root?: %w", err)
	}
	logger := utils.GetLogger()

	return &CLI{
		cfg:    cfg,
		conn:   client,
		logger: logger,
	}, nil
}

var dumpCmd = &cobra.Command{
	Use:   "dump",
	Short: "Directly checkpoint a running process to a directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cli, err := NewCLI()
		if err != nil {
			return err
		}

		pid, err := strconv.Atoi(args[0])
		if err != nil {
			return err
		}

		if dir == "" {
			if cli.cfg.SharedStorage.DumpStorageDir == "" {
				return fmt.Errorf("no dump directory specified")
			}
			dir = cli.cfg.SharedStorage.DumpStorageDir
		}

		a := api.DumpArgs{
			PID: int32(pid),
			Dir: dir,
		}

		var resp api.DumpResp
		err = cli.conn.Call("CedanaDaemon.Dump", a, &resp)
		if err != nil {
			return err
		}

		return nil
	},
}

var restoreCmd = &cobra.Command{}

var natsCmd = &cobra.Command{
	Use:   "nats",
	Short: "Start NATS server for cedana client",
	RunE: func(cmd *cobra.Command, args []string) error {
		cli, err := NewCLI()
		if err != nil {
			return err
		}

		selfId, exists := os.LookupEnv("CEDANA_CLIENT_ID")
		if !exists {
			cli.logger.Fatal().Msg("Could not find CEDANA_CLIENT_ID - something went wrong during instance creation")
		}

		jobId, exists := os.LookupEnv("CEDANA_JOB_ID")
		if !exists {
			cli.logger.Fatal().Msg("Could not find CEDANA_JOB_ID - something went wrong during instance creation")
		}

		authToken, exists := os.LookupEnv("CEDANA_AUTH_TOKEN")
		if !exists {
			cli.logger.Fatal().Msg("Could not find CEDANA_AUTH_TOKEN - something went wrong during instance creation")
		}

		a := api.StartNATSArgs{
			SelfID:    selfId,
			JobID:     jobId,
			AuthToken: authToken,
		}

		var resp api.StartNATSResp
		err = cli.conn.Call("CedanaDaemon.StartNATS", a, &resp)
		if err != nil {
			return err
		}

		cli.logger.Info().Msgf("NATS client started, waiting for commands sent to job %s", jobId)

		return nil
	},
}

func init() {
	rootCmd.AddCommand(dumpCmd)
	rootCmd.AddCommand(restoreCmd)
	clientDaemonCmd.AddCommand(natsCmd)
	clientDaemonCmd.AddCommand()
	dumpCmd.Flags().StringVarP(&dir, "dir", "d", "", "directory to dump to")
}
