package observability

import (
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestProgressHub_PubSub(t *testing.T) {
	hub := NewProgressHub()
	transferID := uuid.New()

	ch, unsub := hub.Subscribe(transferID)
	defer unsub()

	progress := EnrichedProgress{
		TransferID:       transferID,
		BytesTransferred: 100,
		BytesTotal:       1000,
		PercentComplete:  10.0,
		FormattedSpeed:   "100 B/s",
	}
	hub.Publish(transferID, progress)

	select {
	case got := <-ch:
		if got.BytesTransferred != 100 {
			t.Errorf("BytesTransferred = %d, want 100", got.BytesTransferred)
		}
		if got.PercentComplete != 10.0 {
			t.Errorf("PercentComplete = %f, want 10.0", got.PercentComplete)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for progress event")
	}
}

func TestProgressHub_MultipleSubscribers(t *testing.T) {
	hub := NewProgressHub()
	transferID := uuid.New()

	ch1, unsub1 := hub.Subscribe(transferID)
	defer unsub1()
	ch2, unsub2 := hub.Subscribe(transferID)
	defer unsub2()

	progress := EnrichedProgress{TransferID: transferID, BytesTransferred: 50}
	hub.Publish(transferID, progress)

	for i, ch := range []<-chan EnrichedProgress{ch1, ch2} {
		select {
		case got := <-ch:
			if got.BytesTransferred != 50 {
				t.Errorf("subscriber %d: BytesTransferred = %d, want 50", i, got.BytesTransferred)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timed out", i)
		}
	}
}

func TestProgressHub_Unsubscribe(t *testing.T) {
	hub := NewProgressHub()
	transferID := uuid.New()

	ch, unsub := hub.Subscribe(transferID)
	unsub()

	// Channel should be closed after unsubscribe.
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after unsubscribe")
	}

	// Publishing after unsubscribe should not panic.
	hub.Publish(transferID, EnrichedProgress{})
}

func TestProgressHub_CloseTransfer(t *testing.T) {
	hub := NewProgressHub()
	transferID := uuid.New()

	ch1, _ := hub.Subscribe(transferID)
	ch2, _ := hub.Subscribe(transferID)

	hub.CloseTransfer(transferID)

	// Both channels should be closed.
	if _, ok := <-ch1; ok {
		t.Error("ch1: expected channel to be closed")
	}
	if _, ok := <-ch2; ok {
		t.Error("ch2: expected channel to be closed")
	}
}

func TestProgressHub_SlowConsumerNonBlocking(t *testing.T) {
	hub := NewProgressHub()
	transferID := uuid.New()

	ch, unsub := hub.Subscribe(transferID)
	defer unsub()

	// Fill the buffer.
	for i := range subscriberBufferSize + 5 {
		hub.Publish(transferID, EnrichedProgress{BytesTransferred: int64(i)})
	}

	// Should have buffered events (not blocked).
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != subscriberBufferSize {
		t.Errorf("got %d buffered events, want %d", count, subscriberBufferSize)
	}
}

func TestProgressHub_ConcurrentPubSub(t *testing.T) {
	hub := NewProgressHub()
	transferID := uuid.New()

	const numSubscribers = 10
	const numMessages = 100

	var wg sync.WaitGroup
	received := make([]int, numSubscribers)

	for i := range numSubscribers {
		ch, unsub := hub.Subscribe(transferID)
		wg.Add(1)
		go func(idx int, c <-chan EnrichedProgress, u func()) {
			defer wg.Done()
			defer u()
			for range c {
				received[idx]++
			}
		}(i, ch, unsub)
	}

	for range numMessages {
		hub.Publish(transferID, EnrichedProgress{TransferID: transferID})
	}

	hub.CloseTransfer(transferID)
	wg.Wait()

	for i, count := range received {
		if count == 0 {
			t.Errorf("subscriber %d received 0 messages", i)
		}
	}
}

func TestProgressHub_NoSubscribers(t *testing.T) {
	hub := NewProgressHub()
	// Publishing to a transfer with no subscribers should not panic.
	hub.Publish(uuid.New(), EnrichedProgress{})
}

func TestProgressHub_CloseNonExistentTransfer(t *testing.T) {
	hub := NewProgressHub()
	// Closing a non-existent transfer should not panic.
	hub.CloseTransfer(uuid.New())
}
