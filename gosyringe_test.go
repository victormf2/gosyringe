package gosyringe

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRegisterSingleton(t *testing.T) {
	t.Parallel()

	t.Run("should resolve the correct type", func(t *testing.T) {
		t.Parallel()

		container := NewContainer()
		RegisterSingleton(container, NewService)
		service, err := Resolve[IService](container)
		assert.NoError(t, err)

		value := service.GetValue()

		assert.IsType(t, &Service{}, service)
		assert.Equal(t, 12, value)
	})

	t.Run("should resolve a single instance", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		var constructorCallCount int
		var mu sync.Mutex

		RegisterSingleton(c, func() IService {
			time.Sleep(10 * time.Millisecond)
			mu.Lock()
			constructorCallCount++
			mu.Unlock()
			return NewService()
		})

		const goroutines = 100
		var wg sync.WaitGroup
		results := make(chan IService, goroutines)

		for range goroutines {
			wg.Add(1)
			go func() {
				defer wg.Done()
				instance, err := Resolve[IService](c)
				assert.NoError(t, err, "resolve failed")

				results <- instance
			}()
		}

		wg.Wait()
		close(results)

		var firstInstance IService
		for instance := range results {
			if firstInstance == nil {
				firstInstance = instance
			} else if instance != firstInstance {
				t.Errorf("expected same instance, got different ones")
			}
		}

		assert.Equal(t, 1, constructorCallCount)
	})
}

type IService interface {
	GetValue() int
}
type Service struct{}

func (s Service) GetValue() int {
	return 12
}
func NewService() IService {
	return &Service{}
}
