package server

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestRefreshPackageListsWaitsForTheLock(t *testing.T) {
	oldRun, oldRetry := runAptUpdate, aptLockRetry
	t.Cleanup(func() { runAptUpdate, aptLockRetry = oldRun, oldRetry })
	aptLockRetry = time.Millisecond

	calls := 0
	runAptUpdate = func(context.Context) (string, error) {
		calls++
		if calls < 3 {
			return "E: Could not get lock /var/lib/apt/lists/lock. It is held by process 812 (apt-get)", errors.New("exit status 100")
		}
		return "Reading package lists...", nil
	}
	if _, err := refreshPackageLists(context.Background()); err != nil || calls != 3 {
		t.Fatalf("calls %d, err %v", calls, err)
	}

	// Any other failure is returned at once, with apt's words.
	calls = 0
	runAptUpdate = func(context.Context) (string, error) {
		calls++
		return "E: Conflicting values set for option Signed-By", errors.New("exit status 100")
	}
	out, err := refreshPackageLists(context.Background())
	if err == nil || calls != 1 || tailLines(out, 4) != "E: Conflicting values set for option Signed-By" {
		t.Fatalf("calls %d, err %v, out %q", calls, err, out)
	}
}
