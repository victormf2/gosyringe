package main

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/victormf2/gosyringe"
)

// This hooks up gin handler functions with a child container.
// This way every request has a separate scope.
func ContainerMiddleware(rootContainer *gosyringe.Container) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// First create child container
		c := gosyringe.CreateChildContainer(rootContainer)

		// Then register specific context dependencies to be used by Scoped services.

		gosyringe.RegisterValue(c, ctx)

		logger := log.WithFields(log.Fields{
			"request_id": uuid.NewString(),
		})
		gosyringe.RegisterValue(c, logger)

		// Store the container in gin context to retrieve it in the handler functions.
		ctx.Set("container", c)

		ctx.Next()
	}
}

var ErrNoContainerInContext = errors.New("no container in context")

// This is a helper function to retrieve the required instance
// at a gin handler function.
func ResolveHandler[T any](c *gin.Context) (T, error) {
	var zero T
	value, found := c.Get("container")
	if !found {
		return zero, ErrNoContainerInContext
	}

	container, ok := value.(*gosyringe.Container)
	if !ok {
		return zero, ErrNoContainerInContext
	}

	handler, err := gosyringe.Resolve[T](container)
	if err != nil {
		return zero, err
	}

	return handler, nil
}
