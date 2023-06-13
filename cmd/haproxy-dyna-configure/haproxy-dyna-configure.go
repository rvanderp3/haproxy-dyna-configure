package main

import (
	"context"
	"flag"
	goflag "flag"
	"os"
	"os/exec"
	"time"

	"github.com/rvanderp3/haproxy-dyna-configure/pkg"
	log "github.com/sirupsen/logrus"
	flags "github.com/spf13/pflag"
	"go.uber.org/zap"
	ctrl "sigs.k8s.io/controller-runtime"
	logzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func main() {
	loggerOpts := &logzap.Options{
		Development: true, // a sane default
		ZapOpts:     []zap.Option{zap.AddCaller()},
	}
	{
		var goFlagSet goflag.FlagSet
		loggerOpts.BindFlags(&goFlagSet)
		flags.CommandLine.AddGoFlagSet(&goFlagSet)
	}
	flag.Parse()
	ctrl.SetLogger(logzap.New(logzap.UseFlagOptions(loggerOpts)))

	ctx := context.TODO()
	log.SetOutput(os.Stdout)
	log.SetLevel(log.DebugLevel)
	err := pkg.Initialize(ctx)
	if err != nil {
		return
	}
	if pkg.IsHypershiftEnabled() {
		go func() {
			err = pkg.StartManager()
			if err != nil {
				log.Errorf("%s", err)
				return
			}
		}()
	}

	for {
		cfg, err := pkg.CheckRanges(ctx)
		if err != nil {
			log.Errorf("%s", err)
			return
		}

		err = pkg.ApplyDiscoveredConfiguration(cfg)
		if err != nil {
			log.Errorf("%s", err)
			return
		}
		if pkg.IsHypershiftEnabled() {
			err = pkg.ApplyHypershiftConfiguration()
			if err != nil {
				log.Errorf("%s", err)
				return
			}
		}
		log.Info("reloading haproxy")
		_, err = exec.Command("systemctl", "reload", "haproxy").Output()
		if err != nil {
			switch e := err.(type) {
			case *exec.Error:
				log.Error("failed executing:", err)
			case *exec.ExitError:
				log.Error("command exit rc =", e.ExitCode())
			default:
				panic(err)
			}
		}
		time.Sleep(time.Second * 30)
	}
}
