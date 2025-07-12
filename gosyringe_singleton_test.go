package gosyringe

import (
	"fmt"
	"math/rand"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
				require.NoError(t, err)

				value := service.GetValue()

				require.IsType(t, &Service{}, service)
				require.Equal(t, 12, value)
			})
		}
	})

	t.Run("should resolve a single instance", func(t *testing.T) {
		t.Parallel()

		testData := []struct {
			name        string
			constructor any
			fromChild   bool
		}{
			// {
			// 	name:        "safe constructor",
			// 	constructor: NewService,
			// },
			// {
			// 	name:        "unsafe constructor",
			// 	constructor: NewServiceUnsafe,
			// },
			// {
			// 	name:        "safe constructor from child",
			// 	constructor: NewService,
			// 	fromChild:   true,
			// },
			{
				name:        "unsafe constructor from child",
				constructor: NewServiceUnsafe,
				fromChild:   true,
			},
		}

		for _, tt := range testData {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				c := NewContainer()

				var constructorCallCount atomic.Uint32

				RegisterSingleton[IService](c, func() IService {
					time.Sleep(10 * time.Millisecond)

					constructorCallCount.Add(1)

					service := reflect.ValueOf(tt.constructor).Call(nil)[0]
					return service.Interface().(IService)
				})

				const goroutines = 100
				containers := []*Container{}
				if tt.fromChild {
					for range goroutines / 2 {
						containers = append(containers, c)
					}
					for range goroutines / 2 {
						containers = append(containers, CreateChildContainer(c))
					}
					rand.Shuffle(goroutines, func(i, j int) {
						tmp := containers[i]
						containers[i] = containers[j]
						containers[j] = tmp
					})
				} else {
					for range goroutines {
						containers = append(containers, c)
					}
				}

				var wg sync.WaitGroup
				results := make(chan IService, goroutines)

				for i := range goroutines {
					wg.Add(1)
					go func(c *Container) {
						defer wg.Done()
						instance, err := Resolve[IService](c)
						require.NoError(t, err, "resolve failed")

						results <- instance
					}(containers[i])
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

				require.Equal(t, uint32(1), constructorCallCount.Load())
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

				c := NewContainer()

				require.PanicsWithError(t, tt.expectedPanic, func() {
					RegisterSingleton[IService](c, tt.constructor)
				})
			})
		}
	})

	t.Run("should panic when registering Singleton from child container", func(t *testing.T) {
		c := CreateChildContainer(NewContainer())

		require.PanicsWithError(t, "Singleton can only be registered at a root container: gosyringe.IService", func() {
			RegisterSingleton[IService](c, NewService)
		})
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

		require.ErrorIs(t, err, customError)
	})

	t.Run("should resolve constructor with multiple parameters", func(t *testing.T) {
		c := NewContainer()
		RegisterSingleton[IServiceOne](c, NewServiceOne)
		RegisterSingleton[IServiceTwo](c, NewServiceTwo)
		RegisterSingleton[IServiceThree](c, NewServiceThree)
		RegisterSingleton[IServiceFive](c, NewServiceFive)

		service, err := Resolve[IServiceFive](c)

		require.NoError(t, err)
		require.Equal(t, 5, service.GetValueFive())
	})

	t.Run("with slice resolution", func(t *testing.T) {
		t.Parallel()

		t.Run("should resolve the correct type", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterSingleton[IService](c, NewService)
			RegisterSingleton[IService](c, NewServiceUnsafe)
			RegisterSingleton[IService](c, NewOtherService)
			services, err := ResolveAll[IService](c)
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
					instance, err := ResolveAll[IService](c)
					require.NoError(t, err, "resolve failed")

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

			require.Len(t, firstInstance, 3)

			require.Equal(t, 1, constructorCallCount[0])
			require.Equal(t, 1, constructorCallCount[1])
			require.Equal(t, 1, constructorCallCount[2])
		})

		t.Run("should work as parameter injection", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()

			RegisterSingleton[IService](c, NewService)
			RegisterSingleton[IService](c, NewOtherService)
			RegisterSingleton[MultiServiceInjection](c, NewMultiServiceInjection)

			multiService, err := Resolve[MultiServiceInjection](c)
			require.NoError(t, err)

			values := multiService.GetMultiValue()
			require.Equal(t, []int{12, 13}, values)
		})

		t.Run("should return error on Resolve", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterSingleton[IService](c, NewService)
			RegisterSingleton[IService](c, NewServiceError)

			_, err := ResolveAll[IService](c)

			require.ErrorIs(t, err, customError)
		})
	})

	t.Run("should detect circular dependency", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IService](c, NewSeeminglyHarmlessService)
		RegisterSingleton[CircularDependency](c, NewCircularDependency)

		_, err := Resolve[IService](c)
		require.EqualError(t, err, "circular dependency detected: gosyringe.IService -> gosyringe.CircularDependency -> gosyringe.IService")
	})
}
