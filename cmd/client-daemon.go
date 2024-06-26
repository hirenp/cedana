package cmd

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/cedana/cedana/api"
	"github.com/cedana/cedana/utils"
	"github.com/spf13/cobra"
)

var clientDaemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Start daemon for cedana client. Must be run as root, needed for all other cedana functionality.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return fmt.Errorf("missing subcommand")
	},
}

var startDaemonCmd = &cobra.Command{
	Use:   "start",
	Short: "Starts the rpc server. To run as a daemon, use the provided script (systemd) or use systemd/sysv/upstart.",
	Run: func(cmd *cobra.Command, args []string) {
		logger := utils.GetLogger()

		stopOtel, err := utils.InitOtel(cmd.Context(), cmd.Parent().Version)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to initialize otel")
		}
		defer stopOtel(cmd.Context())

		if os.Getenv("CEDANA_PROFILING_ENABLED") == "true" {
			go startProfiler()
		}

		if os.Getenv("CEDANA_GPU_ENABLED") == "true" {
			err := pullGPUBinary("gpucontroller", "/usr/local/bin/gpu-controller")
			if err != nil {
				logger.Warn().Err(err).Msg("could not pull gpu controller")
			}

			err = pullGPUBinary("libcedana", "/usr/local/lib/libcedana-gpu.so")
			if err != nil {
				logger.Warn().Err(err).Msg("could not pull libcedana")
			}
		}

		logger.Info().Msgf("daemon version %s started at %s", cmd.Parent().Version, time.Now().Local())

		startgRPCServer()
	},
}

func startgRPCServer() {
	logger := utils.GetLogger()

	if _, err := api.StartGRPCServer(); err != nil {
		logger.Error().Err(err).Msg("Failed to start gRPC server")
	}

}

// Used for debugging and profiling only!
func startProfiler() {
	utils.StartPprofServer()
}

func init() {
	rootCmd.AddCommand(clientDaemonCmd)
	clientDaemonCmd.AddCommand(startDaemonCmd)
}

func pullGPUBinary(binary string, filePath string) error {
	logger := utils.GetLogger()

	_, err := os.Stat(filePath)
	if err == nil {
		logger.Debug().Msgf("binary exists at %s, doing nothing", filePath)
		// file exists, do nothing.
		// TODO NR - check version of binary
		return nil
	}

	cfg, err := utils.InitConfig()
	if err != nil {
		logger.Err(err).Msg("could not init config")
		return err
	}
	url := "https://" + cfg.Connection.CedanaUrl + "/checkpoint/gpu/" + binary
	logger.Debug().Msgf("pulling %s from %s", binary, url)

	httpClient := &http.Client{}

	req, err := http.NewRequest("GET", url, nil)
	var resp *http.Response
	if err != nil {
		logger.Err(err).Msg("could not create request")
		return err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", cfg.Connection.CedanaAuthToken))

	resp, err = httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		logger.Err(err).Msg("gpu binary get request failed")
		return err
	}
	defer resp.Body.Close()

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0755)
	if err == nil {
		err = os.Chmod(filePath, 0755)
	}
	if err != nil {
		logger.Err(err).Msg("could not create file")
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		logger.Err(err).Msg("could not read file from response")
		return err
	}
	logger.Debug().Msgf("%s downloaded", binary)
	return nil
}
