package reporter

import (
	"fmt"
	"github.com/kaytu-io/kaytu-engine/pkg/internal/httpserver"
	config2 "github.com/kaytu-io/kaytu-util/pkg/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/net/context"
	"os"
)

var HttpAddress = os.Getenv("HTTP_ADDRESS")

func ReporterCommand() *cobra.Command {
	cmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			config := JobConfig{}
			config2.ReadFromEnv(&config, nil)
			j, err := New(config)
			if err != nil {
				panic(err)
			}

			EnsureRunGoroutin(func() {
				j.Run()
			})
			return startHttpServer(cmd.Context(), j)
		},
	}

	return cmd
}

func startHttpServer(ctx context.Context, j *Job) error {

	logger, err := zap.NewProduction()
	if err != nil {
		return fmt.Errorf("new logger: %w", err)
	}

	httpServer := NewHTTPServer(HttpAddress, logger, j)
	if err != nil {
		return fmt.Errorf("init http handler: %w", err)
	}

	return httpserver.RegisterAndStart(logger, HttpAddress, httpServer)
}
