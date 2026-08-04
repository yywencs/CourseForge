package danmakuws

import (
	"sync"
	"testing"
)

func TestVideoConnectionRegistryLifecycle(t *testing.T) {
	registry := newVideoConnectionRegistry()
	first := newTestClientConn(1, 101, 1)
	second := newTestClientConn(2, 101, 1)

	if created := registry.Add(first); !created {
		t.Fatal("first Add() created = false, want true")
	}
	if created := registry.Add(second); created {
		t.Fatal("second Add() created = true, want false")
	}

	snapshot, exists := registry.SnapshotInto(101, nil)
	if !exists {
		t.Fatal("SnapshotInto() exists = false, want true")
	}
	if len(snapshot) != 2 {
		t.Fatalf("SnapshotInto() len = %d, want 2", len(snapshot))
	}
	if count := registry.Counts()[101]; count != 2 {
		t.Fatalf("Counts()[101] = %d, want 2", count)
	}

	removed, empty := registry.Remove(first)
	if !removed || empty {
		t.Fatalf("Remove(first) = (%v, %v), want (true, false)", removed, empty)
	}
	removed, empty = registry.Remove(second)
	if !removed || !empty {
		t.Fatalf("Remove(second) = (%v, %v), want (true, true)", removed, empty)
	}
	if _, exists := registry.SnapshotInto(101, nil); exists {
		t.Fatal("SnapshotInto() exists = true after the video has no connections")
	}
}

func TestVideoConnectionRegistryKeepsConnectionsDistinctPerVideo(t *testing.T) {
	registry := newVideoConnectionRegistry()
	first := newTestClientConn(1, 101, 1)
	second := newTestClientConn(1, 202, 1)

	if !registry.Add(first) || !registry.Add(second) {
		t.Fatal("the first connection for each video should be reported")
	}
	counts := registry.Counts()
	if counts[101] != 1 || counts[202] != 1 {
		t.Fatalf("Counts() = %v, want one connection in each video", counts)
	}
}

func TestVideoConnectionRegistryRemoveUnknownConnection(t *testing.T) {
	registry := newVideoConnectionRegistry()
	known := newTestClientConn(1, 101, 1)
	unknown := newTestClientConn(2, 101, 1)
	registry.Add(known)

	removed, empty := registry.Remove(unknown)
	if removed || empty {
		t.Fatalf("Remove(unknown) = (%v, %v), want (false, false)", removed, empty)
	}
	if count := registry.Counts()[101]; count != 1 {
		t.Fatalf("Counts()[101] = %d, want 1", count)
	}
}

func TestVideoConnectionRegistryDrainRemovesAllConnections(t *testing.T) {
	registry := newVideoConnectionRegistry()
	first := newTestClientConn(1, 101, 1)
	second := newTestClientConn(2, 202, 1)
	registry.Add(first)
	registry.Add(second)

	drained := registry.Drain()
	if len(drained) != 2 {
		t.Fatalf("Drain() len = %d, want 2", len(drained))
	}
	seen := make(map[*clientConn]bool, len(drained))
	for _, client := range drained {
		seen[client] = true
	}
	if !seen[first] || !seen[second] {
		t.Fatal("Drain() did not return every registered connection")
	}
	if counts := registry.Counts(); len(counts) != 0 {
		t.Fatalf("Counts() after Drain = %v, want empty", counts)
	}
	if drainedAgain := registry.Drain(); len(drainedAgain) != 0 {
		t.Fatalf("second Drain() len = %d, want 0", len(drainedAgain))
	}
}

func TestVideoConnectionRegistrySupportsConcurrentLifecycle(t *testing.T) {
	const connectionCount = 128

	registry := newVideoConnectionRegistry()
	connections := make([]*clientConn, connectionCount)
	for i := range connections {
		connections[i] = newTestClientConn(uint64(i+1), 101, 1)
	}

	var addGroup sync.WaitGroup
	for _, client := range connections {
		addGroup.Add(1)
		go func() {
			defer addGroup.Done()
			registry.Add(client)
		}()
	}
	addGroup.Wait()
	if count := registry.Counts()[101]; count != connectionCount {
		t.Fatalf("count after concurrent Add() = %d, want %d", count, connectionCount)
	}

	var lifecycleGroup sync.WaitGroup
	for _, client := range connections {
		lifecycleGroup.Add(2)
		go func() {
			defer lifecycleGroup.Done()
			registry.SnapshotInto(101, nil)
		}()
		go func() {
			defer lifecycleGroup.Done()
			registry.Remove(client)
		}()
	}
	lifecycleGroup.Wait()
	if counts := registry.Counts(); len(counts) != 0 {
		t.Fatalf("Counts() after concurrent Remove() = %v, want empty", counts)
	}
}
