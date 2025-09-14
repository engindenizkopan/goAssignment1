package ingest

import (
	"context"
	"log"
	"time"

	"example.com/goAssignment1/internal/domain"
	spg "example.com/goAssignment1/internal/storage/postgres"
)

type Ingestor struct {
	queue        chan domain.Event
	writer       *spg.Writer
	batchMaxSize int
	batchMaxWait time.Duration
}

func NewIngestor(writer *spg.Writer, queueMaxSize, batchMaxSize int, batchMaxWait time.Duration) *Ingestor {
	return &Ingestor{
		queue:        make(chan domain.Event, queueMaxSize),
		writer:       writer,
		batchMaxSize: batchMaxSize,
		batchMaxWait: batchMaxWait,
	}
}

func (ig *Ingestor) Start(ctx context.Context) {
	go func() {
		batch := make([]domain.Event, 0, ig.batchMaxSize)
		t := time.NewTimer(ig.batchMaxWait)
		defer t.Stop()

		resetTimer := func() {
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			t.Reset(ig.batchMaxWait)
		}

		flush := func() {
			if len(batch) == 0 {
				resetTimer()
				return
			}
			affected, err := ig.writer.InsertBatch(ctx, batch)
			if err != nil {
				log.Printf("[ingest] batch insert FAILED: err=%v dropped=%d", err, len(batch))
			} else {
				log.Printf("[ingest] batch insert OK: inserted=%d size=%d", affected, len(batch))
			}
			batch = batch[:0]
			resetTimer()
		}

		for {
			select {
			case <-ctx.Done():
				flush()
				return
			case ev := <-ig.queue:
				batch = append(batch, ev)
				if len(batch) >= ig.batchMaxSize {
					flush()
				}
			case <-t.C:
				flush()
			}
		}
	}()
}

func (ig *Ingestor) Enqueue(ev domain.Event) bool {
	select {
	case ig.queue <- ev:
		return true
	default:
		return false
	}
}
