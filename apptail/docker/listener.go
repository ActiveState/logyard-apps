package docker

import (
	"github.com/ActiveState/log"
	"github.com/ActiveState/logyard-apps/common"
	"github.com/ActiveState/logyard-apps/docker_events"
	"runtime"
	"sync"
)

const ID_LENGTH = 12

type dockerListener struct {
	waiters map[string]chan bool
	mux     sync.Mutex
}

var DockerListener *dockerListener

func init() {
	DockerListener = new(dockerListener)
	DockerListener.waiters = make(map[string]chan bool)
}

func (l *dockerListener) BlockUntilContainerStops(id string) {
	var total int
	ch := make(chan bool)
	id = id[:ID_LENGTH]

	if len(id) != ID_LENGTH {
		common.Fatal("Invalid docker ID length: %v", len(id))
	}

	// Add a wait channel
	func() {
		l.mux.Lock()
		_, ok := l.waiters[id]
		l.mux.Unlock()
		runtime.Gosched()
		if ok {
			log.Warn("already added")

		} else {
			l.mux.Lock()
			l.waiters[id] = ch
			l.mux.Unlock()
			runtime.Gosched()
		}

		total = len(l.waiters)
	}()

	// Wait
	log.Infof("Waiting for container %v to exit (total waiters: %d)", id, total)
	<-ch
}

func (l *dockerListener) Listen() {
	for evt := range docker_events.Stream() {
		id := evt.Id[:ID_LENGTH]
		if len(id) != ID_LENGTH {
			common.Fatal("Invalid docker ID length: %v (orig: %v)", len(id), len(evt.Id))
		}

		// Notify container stop events by closing the appropriate ch.
		if !(evt.Status == "die" || evt.Status == "kill") {
			continue
		}
		l.mux.Lock()
		if ch, ok := l.waiters[id]; ok {
			close(ch)
			delete(l.waiters, id)
		}
		l.mux.Unlock()
	}
}
