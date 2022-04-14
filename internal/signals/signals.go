package signals

import (
	"os"
	"os/signal"
	"syscall"
)

func CreateChannel() (chan os.Signal, func()) {
	stopCh := make(chan os.Signal, 1)
	signal.Notify(
		stopCh,
		os.Interrupt,
		syscall.SIGTERM,
		syscall.SIGINT)

	return stopCh, func() {
		close(stopCh)
	}
}
