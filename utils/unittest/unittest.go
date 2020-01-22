package unittest

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/storage/ledger/databases/leveldb"
)

func ExpectPanic(expectedMsg string, t *testing.T) {
	if r := recover(); r != nil {
		err := r.(error)
		if err.Error() != expectedMsg {
			t.Errorf("expected %v to be %v", err, expectedMsg)
		}
		return
	}
	t.Errorf("Expected to panic with `%s`, but did not panic", expectedMsg)
}

// AssertReturnsBefore asserts that the given function returns before the
// duration expires.
func AssertReturnsBefore(t *testing.T, f func(), duration time.Duration) {
	done := make(chan struct{})

	go func() {
		f()
		close(done)
	}()

	select {
	case <-time.After(duration):
		t.Log("function did not return in time")
		t.Fail()
	case <-done:
		return
	}
}

func RunWithBadgerDB(t *testing.T, f func(*badger.DB)) {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))

	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	require.Nil(t, err)

	defer func() {
		db.Close()
		os.RemoveAll(dir)
	}()

	f(db)
}

func RunWithLevelDB(t *testing.T, f func(db *leveldb.LevelDB)) {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("flow-test-db-%d", rand.Uint64()))

	kvdbPath := filepath.Join(dir, "kvdb")
	tdbPath := filepath.Join(dir, "tdb")

	db, err := leveldb.NewLevelDB(kvdbPath, tdbPath)
	require.Nil(t, err)

	defer func() {
		db.SafeClose()
		os.RemoveAll(dir)
	}()

	f(db)
}
