package config

import (
	"context"
	"os"
	"os/signal"
)

func defaultReloadSignalSource(ctx context.Context) (<-chan os.Signal, func(), error) {
	signals := reloadSignals()
	if len(signals) == 0 {
		return nil, func() {}, nil
	}

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)
	stop := func() { signal.Stop(ch) }
	go func() {
		<-ctx.Done()
		stop()
	}()
	return ch, stop, nil
}
