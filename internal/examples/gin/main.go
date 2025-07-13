package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/victormf2/gosyringe"
	"github.com/victormf2/gosyringe/internal/examples/gin/setup"
)

// Some of this code was taken from the GIN graceful shutdown example
// and adapted to run with the gosyringe features
// https://github.com/gin-gonic/examples/blob/9fd0db1d6a7cdfd8dd1e0b163146674ea9d4ecfd/graceful-shutdown/graceful-shutdown/notify-with-context/server.go
func main() {
	// First create the root Container
	c := gosyringe.NewContainer()

	// Then register all dependencies. You can do the registrations inline,
	// but it's usually better to separate that logic in a function, so you
	// can use it in your tests.
	setup.RegisterServices(c)
	setup.RegisterHttpServer(c)

	// Create context that listens for the interrupt signal from the OS.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Start the server with the callback registered with OnStart
	gosyringe.Start(c)

	// Listen for the interrupt signal.
	<-ctx.Done()

	// Restore default behavior on the interrupt signal and notify user of shutdown.
	stop()
	log.Println("shutting down gracefully, press Ctrl+C again to force")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server with the callback registered with OnDispose
	gosyringe.Dispose(ctx, c)

	log.Println("Server exiting")
}
