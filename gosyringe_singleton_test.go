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

	t.Run("should resolve the last registered instance", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingleton[IService](c, NewServiceError)
		RegisterSingleton[IService](c, NewService)
		RegisterSingleton[IService](c, NewOtherService)

		service, err := Resolve[IService](c)
		assert.NoError(t, err)

		value := service.GetValue()

		assert.IsType(t, &OtherService{}, service)
		assert.Equal(t, 13, value)
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
					return NewService(), NewService()
				},
				expectedPanic: "constructor must be a function returning exactly one value, or a value and an error",
			},
			{
				name: "more than 2 return values",
				constructor: func() (IService, IService, error) {
					return NewService(), NewService(), fmt.Errorf("some error")
				},
				expectedPanic: "constructor must be a function returning exactly one value, or a value and an error",
			},
			{
				name: "constructor type mismatch",
				constructor: func() Service {
					return Service{}
				},
				expectedPanic: "the type parameter gosyringe.IService must be equal to the return type of the constructor gosyringe.Service",
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

	t.Run("should panic when registering Singleton from child container", func(t *testing.T) {
		defer func() {
			actualPanic := recover()
			assert.Equal(t, "Singletons can only be registered at a root container: gosyringe.IService", actualPanic)
		}()

		c := CreateChildContainer(NewContainer())
		RegisterSingleton[IService](c, NewService)
	})

	t.Run("should accept custom error return", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IService](c, func() (IService, *CustomError) {
			return NewService(), nil
		})
	})

	t.Run("should return error on Resolve", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IService](c, NewServiceError)

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

	t.Run("with slice resolution", func(t *testing.T) {
		t.Parallel()

		t.Run("should resolve the correct type", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterSingleton[IService](c, NewService)
			RegisterSingleton[IService](c, NewServiceUnsafe)
			RegisterSingleton[IService](c, NewOtherService)
			services, err := Resolve[[]IService](c)
			assert.NoError(t, err)

			value := 0
			for _, service := range services {
				value += service.GetValue()
			}

			assert.IsType(t, &Service{}, services[0])
			assert.IsType(t, &Service{}, services[1])
			assert.IsType(t, &OtherService{}, services[2])
			assert.Equal(t, 37, value)
		})

		t.Run("should resolve a single instance", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()

			constructorCallCount := [3]int{}
			constructors := [3]any{
				NewService,
				NewOtherService,
				NewServiceUnsafe,
			}
			var mu sync.Mutex

			for i := range 3 {
				i := i
				RegisterSingleton[IService](c, func() IService {
					time.Sleep(10 * time.Millisecond)
					mu.Lock()
					constructorCallCount[i]++
					mu.Unlock()

					service := reflect.ValueOf(constructors[i]).Call(nil)[0]
					return service.Interface().(IService)
				})
			}

			const goroutines = 100
			var wg sync.WaitGroup
			results := make(chan []IService, goroutines)

			for range goroutines {
				wg.Add(1)
				go func() {
					defer wg.Done()
					instance, err := Resolve[[]IService](c)
					assert.NoError(t, err, "resolve failed")

					results <- instance
				}()
			}

			wg.Wait()
			close(results)

			var firstInstance []IService
			for instance := range results {
				if firstInstance == nil {
					firstInstance = instance
				} else if !reflect.DeepEqual(instance, firstInstance) {
					t.Errorf("expected same instance, got different ones")
				}
			}

			assert.Len(t, firstInstance, 3)

			assert.Equal(t, 1, constructorCallCount[0])
			assert.Equal(t, 1, constructorCallCount[1])
			assert.Equal(t, 1, constructorCallCount[2])
		})

		t.Run("should work as parameter injection", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()

			RegisterSingleton[IService](c, NewService)
			RegisterSingleton[IService](c, NewOtherService)
			RegisterSingleton[MultiServiceInjection](c, NewMultiServiceInjection)

			multiService, err := Resolve[MultiServiceInjection](c)
			assert.NoError(t, err)

			values := multiService.GetMultiValue()
			assert.Equal(t, []int{12, 13}, values)
		})

		t.Run("should return error on Resolve", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterSingleton[IService](c, NewService)
			RegisterSingleton[IService](c, NewServiceError)

			_, err := Resolve[[]IService](c)

			assert.ErrorIs(t, err, customError)
		})
	})

	t.Run("should detect circular dependency", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IService](c, NewSeeminglyHarmlessService)
		RegisterSingleton[CircularDependency](c, NewCircularDependency)

		_, err := Resolve[IService](c)
		assert.EqualError(t, err, "circular dependency detected: gosyringe.IService -> gosyringe.CircularDependency -> gosyringe.IService")
	})
}
