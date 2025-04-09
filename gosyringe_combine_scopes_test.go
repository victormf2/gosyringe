package gosyringe

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCombineScopes(t *testing.T) {
	t.Parallel()

	t.Run("should panic when registering different scopes", func(t *testing.T) {
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
				name:          "singleton, transient",
				first:         RegisterSingleton[IService],
				second:        RegisterTransient[IService],
				expectedPanic: "cannot register type gosyringe.IService as Transient, because it was already registered as Singleton",
			},
		}

		for _, tt := range testData {
			tt := tt
			t.Run(tt.name, func(t *testing.T) {
				c := NewContainer()

				defer func() {
					actualPanic := recover()
					assert.Equal(t, tt.expectedPanic, actualPanic)
				}()

				tt.first(c, NewService)
				tt.second(c, NewService)
			})
		}
	})
}
