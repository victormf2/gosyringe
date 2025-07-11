package gosyringe

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/victormf2/gosyringe/internal"
)

func TestCombineScopes(t *testing.T) {
	t.Parallel()

	t.Run("should panic when registering different scopes", func(t *testing.T) {
		t.Parallel()

		type RegisterFunction func(c *Container, constructor any)

		testData := []struct {
			name          string
			first         RegisterFunction
			second        RegisterFunction
			expectedPanic string
		}{
			{
				name:          "transient, singleton",
				first:         RegisterTransient[IService],
				second:        RegisterSingleton[IService],
				expectedPanic: "cannot register type gosyringe.IService as Singleton, because it was already registered as Transient",
			},
			{
				name:          "transient, scoped",
				first:         RegisterTransient[IService],
				second:        RegisterScoped[IService],
				expectedPanic: "cannot register type gosyringe.IService as Scoped, because it was already registered as Transient",
			},
			{
				name:          "scoped, transient",
				first:         RegisterScoped[IService],
				second:        RegisterTransient[IService],
				expectedPanic: "cannot register type gosyringe.IService as Transient, because it was already registered as Scoped",
			},
			{
				name:          "scoped, transient",
				first:         RegisterScoped[IService],
				second:        RegisterSingleton[IService],
				expectedPanic: "cannot register type gosyringe.IService as Singleton, because it was already registered as Scoped",
			},
			{
				name:          "singleton, transient",
				first:         RegisterSingleton[IService],
				second:        RegisterTransient[IService],
				expectedPanic: "cannot register type gosyringe.IService as Transient, because it was already registered as Singleton",
			},
			{
				name:          "singleton, scoped",
				first:         RegisterSingleton[IService],
				second:        RegisterScoped[IService],
				expectedPanic: "cannot register type gosyringe.IService as Scoped, because it was already registered as Singleton",
			},
		}

		for _, tt := range testData {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				c := NewContainer()

				tt.first(c, NewService)

				require.PanicsWithError(t, tt.expectedPanic, func() {
					tt.second(c, NewService)
				})
			})
		}
	})

	t.Run("injection lifetime rules", func(t *testing.T) {
		t.Parallel()

		t.Run("can inject Transient into Transient", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterTransient[Injecting](c, NewInjecting)
			RegisterTransient[ReceivingInjection](c, NewReceivingInjection)

			_, err := Resolve[ReceivingInjection](c)
			require.NoError(t, err)
		})
		t.Run("can inject Scoped into Transient", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterScoped[Injecting](c, NewInjecting)
			RegisterTransient[ReceivingInjection](c, NewReceivingInjection)

			_, err := Resolve[ReceivingInjection](CreateChildContainer(c))
			require.NoError(t, err)
		})
		t.Run("can inject Singleton into Transient", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterSingleton[Injecting](c, NewInjecting)
			RegisterTransient[ReceivingInjection](c, NewReceivingInjection)

			_, err := Resolve[ReceivingInjection](c)
			require.NoError(t, err)
		})

		t.Run("can inject Transient into Scoped", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterTransient[Injecting](c, NewInjecting)
			RegisterScoped[ReceivingInjection](c, NewReceivingInjection)

			_, err := Resolve[ReceivingInjection](CreateChildContainer(c))
			require.NoError(t, err)
		})
		t.Run("can inject Scoped into Scoped", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterScoped[Injecting](c, NewInjecting)
			RegisterScoped[ReceivingInjection](c, NewReceivingInjection)

			_, err := Resolve[ReceivingInjection](CreateChildContainer(c))
			require.NoError(t, err)
		})
		t.Run("can inject Singleton into Scoped", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterSingleton[Injecting](c, NewInjecting)
			RegisterScoped[ReceivingInjection](c, NewReceivingInjection)

			_, err := Resolve[ReceivingInjection](CreateChildContainer(c))
			require.NoError(t, err)
		})

		t.Run("can inject Transient into Singleton", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterTransient[Injecting](c, NewInjecting)
			RegisterSingleton[ReceivingInjection](c, NewReceivingInjection)

			_, err := Resolve[ReceivingInjection](c)
			require.NoError(t, err)
		})
		t.Run("should not inject Scoped into Singleton", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterScoped[Injecting](c, NewInjecting)
			RegisterSingleton[ReceivingInjection](c, NewReceivingInjection)

			_, err := Resolve[ReceivingInjection](c)
			require.Error(t, err)
		})
		t.Run("should not inject Scoped into Singleton indirectly", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterScoped[Injecting](c, NewInjecting)
			RegisterTransient[ReceivingInjection](c, NewReceivingInjection)
			RegisterSingleton[ReceivingReceivingInjection](c, NewReceivingReceivingInjection)

			childContainer := CreateChildContainer(c)

			_, err := Resolve[ReceivingReceivingInjection](childContainer)
			require.Error(t, err)
		})
		t.Run("can inject Singleton into Singleton", func(t *testing.T) {
			t.Parallel()

			c := NewContainer()
			RegisterSingleton[Injecting](c, NewInjecting)
			RegisterSingleton[ReceivingInjection](c, NewReceivingInjection)

			_, err := Resolve[ReceivingInjection](c)
			require.NoError(t, err)
		})
	})

	t.Run("should resolve from deep level nesting of child containers", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()
		RegisterSingleton[IService](c, NewService)

		c1 := CreateChildContainer(c)
		c2 := CreateChildContainer(c1)
		c3 := CreateChildContainer(c2)

		_, err := Resolve[IService](c3)
		require.NoError(t, err)
	})

	t.Run("simulate real application services", func(t *testing.T) {
		t.Parallel()

		c := NewContainer()

		RegisterSingleton[ISomeSingletonExternalService](c, NewSomeSingletonExternalService)
		RegisterTransient[IDate](c, NewMarsDate)
		RegisterScoped[RequestContext](c, NewRequestContext)
		RegisterScoped[RequestHandler](c, NewRequestHandler)

		singleton0, err := Resolve[ISomeSingletonExternalService](c)
		require.NoError(t, err)

		transient0, err := Resolve[IDate](c)
		require.NoError(t, err)

		child0 := CreateChildContainer(c)

		handler0, err := Resolve[RequestHandler](child0)
		require.NoError(t, err)
		handler1, err := Resolve[RequestHandler](child0)
		require.NoError(t, err)

		child1 := CreateChildContainer(c)

		handler2, err := Resolve[RequestHandler](child1)
		require.NoError(t, err)

		singleton1 := handler0.service
		singleton2 := handler1.service
		singleton3 := handler2.service

		transient1 := handler0.date
		transient2 := handler1.date
		transient3 := handler2.date

		require.Exactly(t, handler0, handler1)  // handlers from same child container
		require.NotEqual(t, handler1, handler2) // handlers from different child container

		require.Exactly(t, handler0.ctx.RequestId, handler1.ctx.RequestId)  // handlers from same child container
		require.NotEqual(t, handler1.ctx.RequestId, handler2.ctx.RequestId) // handlers from different child container

		require.Equal(t, "thing1 thing2 2025-01-02", handler0.Handle())
		require.Equal(t, "thing1 thing2 2025-01-02", handler1.Handle())
		require.Equal(t, "thing1 thing2 2025-01-03", handler2.Handle())

		require.Exactly(t, singleton0, singleton1) // singleton should always be equal
		require.Exactly(t, singleton1, singleton2) // singleton should always be equal
		require.Exactly(t, singleton2, singleton3) // singleton should always be equal

		require.NotEqual(t, transient0, transient1) // 0 -> independent, 1 -> handler0
		require.Exactly(t, transient1, transient2)  // 1 -> handler0, 2 -> handler1, handler0 == handler1
		require.NotEqual(t, transient2, transient3) // 2 -> handler1, 3 -> handler2, handler1 != handler2
	})
}

type Injecting struct{}

func NewInjecting() Injecting {
	return Injecting{}
}

type ReceivingInjection struct{}

func NewReceivingInjection(injecting Injecting) ReceivingInjection {
	return ReceivingInjection{}
}

type ReceivingReceivingInjection struct{}

func NewReceivingReceivingInjection(injecting ReceivingInjection) ReceivingReceivingInjection {
	return ReceivingReceivingInjection{}
}

type Thing struct {
	Title string
}
type ISomeSingletonExternalService interface {
	GetThings() []Thing
}
type SomeSingletonExternalService struct{}

func (s SomeSingletonExternalService) GetThings() []Thing {
	return []Thing{
		{
			Title: "thing1",
		},
		{
			Title: "thing2",
		},
	}
}

func NewSomeSingletonExternalService() ISomeSingletonExternalService {
	return &SomeSingletonExternalService{}
}

type IDate interface {
	GetDate() string
}

var (
	currentIndex = 0
	dates        = []string{
		"2025-01-01",
		"2025-01-02",
		"2025-01-03",
		"2025-01-04",
		"2025-01-05",
		"2025-01-06",
		"2025-01-07",
		"2025-01-08",
		"2025-01-09",
		"2025-01-10",
	}
)

type MarsDate struct {
	date string
}

func NewMarsDate() IDate {
	date := dates[currentIndex]
	currentIndex += 1
	return &MarsDate{date}
}

func (d *MarsDate) GetDate() string {
	return d.date
}

type RequestContext struct {
	RequestId string
}

func NewRequestContext() RequestContext {
	return RequestContext{
		RequestId: uuid.NewString(),
	}
}

type RequestHandler struct {
	service ISomeSingletonExternalService
	date    IDate
	ctx     RequestContext
}

func NewRequestHandler(service ISomeSingletonExternalService, date IDate, ctx RequestContext) RequestHandler {
	return RequestHandler{
		service,
		date,
		ctx,
	}
}

func (h RequestHandler) Handle() string {
	things := h.service.GetThings()
	titles := internal.Map(things, func(thing Thing) string { return thing.Title })

	value := fmt.Sprintf("%s %s", strings.Join(titles, " "), h.date.GetDate())

	return value
}
