package trumpet

import (
	"time"
)

type Scheduler interface {
	Train(d time.Time)
	Chan() <-chan struct{}
}