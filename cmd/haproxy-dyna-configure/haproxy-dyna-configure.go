package main

import (
	"context"
	"os"

	"github.com/rvanderp3/haproxy-dyna-configure/pkg"
	log "github.com/sirupsen/logrus"
)

func main() {
	ctx := context.TODO()
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	err := pkg.Initialize(ctx)
	if err != nil {
		log.Errorf("unable to initialize %s", err)
		return
	}
	cfg, err := pkg.CheckRanges(ctx)
	if err != nil {
		log.Errorf("unable to check ranges %s", err)
		return
	}
	err = pkg.ApplyConfiguration(cfg)
	if err != nil {
		log.Errorf("unable to apply configuration %s", err)
		return
	}
}
