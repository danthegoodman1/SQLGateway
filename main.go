package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/danthegoodman1/PSQLGateway/gologger"
	"github.com/danthegoodman1/PSQLGateway/http_server"
	"github.com/danthegoodman1/PSQLGateway/pg"
	"github.com/danthegoodman1/PSQLGateway/utils"
)

var logger = gologger.NewLogger()

func main() {
	logger.Info().Msg("starting PSQLGateway")

	if err := pg.ConnectToDB(); err != nil {
		logger.Error().Err(err).Msg("error connecting to PG Pool")
		os.Exit(1)
	}

	httpServer := http_server.StartHTTPServer()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	logger.Warn().Msg("received shutdown signal!")

	// Convert the time to seconds
	sleepTime := utils.GetEnvOrDefaultInt("SHUTDOWN_SLEEP_SEC", 0)
	logger.Info().Msg(fmt.Sprintf("sleeping for %ds before exiting", sleepTime))

	time.Sleep(time.Second * time.Duration(sleepTime))
	logger.Info().Msg(fmt.Sprintf("slept for %ds, exiting", sleepTime))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("failed to shutdown HTTP server")
	} else {
		logger.Info().Msg("successfully shutdown HTTP server")
	}
}
