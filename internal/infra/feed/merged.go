package feed

import (
	"context"
	"container/heap"
	"fmt"
	"sync"

	"github.com/felix/papertrading/internal/domain/market"
)

type tickItem struct {
	tick market.Tick
	idx  int
}

type priorityQueue []*tickItem

func (pq priorityQueue) Len() int           { return len(pq) }
func (pq priorityQueue) Less(i, j int) bool { return pq[i].tick.Timestamp.Before(pq[j].tick.Timestamp) }
func (pq priorityQueue) Swap(i, j int)      { pq[i], pq[j] = pq[j], pq[i] }

func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*tickItem))
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[:n-1]
	return item
}

type MergedFeed struct {
	feeds []market.Feed
}

func NewMergedFeed(feeds ...market.Feed) *MergedFeed {
	return &MergedFeed{feeds: feeds}
}

func (f *MergedFeed) Stream(ctx context.Context, ticks chan<- market.Tick) error {
	if len(f.feeds) == 0 {
		close(ticks)
		return nil
	}
	if len(f.feeds) == 1 {
		return f.feeds[0].Stream(ctx, ticks)
	}

	defer close(ticks)

	channels := make([]chan market.Tick, len(f.feeds))
	errCh := make(chan error, len(f.feeds))
	var wg sync.WaitGroup

	for i, feed := range f.feeds {
		ch := make(chan market.Tick, 100)
		channels[i] = ch
		wg.Add(1)
		go func(fd market.Feed, ch chan market.Tick, idx int) {
			defer wg.Done()
			if err := fd.Stream(ctx, ch); err != nil {
				errCh <- fmt.Errorf("feed %d: %w", idx, err)
			}
		}(feed, ch, i)
	}

	pq := &priorityQueue{}
	heap.Init(pq)

	for i, ch := range channels {
		tick, ok := <-ch
		if ok {
			heap.Push(pq, &tickItem{tick: tick, idx: i})
		}
	}

	for pq.Len() > 0 {
		item := heap.Pop(pq).(*tickItem)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case ticks <- item.tick:
		}

		nextTick, ok := <-channels[item.idx]
		if ok {
			heap.Push(pq, &tickItem{tick: nextTick, idx: item.idx})
		}
	}

	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}
