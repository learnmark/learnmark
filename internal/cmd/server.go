package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/learnmark/learnmark/internal/gateway"
	"github.com/learnmark/learnmark/internal/grpc"
	"github.com/learnmark/learnmark/pkg/log"
	"github.com/learnmark/learnmark/pkg/utils"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

var (
	// cfgPath is the path to the EnvoyGateway configuration file.
	cfgPath string
)

func GetServerCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Run both grpc and http server",
		RunE: func(cmd *cobra.Command, args []string) error {
			log, err := log.New()
			if err != nil {
				return err
			}
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				log.Infof("FLAG: --%s=%q", flag.Name, flag.Value)
			})

			stop := make(chan struct{})
			defer WaitSignal(stop)

			opts := utils.Options{
				HTTPAddr:   viper.GetString("server.http.addr"),
				GRPCAddr:   viper.GetString("server.grpc.addr"),
				Network:    "tcp",
				OpenAPIDir: "api/v1",
			}

			if err := grpc.Run(context.Background(), opts); err != nil {
				log.Fatalf("grpc start error: %s", err)
			}
			if err := gateway.Run(context.Background(), opts); err != nil {
				log.Fatalf("grpc gateway start error: %s", err)
			}

			return nil
		},
	}
	cobra.OnInitialize(InitConfig)
	cmd.PersistentFlags().StringVar(&cfgFile, "config", "./config/config.yaml", "config file (default is $HOME/.learnmark.yaml)")
	cmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	cmd.PersistentFlags().StringVarP(&cfgPath, "config-path", "c", "",
		"The path to the configuration file.")

	return cmd
}

func WaitSignal(stop chan struct{}) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	<-sigs
	close(stop)
}

func InitConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.AddConfigPath(home)
		viper.SetConfigName(".learnmark")
	}

	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
