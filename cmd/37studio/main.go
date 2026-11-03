// Command 37studio serves the app and runs the agent's helper commands (gen, clip, transcribe).
package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/Shaitrish/37studio/internal/config"
	"github.com/Shaitrish/37studio/internal/server"
)

func main() {
	if code, ok := server.CLI(os.Args[1:]); ok {
		os.Exit(code)
	}
	if err := server.Init(); err != nil {
		log.Fatal(err)
	}
	ln, err := net.Listen("tcp", config.Addr)
	if err != nil {
		log.Fatal(err)
	}
	// docker stop sends SIGTERM: finish requests and end agent runs instead of leaving them orphaned.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := server.Serve(ctx, ln); err != nil {
		log.Fatal(err)
	}
}
