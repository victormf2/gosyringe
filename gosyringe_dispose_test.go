package gosyringe

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDispose(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	t.Run("container should only dispose instances it resolved", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		shouldNotDisposeCount := int32(0)
		RegisterSingleton[IService](c, NewService)
		OnDispose(c, func(ctx context.Context, service IService) {
			atomic.AddInt32(&shouldNotDisposeCount, 1)
		})
		RegisterSingleton[IDisposable](c, NewDisposableService)
		OnDispose(c, func(ctx context.Context, service IDisposable) {
			service.Dispose()
		})

		service, err := Resolve[IDisposable](c)
		require.NoError(t, err)

		Dispose(ctx, c)

		require.True(t, service.IsDisposed())
		require.Equal(t, int32(0), shouldNotDisposeCount)
	})

	t.Run("should dispose slice resolved instances", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingleton[IDisposable](c, NewDisposableService)
		RegisterSingleton[IDisposable](c, NewDisposableService)
		RegisterSingleton[IDisposable](c, NewDisposableService)
		OnDispose(c, func(ctx context.Context, service IDisposable) {
			service.Dispose()
		})

		allServices, err := Resolve[[]IDisposable](c)
		require.NoError(t, err)
		require.Len(t, allServices, 3)

		Dispose(ctx, c)

		for i, service := range allServices {
			require.True(t, service.IsDisposed(), "service %d is not disposed", i)
		}
	})

	t.Run("should dispose key resolved instances", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingletonWithKey[IDisposable](c, "s0", NewDisposableService)
		RegisterSingletonWithKey[IDisposable](c, "s1", NewDisposableService)
		OnDispose(c, func(ctx context.Context, service IDisposable) {
			service.Dispose()
		})

		s0, err := ResolveWithKey[IDisposable](c, "s0")
		require.NoError(t, err)
		s1, err := ResolveWithKey[IDisposable](c, "s1")
		require.NoError(t, err)

		Dispose(ctx, c)

		require.True(t, s0.IsDisposed())
		require.True(t, s1.IsDisposed())
	})

	t.Run("should not allow resolve on disposed container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingleton[IService](c, NewService)

		Dispose(ctx, c)

		_, err := Resolve[IService](c)
		require.EqualError(t, err, "cannot resolve on a disposed container")
	})

	t.Run("should wait all resolutions before disposing", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		resolveCount := int32(0)
		RegisterTransient[IService](c, NewServiceWithCallback(func() {
			time.Sleep(200 * time.Millisecond)
			atomic.AddInt32(&resolveCount, 1)
		}))

		const goroutines = int32(100)
		var wg sync.WaitGroup
		for range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				Resolve[IService](c)
			}()
		}

		time.Sleep(50 * time.Millisecond)

		Dispose(ctx, c)

		wg.Wait()

		require.Equal(t, goroutines, resolveCount)
	})

	t.Run("should not allow register on disposed container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingleton[IService](c, NewService)

		Dispose(ctx, c)

		require.PanicsWithValue(t, "cannot register on a disposed container", func() {
			RegisterSingleton[IService](c, NewService)
		})
	})

	t.Run("should not dispose root container instances from child", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IDisposable](c, NewDisposableService)
		OnDispose(c, func(ctx context.Context, service IDisposable) {
			service.Dispose()
		})

		child := CreateChildContainer(c)

		service, err := Resolve[IDisposable](child)
		require.NoError(t, err)

		Dispose(ctx, child)

		require.False(t, service.IsDisposed())
	})

	t.Run("should dispose child container resolved instances", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterScoped[IDisposable](c, NewDisposableService)
		OnDispose(c, func(ctx context.Context, service IDisposable) {
			service.Dispose()
		})

		child := CreateChildContainer(c)
		service, err := Resolve[IDisposable](child)
		require.NoError(t, err)

		Dispose(ctx, child)

		require.True(t, service.IsDisposed())
	})

	t.Run("should dispose transient instances from root", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterTransient[IDisposable](c, NewDisposableService)
		OnDispose(c, func(ctx context.Context, service IDisposable) {
			service.Dispose()
		})

		child := CreateChildContainer(c)

		s1Root, err := Resolve[IDisposable](c)
		require.NoError(t, err)
		s2Root, err := Resolve[IDisposable](c)
		require.NoError(t, err)
		sChild, err := Resolve[IDisposable](child)
		require.NoError(t, err)

		Dispose(ctx, c)

		require.True(t, s1Root.IsDisposed())
		require.True(t, s2Root.IsDisposed())
		require.False(t, sChild.IsDisposed())
	})

	t.Run("should dispose transient instances from child", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterTransient[IDisposable](c, NewDisposableService)
		OnDispose(c, func(ctx context.Context, service IDisposable) {
			service.Dispose()
		})

		child := CreateChildContainer(c)

		sRoot, err := Resolve[IDisposable](c)
		require.NoError(t, err)
		s1Child, err := Resolve[IDisposable](child)
		require.NoError(t, err)
		s2Child, err := Resolve[IDisposable](child)
		require.NoError(t, err)

		Dispose(ctx, child)

		require.False(t, sRoot.IsDisposed())
		require.True(t, s1Child.IsDisposed())
		require.True(t, s2Child.IsDisposed())
	})

	t.Run("should dispose all before timeout", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IDisposable](c, NewDisposableService)
		OnDispose(c, func(ctx context.Context, service IDisposable) {
			time.Sleep(50 * time.Millisecond)
			service.Dispose()
		})

		service, err := Resolve[IDisposable](c)
		require.NoError(t, err)

		disposeCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		defer cancel()
		Dispose(disposeCtx, c)

		require.True(t, service.IsDisposed())
	})

	t.Run("should abandon too long dispose calls after timeout", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IDisposable](c, NewDisposableService)
		OnDispose(c, func(ctx context.Context, service IDisposable) {
			time.Sleep(5000 * time.Millisecond)
			service.Dispose()
		})

		service, err := Resolve[IDisposable](c)
		require.NoError(t, err)

		disposeCtx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
		defer cancel()
		Dispose(disposeCtx, c)

		require.False(t, service.IsDisposed())
	})
}

type IDisposable interface {
	Dispose()
	IsDisposed() bool
}

type DisposableService struct {
	disposed bool
}

func NewDisposableService() IDisposable {
	return &DisposableService{}
}

func (d *DisposableService) Dispose() {
	d.disposed = true
}

func (d *DisposableService) IsDisposed() bool {
	return d.disposed
}
