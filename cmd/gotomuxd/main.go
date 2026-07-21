package main

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/fm39hz/gotomux/internal/daemon"
)

func main() {
	d, err := daemon.New()
	if err != nil {
		fmt.Fprintf(os.Stderr, "gotomuxd: %v\n", err)
		os.Exit(1)
	}
	defer d.Close()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, os.Kill)
	<-sigCh
}
