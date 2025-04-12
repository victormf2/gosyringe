package main

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/victormf2/gosyringe"
	"github.com/victormf2/gosyringe/internal/examples/gin/handlers"
	"github.com/victormf2/gosyringe/internal/examples/gin/setup"
)

func main() {
	// First create the root Container
	c := gosyringe.NewContainer()

	// Then register all dependencies. You can do the registrations inline,
	// but it's usually better to separate that logic in a function, so you
	// can use it in your tests.
	setup.RegisterServices(c)

	// Setting up gin with the ContainerMiddleware
	router := gin.Default()
	router.Use(ContainerMiddleware(c))

	router.GET("/users/:id", func(c *gin.Context) {
		// You don't need to manually instantiate your handlers,
		// just leave it to gosyringe to do the work for you.
		handler, err := ResolveHandler[*handlers.GetUserByIDHandler](c)
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
		handler, err := ResolveHandler[*handlers.CreateUserHandler](c)
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
		handler, err := ResolveHandler[*handlers.UpdateUserHandler](c)
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
		handler, err := ResolveHandler[*handlers.DeleteUserHandler](c)
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

	router.Run() // listen and serve on 0.0.0.0:8080
}
