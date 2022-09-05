package whoismain

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

var (
	AliveTimeout    = 200 * time.Millisecond
	RefreshInterval = 100 * time.Millisecond
	UpdateInterval  = 50 * time.Millisecond
)

type Node struct {
	Start   int64
	Updated int64
}

func (n *Node) IsAlive() bool {
	return time.Now().UnixNano()-n.Updated < AliveTimeout.Nanoseconds()
}

type AliveData struct {
	Name  string `json:"name"`
	Start int64  `json:"start"`
}

type SendAliveFunc func(alive AliveData) bool

type WhoIsMain struct {
	start         int64
	id            string
	main          bool
	alive         bool
	nodes         map[string]*Node
	mu            *sync.Mutex
	cancel        context.CancelFunc
	sendAliveFunc SendAliveFunc
}

func New(sendAliveFunc SendAliveFunc) WhoIsMain {
	return NewWithID(uuid.New().String(), sendAliveFunc)
}

func NewWithID(id string, sendAliveFunc SendAliveFunc) WhoIsMain {
	return WhoIsMain{
		start:         time.Now().UnixNano(),
		id:            id,
		main:          false,
		alive:         true,
		nodes:         map[string]*Node{},
		mu:            &sync.Mutex{},
		sendAliveFunc: sendAliveFunc,
	}
}

func (w *WhoIsMain) IsAlive() bool {
	return w.alive
}

func (w *WhoIsMain) IsMain() bool {
	return w.main
}

func (w *WhoIsMain) ID() string {
	return w.id
}

func (w *WhoIsMain) Open(ctx context.Context) error {
	if w.cancel != nil {
		return errors.New("already open")
	}
	ctx, w.cancel = context.WithCancel(ctx)

	go w.worker(ctx)

	return nil
}

func (w *WhoIsMain) Close() {
	if w.cancel != nil {
		w.cancel()
	}
}

func (w *WhoIsMain) AliveReceived(alive AliveData) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, ok := w.nodes[alive.Name]; !ok {
		w.nodes[alive.Name] = &Node{
			Start: alive.Start,
		}
	}
	w.nodes[alive.Name].Updated = time.Now().UnixNano()
}

func (w *WhoIsMain) update() {
	if !w.IsAlive() {
		return
	}

	w.mu.Lock()

	start := w.start
	for name, node := range w.nodes {
		if node.IsAlive() {
			if node.Start < start {
				start = node.Start
			}
		} else {
			delete(w.nodes, name)
		}
	}
	w.mu.Unlock()

	w.main = w.start <= start
}

func (w *WhoIsMain) aliveData() AliveData {
	return AliveData{
		Name:  w.id,
		Start: w.start,
	}
}

func (w *WhoIsMain) worker(ctx context.Context) {
	tickerAlive := time.NewTicker(RefreshInterval)
	tickerUpdate := time.NewTicker(UpdateInterval)
	startUp := int(RefreshInterval/UpdateInterval) + 1
	for {
		select {
		case <-ctx.Done():
			tickerAlive.Stop()
			tickerUpdate.Stop()
			w.cancel = nil
			return
		case <-tickerAlive.C:
			w.alive = w.sendAliveFunc(w.aliveData())
		case <-tickerUpdate.C:
			if startUp > 0 {
				startUp--
				continue
			}
			w.update()
		}
	}
}
