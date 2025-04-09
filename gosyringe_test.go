package gosyringe

import (
	"fmt"
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

	t.Run("should panic with invalid constructor", func(t *testing.T) {
		t.Parallel()

		testData := []struct {
			name        string
			constructor any
		}{
			{
				name:        "non function",
				constructor: struct{}{},
			},
			{
				name:        "no return value",
				constructor: func() {},
			},
			{
				name: "2nd return value not error",
				constructor: func() (IService, IService) {
					return &Service{}, &Service{}
				},
			},
			{
				name: "more than 2 return values",
				constructor: func() (IService, IService, error) {
					return &Service{}, &Service{}, fmt.Errorf("some error")
				},
			},
		}

		for _, tt := range testData {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				// https://stackoverflow.com/a/31596110
				defer func() {
					r := recover()
					if r == nil {
						t.Errorf("The code did not panic")
					}
				}()

				c := NewContainer()

				RegisterSingleton(c, tt.constructor)
			})
		}

	})

	t.Run("should accept custom error return", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton(c, func() (IService, *CustomError) {
			return &Service{}, nil
		})
	})

	t.Run("should return error on Resolve", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton(c, func() (IService, error) {
			return nil, customError
		})

		_, err := Resolve[IService](c)

		assert.ErrorIs(t, err, customError)
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

type CustomError struct{}

func (c CustomError) Error() string {
	return "custom error"
}

var customError = &CustomError{}
