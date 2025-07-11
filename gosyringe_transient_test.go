package gosyringe

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func Print(n any) string {
	return fmt.Sprint(n)
}

func TestRegisterTransient(t *testing.T) {
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
				RegisterTransient[IService](container, tt.constructor)
				service, err := Resolve[IService](container)
				require.NoError(t, err)

				value := service.GetValue()

				require.IsType(t, &Service{}, service)
				require.Equal(t, 12, value)
			})
		}
	})

	t.Run("should always resolve new instances", func(t *testing.T) {
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

				RegisterTransient[IService](c, func() IService {
					time.Sleep(10 * time.Millisecond)
					mu.Lock()
					constructorCallCount++
					mu.Unlock()

					service := reflect.ValueOf(tt.constructor).Call(nil)[0]
					return service.Interface().(IService)
				})

				const goroutines = 50
				var wg sync.WaitGroup
				results := make(chan IService, goroutines)

				for range goroutines {
					wg.Add(1)
					go func() {
						defer wg.Done()
						instance, err := Resolve[IService](c)
						require.NoError(t, err, "resolve failed")

						results <- instance
					}()
				}

				wg.Wait()
				close(results)

				analyzedInstances := []IService{}
				for instance := range results {
					for _, previousInstance := range analyzedInstances {
						if instance == previousInstance {
							fmt.Print(Print(instance))
							t.Errorf("expected different instances, got equal ones")
						}
					}
					analyzedInstances = append(analyzedInstances, instance)
				}

				require.Equal(t, 50, constructorCallCount)
			})
		}
	})

	t.Run("should resolve the last registered instance", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterTransient[IService](c, NewServiceError)
		RegisterTransient[IService](c, NewService)
		RegisterTransient[IService](c, NewOtherService)

		service, err := Resolve[IService](c)
		require.NoError(t, err)

		value := service.GetValue()

		require.IsType(t, &OtherService{}, service)
		require.Equal(t, 13, value)
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
				expectedPanic: "the type parameter gosyringe.IService must be equal to the return type of the constructor gosyringe.Service",
			},
		}

		for _, tt := range testData {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				c := NewContainer()

				require.PanicsWithError(t, tt.expectedPanic, func() {
					RegisterTransient[IService](c, tt.constructor)
				})
			})
		}
	})

	t.Run("should accept custom error return", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterTransient[IService](c, func() (IService, *CustomError) {
			return &Service{}, nil
		})
	})

	t.Run("should return error on Resolve", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterTransient[IService](c, NewServiceError)

		_, err := Resolve[IService](c)

		require.ErrorIs(t, err, customError)
	})

	t.Run("should resolve constructor with multiple parameters", func(t *testing.T) {
		c := NewContainer()
		RegisterTransient[IServiceOne](c, NewServiceOne)
		RegisterTransient[IServiceTwo](c, NewServiceTwo)
		RegisterTransient[IServiceThree](c, NewServiceThree)
		RegisterTransient[IServiceFive](c, NewServiceFive)

		service, err := Resolve[IServiceFive](c)

		require.NoError(t, err)
		require.Equal(t, 5, service.GetValueFive())
	})

	t.Run("with slice resolution", func(t *testing.T) {
		t.Parallel()

		t.Run("should resolve the correct type", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterTransient[IService](c, NewService)
			RegisterTransient[IService](c, NewServiceUnsafe)
			RegisterTransient[IService](c, NewOtherService)
			services, err := Resolve[[]IService](c)
			require.NoError(t, err)

			value := 0
			for _, service := range services {
				value += service.GetValue()
			}

			require.IsType(t, &Service{}, services[0])
			require.IsType(t, &Service{}, services[1])
			require.IsType(t, &OtherService{}, services[2])
			require.Equal(t, 37, value)
		})

		t.Run("should always resolve new instances", func(t *testing.T) {
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
				RegisterTransient[IService](c, func() IService {
					time.Sleep(10 * time.Millisecond)
					mu.Lock()
					constructorCallCount[i]++
					mu.Unlock()

					service := reflect.ValueOf(constructors[i]).Call(nil)[0]
					return service.Interface().(IService)
				})
			}

			const goroutines = 50
			var wg sync.WaitGroup
			results := make(chan []IService, goroutines)

			for range goroutines {
				wg.Add(1)
				go func() {
					defer wg.Done()
					instance, err := Resolve[[]IService](c)
					require.NoError(t, err, "resolve failed")

					results <- instance
				}()
			}

			wg.Wait()
			close(results)

			analyzedInstances := [][]IService{}
			for instance := range results {
				for _, previousInstance := range analyzedInstances {
					if reflect.DeepEqual(instance, previousInstance) {
						t.Errorf("expected different instance, got equal ones")
					}
				}
				analyzedInstances = append(analyzedInstances, instance)
			}

			require.Len(t, analyzedInstances[0], 3)

			require.Equal(t, 50, constructorCallCount[0])
			require.Equal(t, 50, constructorCallCount[1])
			require.Equal(t, 50, constructorCallCount[2])
		})

		t.Run("should work as parameter injection", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()

			RegisterTransient[IService](c, NewService)
			RegisterTransient[IService](c, NewOtherService)
			RegisterTransient[MultiServiceInjection](c, NewMultiServiceInjection)

			multiService, err := Resolve[MultiServiceInjection](c)
			require.NoError(t, err)

			values := multiService.GetMultiValue()
			require.Equal(t, []int{12, 13}, values)
		})

		t.Run("should return error on Resolve", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterTransient[IService](c, NewService)
			RegisterTransient[IService](c, NewServiceError)

			_, err := Resolve[[]IService](c)

			require.ErrorIs(t, err, customError)
		})
	})

	t.Run("should detect circular dependency", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterTransient[IService](c, NewSeeminglyHarmlessService)
		RegisterTransient[CircularDependency](c, NewCircularDependency)

		_, err := Resolve[IService](c)
		require.EqualError(t, err, "circular dependency detected: gosyringe.IService -> gosyringe.CircularDependency -> gosyringe.IService")
	})
}
