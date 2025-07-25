package gosyringe

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRegisterScoped(t *testing.T) {
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
				RegisterScoped[IService](container, tt.constructor)
				service, err := Resolve[IService](CreateChildContainer(container))
				require.NoError(t, err)

				value := service.GetValue()

				require.IsType(t, &Service{}, service)
				require.Equal(t, 12, value)
			})
		}
	})

	t.Run("should resolve a single instance on same container", func(t *testing.T) {
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

				RegisterScoped[IService](c, func() IService {
					time.Sleep(10 * time.Millisecond)
					mu.Lock()
					constructorCallCount++
					mu.Unlock()

					service := reflect.ValueOf(tt.constructor).Call(nil)[0]
					return service.Interface().(IService)
				})

				childContainer := CreateChildContainer(c)

				const goroutines = 100
				var wg sync.WaitGroup
				results := make(chan IService, goroutines)

				for range goroutines {
					wg.Add(1)
					go func() {
						defer wg.Done()
						instance, err := Resolve[IService](childContainer)
						require.NoError(t, err, "resolve failed")

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

				require.Equal(t, 1, constructorCallCount)
			})
		}
	})

	t.Run("should resolve different instances on different containers", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterScoped[IService](c, NewService)

		child := CreateChildContainer(c)
		otherChild := CreateChildContainer(c)
		grandChild := CreateChildContainer(child)

		serviceChild, err := Resolve[IService](child)
		require.NoError(t, err)
		serviceOtherChild, err := Resolve[IService](otherChild)
		require.NoError(t, err)
		serviceGrandChild, err := Resolve[IService](grandChild)
		require.NoError(t, err)

		require.NotSame(t, serviceChild, serviceOtherChild)
		require.NotSame(t, serviceChild, serviceGrandChild)
	})

	t.Run("should resolve the last registered instance", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterScoped[IService](c, NewServiceError)
		RegisterScoped[IService](c, NewService)
		RegisterScoped[IService](c, NewOtherService)

		service, err := Resolve[IService](CreateChildContainer(c))
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
					RegisterScoped[IService](c, tt.constructor)
				})
			})
		}
	})

	t.Run("should accept custom error return", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterScoped[IService](c, func() (IService, *CustomError) {
			return NewService(), nil
		})
	})

	t.Run("should return error on Resolve", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterScoped[IService](c, NewServiceError)

		_, err := Resolve[IService](CreateChildContainer(c))

		require.ErrorIs(t, err, customError)
	})

	t.Run("should resolve constructor with multiple parameters", func(t *testing.T) {
		c := NewContainer()
		RegisterScoped[IServiceOne](c, NewServiceOne)
		RegisterScoped[IServiceTwo](c, NewServiceTwo)
		RegisterScoped[IServiceThree](c, NewServiceThree)
		RegisterScoped[IServiceFive](c, NewServiceFive)

		service, err := Resolve[IServiceFive](CreateChildContainer(c))

		require.NoError(t, err)
		require.Equal(t, 5, service.GetValueFive())
	})

	t.Run("with slice resolution", func(t *testing.T) {
		t.Parallel()

		t.Run("should resolve the correct type", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterScoped[IService](c, NewService)
			RegisterScoped[IService](c, NewServiceUnsafe)
			RegisterScoped[IService](c, NewOtherService)
			services, err := ResolveAll[IService](CreateChildContainer(c))
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

		t.Run("should resolve a single instance on same container", func(t *testing.T) {
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
				RegisterScoped[IService](c, func() IService {
					time.Sleep(10 * time.Millisecond)
					mu.Lock()
					constructorCallCount[i]++
					mu.Unlock()

					service := reflect.ValueOf(constructors[i]).Call(nil)[0]
					return service.Interface().(IService)
				})
			}

			childContainer := CreateChildContainer(c)

			const goroutines = 100
			var wg sync.WaitGroup
			results := make(chan []IService, goroutines)

			for range goroutines {
				wg.Add(1)
				go func() {
					defer wg.Done()
					instance, err := ResolveAll[IService](childContainer)
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

			RegisterScoped[IService](c, NewService)
			RegisterScoped[IService](c, NewOtherService)
			RegisterScoped[MultiServiceInjection](c, NewMultiServiceInjection)

			multiService, err := Resolve[MultiServiceInjection](CreateChildContainer(c))
			require.NoError(t, err)

			values := multiService.GetMultiValue()
			require.Equal(t, []int{12, 13}, values)
		})

		t.Run("should return error on Resolve", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterScoped[IService](c, NewService)
			RegisterScoped[IService](c, NewServiceError)

			_, err := ResolveAll[IService](CreateChildContainer(c))

			require.ErrorIs(t, err, customError)
		})
	})

	t.Run("should detect circular dependency", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterScoped[IService](c, NewSeeminglyHarmlessService)
		RegisterScoped[CircularDependency](c, NewCircularDependency)

		_, err := Resolve[IService](CreateChildContainer(c))
		require.EqualError(t, err, "circular dependency detected: gosyringe.IService -> gosyringe.CircularDependency -> gosyringe.IService")
	})

	t.Run("should not allow resolution from root container", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterScoped[IService](c, NewService)

		_, err := Resolve[IService](c)

		require.EqualError(t, err, "cannot resolve Scoped dependencies from the root Container: gosyringe.IService")
	})

	t.Run("child container registrations take precedence", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterTransient[IService](c, NewService)

		childContainer := CreateChildContainer(c)
		RegisterScoped[IService](childContainer, NewOtherService)

		grandChildContainer := CreateChildContainer(childContainer)
		RegisterScoped[IService](grandChildContainer, NewService)

		serviceRoot, err := Resolve[IService](c)
		require.NoError(t, err)
		serviceChild, err := Resolve[IService](childContainer)
		require.NoError(t, err)
		serviceGrandChild, err := Resolve[IService](grandChildContainer)
		require.NoError(t, err)

		require.Equal(t, 12, serviceRoot.GetValue())
		require.Equal(t, 13, serviceChild.GetValue())
		require.Equal(t, 12, serviceGrandChild.GetValue())
	})
}
