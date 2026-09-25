package helpers

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type slowHelper struct {
	name    string
	fields  []Field
	delay   time.Duration
	running atomic.Int32
	maxSeen atomic.Int32
}

func (h *slowHelper) Name() string    { return h.name }
func (h *slowHelper) Fields() []Field { return h.fields }

func (h *slowHelper) Collect(ctx context.Context, projectPath string) (map[string]any, error) {
	cur := h.running.Add(1)
	defer h.running.Add(-1)
	for {
		prev := h.maxSeen.Load()
		if cur <= prev || h.maxSeen.CompareAndSwap(prev, cur) {
			break
		}
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(h.delay):
		return map[string]any{"slow.value": projectPath}, nil
	}
}

func TestHelperEngineCapsParallelism(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan any, 64)
	h := &slowHelper{
		name:  "slow",
		delay: 40 * time.Millisecond,
		fields: []Field{{
			ID: "slow.value", Label: "Slow", Kind: FieldString, Sortable: true, Filterable: true,
		}},
	}
	engine := NewEngine(ctx, events, []Helper{h})

	const n = 20
	for i := 0; i < n; i++ {
		engine.Enrich(filepath.Join("/tmp", "p", string(rune('a'+i))))
	}

	got := 0
	deadline := time.After(3 * time.Second)
	for got < n {
		select {
		case msg := <-events:
			if _, ok := msg.(MetaMsg); ok {
				got++
			}
		case <-deadline:
			t.Fatalf("timed out waiting for helper results: got %d/%d", got, n)
		}
	}

	if max := h.maxSeen.Load(); max > helperMaxParallel {
		t.Fatalf("max parallel = %d, want <= %d", max, helperMaxParallel)
	}
	if max := h.maxSeen.Load(); max < 2 {
		t.Fatalf("expected some parallelism, max=%d", max)
	}
}

func TestMtimeHelperCollect(t *testing.T) {
	dir := t.TempDir()
	meta, err := (MtimeHelper{}).Collect(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	v, ok := meta[FieldDirMtime].(time.Time)
	if !ok || v.IsZero() {
		t.Fatalf("mtime meta = %#v", meta)
	}
}

func TestDefaultHelpersRegisterFields(t *testing.T) {
	engine := NewEngine(context.Background(), make(chan any, 1), Default(t.TempDir(), time.Hour, nil, nil))
	ids := map[string]bool{}
	for _, f := range engine.Fields() {
		ids[f.ID] = true
	}
	for _, want := range []string{"name", "tags", FieldDirMtime, FieldGitLastCommit, FieldGitHubOwner} {
		if !ids[want] {
			t.Fatalf("missing field %q in %#v", want, ids)
		}
	}
}

func TestHelperEngineIndependentCaps(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan any, 128)

	a := &slowHelper{name: "a", delay: 30 * time.Millisecond, fields: []Field{{ID: "a", Label: "A", Kind: FieldString, Sortable: true}}}
	b := &slowHelper{name: "b", delay: 30 * time.Millisecond, fields: []Field{{ID: "b", Label: "B", Kind: FieldString, Sortable: true}}}
	engine := NewEngine(ctx, events, []Helper{a, b})

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		path := filepath.Join("/tmp", "x", string(rune('a'+i)))
		go func() {
			defer wg.Done()
			engine.Enrich(path)
		}()
	}
	wg.Wait()

	deadline := time.After(3 * time.Second)
	got := 0
	for got < 20 {
		select {
		case <-events:
			got++
		case <-deadline:
			t.Fatalf("got %d/20", got)
		}
	}
	if a.maxSeen.Load() > helperMaxParallel || b.maxSeen.Load() > helperMaxParallel {
		t.Fatalf("caps exceeded a=%d b=%d", a.maxSeen.Load(), b.maxSeen.Load())
	}
}
