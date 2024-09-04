package main

import (
	"context"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/signal"
	"pinzoom"
	"pinzoom/config"
	"syscall"
	"time"
)

func init() {
	logrus.SetFormatter(&logrus.TextFormatter{
		ForceColors:     true,
		FullTimestamp:   true,
		TimestampFormat: time.DateTime,
	})
	logrus.SetOutput(os.Stdout)
}

func main() {
	var cmd = &cobra.Command{Use: "pinzoomd"}
	var configPath string

	cmd.PersistentFlags().StringVar(&configPath, "config", "config/config.yaml", "path to a config.yaml file")
	cmd.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Runs a server",
		Long:  `Runs a pinzoomd server`,
		Run: func(_ *cobra.Command, args []string) {
			// Handle interrupt signals
			interrupt := make(chan os.Signal, 1)
			signal.Notify(interrupt, os.Interrupt, syscall.SIGTERM)

			config := mustReadConfig(configPath)
			d := server.New(config)

			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				<-interrupt
				cancel()
			}()
			if err := d.Run(ctx, config); err != nil {
				logrus.Fatal(err)
			}
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "migrate",
		Short: "Applies database migrations",
		Long:  "Applies pending database migrations",
		Run: func(_ *cobra.Command, args []string) {
			config := mustReadConfig(configPath)
			d := server.New(config)
			if err := d.Migrate(); err != nil {
				logrus.Fatal(err)
			}
		},
	})

	err := cmd.Execute()
	if err != nil {
		return
	}
}

func mustReadConfig(path string) *config.Config {
	if path == "" {
		logrus.Fatal("--config required")
	}

	config, err := config.ReadYAMLFile(path)
	if err != nil {
		logrus.Fatalln(err)
	}
	return config
}
