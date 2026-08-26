package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func BenchmarkSnapshotStorage(b *testing.B) {
	contents := make([]byte, 256<<10)
	for index := range contents {
		contents[index] = byte(index)
	}
	snapshots, err := Open(b.TempDir(), 1<<20, nil)
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(contents)))
	b.ReportAllocs()
	for index := 0; b.Loop(); index++ {
		contents[0] = byte(index)
		hash := sha256.Sum256(contents)
		if _, err := snapshots.Put(Manifest{ContentHash: hex.EncodeToString(hash[:])}, contents); err != nil {
			b.Fatal(err)
		}
	}
}
