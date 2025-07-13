package setup

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/victormf2/gosyringe"
	"github.com/victormf2/gosyringe/internal/examples/gin/handlers"
)

func RegisterHttpServer(c *gosyringe.Container) {
	gosyringe.RegisterSingleton[*gin.Engine](c, func(c *gosyringe.Container) *gin.Engine {
		router := gin.Default()

		middlewares, err := gosyringe.ResolveAllWithKey[gin.HandlerFunc](c, "middleware")
		if err != nil {
			panic(fmt.Errorf("failed to resolve middlewares: %w", err))
		}
		router.Use(middlewares...)

		router.GET("/users/:id", func(c *gin.Context) {
			// You don't need to manually instantiate your handlers,
			// just leave it to gosyringe to do the work for you.
			handler, err := resolveHandler[*handlers.GetUserByIDHandler](c)
			if err != nil {
				c.JSON(500, gin.H{"msg": err.Error()})
				return
			}

			input := &handlers.GetUserByIDInput{}
			err = c.ShouldBindUri(input)
			if err != nil {
				c.JSON(400, gin.H{"msg": err.Error()})
				return
			}

			output, err := handler.Handle(input)
			if err != nil {
				if errors.Is(err, handlers.ErrUserNotFound) {
					c.JSON(404, gin.H{"msg": err.Error()})
				} else {
					c.JSON(500, gin.H{"msg": err.Error()})
				}
				return
			}

			c.JSON(http.StatusOK, output)
		})

		router.POST("/users", func(c *gin.Context) {
			handler, err := resolveHandler[*handlers.CreateUserHandler](c)
			if err != nil {
				c.JSON(500, gin.H{"msg": err.Error()})
				return
			}

			input := &handlers.CreateUserInput{}
			err = c.ShouldBindJSON(input)
			if err != nil {
				c.JSON(400, gin.H{"msg": err.Error()})
				return
			}

			output, err := handler.Handle(input)
			if err != nil {
				c.JSON(500, gin.H{"msg": err.Error()})
				return
			}

			c.JSON(http.StatusOK, output)
		})

		router.PUT("/users", func(c *gin.Context) {
			handler, err := resolveHandler[*handlers.UpdateUserHandler](c)
			if err != nil {
				c.JSON(500, gin.H{"msg": err.Error()})
				return
			}

			input := &handlers.UpdateUserInput{}
			err = c.ShouldBindJSON(input)
			if err != nil {
				c.JSON(400, gin.H{"msg": err.Error()})
				return
			}

			output, err := handler.Handle(input)
			if err != nil {
				c.JSON(500, gin.H{"msg": err.Error()})
				return
			}

			c.JSON(http.StatusOK, output)
		})

		router.DELETE("/users/:id", func(c *gin.Context) {
			handler, err := resolveHandler[*handlers.DeleteUserHandler](c)
			if err != nil {
				c.JSON(500, gin.H{"msg": err.Error()})
				return
			}

			input := &handlers.DeleteUserInput{}
			err = c.ShouldBindUri(input)
			if err != nil {
				c.JSON(400, gin.H{"msg": err.Error()})
				return
			}

			output, err := handler.Handle(input)
			if err != nil {
				c.JSON(500, gin.H{"msg": err.Error()})
				return
			}

			c.JSON(http.StatusOK, output)
		})

		return router
	})

	// Middlewares registration order matters, because they get resolved with ResolveAll
	gosyringe.RegisterSingletonWithKey[gin.HandlerFunc](c, "middleware", func() gin.HandlerFunc {
		return requestLogMiddleware
	})
	gosyringe.RegisterSingletonWithKey[gin.HandlerFunc](c, "middleware", func(c *gosyringe.Container) gin.HandlerFunc {
		return containerMiddleware(c)
	})

	gosyringe.RegisterSingleton[*http.Server](c, func(router *gin.Engine) *http.Server {
		server := &http.Server{
			Addr:    ":8080",
			Handler: router,
		}
		return server
	})
	gosyringe.OnStart(c, func(server *http.Server) {
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	})
	gosyringe.OnDispose(c, func(ctx context.Context, server *http.Server) {
		err := server.Shutdown(ctx)
		if err != nil {
			log.Fatal("Server forced to shutdown: ", err)
		}
	})
}

func requestLogMiddleware(c *gin.Context) {
	start := time.Now()

	// Process request
	c.Next()

	duration := time.Since(start)
	status := c.Writer.Status()
	method := c.Request.Method
	path := c.Request.URL.Path
	clientIP := c.ClientIP()

	log.Printf("[GIN] %v | %3d | %13v | %15s | %-7s %s\n",
		start.Format("2006/01/02 - 15:04:05"),
		status,
		duration,
		clientIP,
		method,
		path,
	)
}

// This hooks up gin handler functions with a child container.
// This way every request has a separate scope.
func containerMiddleware(rootContainer *gosyringe.Container) gin.HandlerFunc {
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
func resolveHandler[T any](c *gin.Context) (T, error) {
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
