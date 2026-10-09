package unit

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ibenrustandi/inventory-reservation/backend/internal/sweeper"
	"github.com/ibenrustandi/inventory-reservation/backend/internal/usecase"
	"github.com/ibenrustandi/inventory-reservation/backend/test/unit/mocks"
)

// AC-OP-02
func TestSweeperRun_StopsOnCancel(t *testing.T) {
	var sweeps atomic.Int32
	releaser := &mocks.InventoryUsecase{
		ReleaseExpiredFunc: func(context.Context) (int, error) {
			sweeps.Add(1)
			return 0, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		sweeper.New(releaser, time.Millisecond).Run(ctx)
	}()
	require.Eventually(t, func() bool { return sweeps.Load() >= 3 },
		5*time.Second, time.Millisecond, "sweeper never ran")

	cancel()

	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after its context was cancelled")
	}
	after := sweeps.Load()
	require.Never(t, func() bool { return sweeps.Load() != after },
		50*time.Millisecond, 5*time.Millisecond, "sweeper kept sweeping after Run returned")
}

func TestSweeperRun_KeepsGoingAfterAFailedSweep(t *testing.T) {
	var sweeps atomic.Int32
	releaser := &mocks.InventoryUsecase{
		ReleaseExpiredFunc: func(context.Context) (int, error) {
			if sweeps.Add(1) == 1 {
				return 0, errors.New("connection reset")
			}
			return 1, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		sweeper.New(releaser, time.Millisecond).Run(ctx)
	}()
	defer func() {
		cancel()
		<-stopped
	}()

	require.Eventually(t, func() bool { return sweeps.Load() >= 3 },
		5*time.Second, time.Millisecond, "sweeper stopped after a failed sweep")
}

func TestReleaseExpiredUsecase(t *testing.T) {
	// sweepsReturning is a repository whose SweepDue returns the given
	// counts in turn.
	sweepsReturning := func(limits *[]int, counts ...int) *mocks.InventoryRepository {
		return &mocks.InventoryRepository{
			SweepDueFunc: func(_ context.Context, limit int) (int, error) {
				*limits = append(*limits, limit)
				n := counts[0]
				counts = counts[1:]
				return n, nil
			},
		}
	}

	t.Run("keeps sweeping while batches come back full", func(t *testing.T) {
		var limits []int
		inventoryUsecase := usecase.NewInventoryUsecase(
			sweepsReturning(&limits, usecase.DefaultSweepBatchSize, usecase.DefaultSweepBatchSize, 7),
			reservationTTL)

		released, err := inventoryUsecase.ReleaseExpired(ctx)

		require.NoError(t, err)
		require.Equal(t, 2*usecase.DefaultSweepBatchSize+7, released)
		require.Equal(t, []int{usecase.DefaultSweepBatchSize, usecase.DefaultSweepBatchSize, usecase.DefaultSweepBatchSize}, limits)
	})

	t.Run("stops after one call when nothing is due", func(t *testing.T) {
		var limits []int
		inventoryUsecase := usecase.NewInventoryUsecase(sweepsReturning(&limits, 0), reservationTTL)

		released, err := inventoryUsecase.ReleaseExpired(ctx)

		require.NoError(t, err)
		require.Equal(t, 0, released)
		require.Len(t, limits, 1)
	})

	t.Run("reports what was released before an error", func(t *testing.T) {
		repoErr := errors.New("connection reset")
		calls := 0
		inventoryRepository := &mocks.InventoryRepository{
			SweepDueFunc: func(context.Context, int) (int, error) {
				calls++
				if calls == 1 {
					return usecase.DefaultSweepBatchSize, nil
				}
				return 0, repoErr
			},
		}
		inventoryUsecase := usecase.NewInventoryUsecase(inventoryRepository, reservationTTL)

		released, err := inventoryUsecase.ReleaseExpired(ctx)

		require.ErrorIs(t, err, repoErr)
		require.Equal(t, usecase.DefaultSweepBatchSize, released)
	})
}
