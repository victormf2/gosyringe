package gosyringe

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestRegisterSingleton(t *testing.T) {
	t.Parallel()

	t.Run("should resolve the correct type", func(t *testing.T) {
		t.Parallel()

		testData := []struct {
			name        string
			constructor any
		}{
			{
				name:        "safe constructor",
				constructor: NewService,
			},
			{
				name:        "unsafe constructor",
				constructor: NewServiceUnsafe,
			},
		}

		for _, tt := range testData {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				container := NewContainer()
				RegisterSingleton[IService](container, tt.constructor)
				service, err := Resolve[IService](container)
				assert.NoError(t, err)

				value := service.GetValue()

				assert.IsType(t, &Service{}, service)
				assert.Equal(t, 12, value)
			})
		}

	})

	t.Run("should resolve a single instance", func(t *testing.T) {
		t.Parallel()

		testData := []struct {
			name        string
			constructor any
		}{
			{
				name:        "safe constructor",
				constructor: NewService,
			},
			{
				name:        "unsafe constructor",
				constructor: NewServiceUnsafe,
			},
		}

		for _, tt := range testData {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				c := NewContainer()

				var constructorCallCount int
				var mu sync.Mutex

				RegisterSingleton[IService](c, func() IService {
					time.Sleep(10 * time.Millisecond)
					mu.Lock()
					constructorCallCount++
					mu.Unlock()

					service := reflect.ValueOf(tt.constructor).Call(nil)[0]
					return service.Interface().(IService)
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
	})

	t.Run("should panic with invalid constructor", func(t *testing.T) {
		t.Parallel()

		testData := []struct {
			name          string
			constructor   any
			expectedPanic string
		}{
			{
				name:          "non function",
				constructor:   struct{}{},
				expectedPanic: "constructor must be a function returning exactly one value, or a value and an error",
			},
			{
				name:          "no return value",
				constructor:   func() {},
				expectedPanic: "constructor must be a function returning exactly one value, or a value and an error",
			},
			{
				name: "2nd return value not error",
				constructor: func() (IService, IService) {
					return &Service{}, &Service{}
				},
				expectedPanic: "constructor must be a function returning exactly one value, or a value and an error",
			},
			{
				name: "more than 2 return values",
				constructor: func() (IService, IService, error) {
					return &Service{}, &Service{}, fmt.Errorf("some error")
				},
				expectedPanic: "constructor must be a function returning exactly one value, or a value and an error",
			},
			{
				name: "constructor type mismatch",
				constructor: func() Service {
					return Service{}
				},
				expectedPanic: "the type parameter gosyringe.IService is not the same as the return type of the constructor gosyringe.Service",
			},
		}

		for _, tt := range testData {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				// https://stackoverflow.com/a/31596110
				defer func() {
					actualPanic := recover()
					assert.Equal(t, tt.expectedPanic, actualPanic)
				}()

				c := NewContainer()

				RegisterSingleton[IService](c, tt.constructor)
			})
		}

	})

	t.Run("should accept custom error return", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IService](c, func() (IService, *CustomError) {
			return &Service{}, nil
		})
	})

	t.Run("should return error on Resolve", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IService](c, func() (IService, error) {
			return nil, customError
		})

		_, err := Resolve[IService](c)

		assert.ErrorIs(t, err, customError)
	})

	t.Run("should resolve constructor with multiple parameters", func(t *testing.T) {
		c := NewContainer()
		RegisterSingleton[IServiceOne](c, NewServiceOne)
		RegisterSingleton[IServiceTwo](c, NewServiceTwo)
		RegisterSingleton[IServiceThree](c, NewServiceThree)
		RegisterSingleton[IServiceFive](c, NewServiceFive)

		service, err := Resolve[IServiceFive](c)

		assert.NoError(t, err)
		assert.Equal(t, 5, service.GetValueFive())
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
func NewServiceUnsafe() (IService, error) {
	return &Service{}, nil
}

type CustomError struct{}

func (c CustomError) Error() string {
	return "custom error"
}

var customError = &CustomError{}

type IServiceOne interface {
	GetValueOne() int
}
type ServiceOne struct{}

func NewServiceOne() IServiceOne {
	return &ServiceOne{}
}

func (s ServiceOne) GetValueOne() int {
	return 1
}

type IServiceTwo interface {
	GetValueTwo() int
}
type ServiceTwo struct{}

func NewServiceTwo() (IServiceTwo, error) {
	return &ServiceTwo{}, nil
}

func (s ServiceTwo) GetValueTwo() int {
	return 2
}

type IServiceThree interface {
	GetValueThree() int
}
type ServiceThree struct {
	serviceOne IServiceOne
	serviceTwo IServiceTwo
}

func NewServiceThree(serviceOne IServiceOne, serviceTwo IServiceTwo) IServiceThree {
	return &ServiceThree{
		serviceOne,
		serviceTwo,
	}
}

func (s ServiceThree) GetValueThree() int {
	return s.serviceOne.GetValueOne() + s.serviceTwo.GetValueTwo()
}

type IServiceFive interface {
	GetValueFive() int
}
type ServiceFive struct {
	serviceTwo   IServiceTwo
	serviceThree IServiceThree
}

func NewServiceFive(serviceOne IServiceTwo, serviceThree IServiceThree) IServiceFive {
	return &ServiceFive{
		serviceOne,
		serviceThree,
	}
}

func (s ServiceFive) GetValueFive() int {
	return s.serviceTwo.GetValueTwo() + s.serviceThree.GetValueThree()
}
