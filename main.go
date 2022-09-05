package main

import (
	"context"
	"fmt"
	"time"

	"github.com/docseltsam/main/whoismain"
)

type fanout struct {
	channel chan whoismain.AliveData
}

var (
	in      = make(chan whoismain.AliveData)
	fanouts = []fanout{}
)

func doFanout(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case i := <-in:
			for _, f := range fanouts {
				f.channel <- i
			}
		}
	}
}

func (f *fanout) sendAlive(alive whoismain.AliveData) bool {
	in <- alive
	return true
}

func (f *fanout) process(wim *whoismain.WhoIsMain) {
	for {
		alive := <-f.channel
		wim.AliveReceived(alive)
	}
}

func showMain(ctx context.Context, wim *whoismain.WhoIsMain) {
	first := true
	main := false

	ticker := time.NewTicker(50 * time.Millisecond)

	for {
		select {
		case <-ctx.Done():
			ticker.Stop()
			return
		case <-ticker.C:
			if first || main != wim.IsMain() {
				fmt.Printf("%v: IsMain: %v\n", wim.ID(), wim.IsMain())
				first = false
				main = wim.IsMain()
			}
		}
	}
}

func start(ctx context.Context, id string) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(ctx)

	fo := fanout{
		channel: make(chan whoismain.AliveData),
	}
	fanouts = append(fanouts, fo)

	wim := whoismain.NewWithID(id, fo.sendAlive)
	go fo.process(&wim)

	wim.Open(ctx)

	go showMain(ctx, &wim)

	fmt.Printf("%v started\n", wim.ID())

	return ctx, cancel
}

func main() {
	ctx := context.Background()

	go doFanout(ctx)

	_, cancel1 := start(ctx, "node1")
	_, cancel2 := start(ctx, "node2")
	_, cancel3 := start(ctx, "node3")

	time.Sleep(2 * time.Second)
	fmt.Println("Cancel 1")
	cancel1()

	time.Sleep(1 * time.Second)
	fmt.Println("Cancel 2")
	cancel2()

	fmt.Println("Start 4")
	start(ctx, "node4")

	time.Sleep(1 * time.Second)
	fmt.Println("Cancel 3")
	cancel3()

	var forever chan interface{}
	<-forever
}
