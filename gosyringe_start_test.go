package gosyringe

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStart(t *testing.T) {
	t.Parallel()

	t.Run("should not allow register start callback on a child container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingleton[IStartable](c, NewStartable)

		child := CreateChildContainer(c)

		require.PanicsWithError(t, "cannot register start callback on a child container, only root allowed", func() {
			OnStart(child, func(_ IStartable) {})
		})
	})

	t.Run("should not allow register start callback for a non registered dependency", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		require.PanicsWithError(t, "you must register a dependency for gosyringe.IStartable before a start callback", func() {
			OnStart(c, func(_ IStartable) {})
		})
	})

	t.Run("should not allow register start callback for a non Singleton dependency", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterScoped[IStartable](c, NewStartable)

		require.PanicsWithError(t, "cannot register callback on a Scoped dependency, only Singleton allowed", func() {
			OnStart(c, func(_ IStartable) {})
		})
	})

	t.Run("should not allow register start callback for a started container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IStartable](c, NewStartable)

		Start(c)

		require.PanicsWithError(t, "cannot register start callback on a started container", func() {
			OnStart(c, func(_ IStartable) {})
		})
	})

	t.Run("should not start a child container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		child := CreateChildContainer(c)

		require.PanicsWithError(t, "cannot start a child container, only root allowed", func() {
			Start(child)
		})
	})

	t.Run("should not start multiple times", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IStartable](c, NewStartable)

		startCount := atomic.Int32{}
		OnStart(c, func(service IStartable) {
			startCount.Add(1)
			service.Start()
		})

		const goroutines = 100
		var wg sync.WaitGroup
		for range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				Start(c)

				time.Sleep(10 * time.Millisecond)
			}()
		}

		wg.Wait()

		require.Equal(t, int32(1), startCount.Load())
	})

	t.Run("OnStart should start last registered dependency for a type", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		called := [3]bool{}
		RegisterSingleton[IStartable](c, func() IStartable {
			called[0] = true
			return NewStartable()
		})
		RegisterSingleton[IStartable](c, func() IStartable {
			called[1] = true
			return NewStartable()
		})
		RegisterSingleton[IStartable](c, func() IStartable {
			called[2] = true
			return NewStartable()
		})
		OnStart(c, func(service IStartable) {
			service.Start()
		})

		Start(c)

		time.Sleep(10 * time.Millisecond)

		service, err := Resolve[IStartable](c)
		require.NoError(t, err)

		require.False(t, called[0])
		require.False(t, called[1])
		require.True(t, called[2])
		require.True(t, service.HasStarted(), "service 2 has not started")
	})

	t.Run("OnStartAll should start all registered dependencies for a type", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IStartable](c, NewStartable)
		RegisterSingleton[IStartable](c, NewStartable)
		RegisterSingleton[IStartable](c, NewStartable)
		OnStartAll(c, func(service IStartable) {
			service.Start()
		})

		Start(c)

		time.Sleep(10 * time.Millisecond)

		allServices, err := ResolveAll[IStartable](c)
		require.NoError(t, err)

		for i, service := range allServices {
			require.True(t, service.HasStarted(), "service %d has not started", i)
		}
	})

	t.Run("OnStartWithKey should start last registered dependency for a key", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		called := [3]bool{}
		RegisterSingletonWithKey[IStartable](c, "service", func() IStartable {
			called[0] = true
			return NewStartable()
		})
		RegisterSingletonWithKey[IStartable](c, "service", func() IStartable {
			called[1] = true
			return NewStartable()
		})
		RegisterSingletonWithKey[IStartable](c, "service", func() IStartable {
			called[2] = true
			return NewStartable()
		})
		OnStartWithKey(c, "service", func(service IStartable) {
			service.Start()
		})

		Start(c)

		time.Sleep(10 * time.Millisecond)

		service, err := ResolveWithKey[IStartable](c, "service")
		require.NoError(t, err)

		require.False(t, called[0])
		require.False(t, called[1])
		require.True(t, called[2])
		require.True(t, service.HasStarted(), "service 2 has not started")
	})

	t.Run("OnStartAllWithKey should start all registered dependencies for a key", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingletonWithKey[IStartable](c, "service", NewStartable)
		RegisterSingletonWithKey[IStartable](c, "service", NewStartable)
		RegisterSingletonWithKey[IStartable](c, "service", NewStartable)
		OnStartAllWithKey(c, "service", func(service IStartable) {
			service.Start()
		})

		Start(c)

		time.Sleep(10 * time.Millisecond)

		allServices, err := ResolveAllWithKey[IStartable](c, "service")
		require.NoError(t, err)

		for i, service := range allServices {
			require.True(t, service.HasStarted(), "service %d has not started", i)
		}
	})
}

type IStartable interface {
	Start()
	HasStarted() bool
}

type Startable struct {
	started bool
}

func NewStartable() IStartable {
	return &Startable{}
}

func (s *Startable) Start() {
	s.started = true
}

func (s *Startable) HasStarted() bool {
	return s.started
}
