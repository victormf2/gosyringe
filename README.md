# gosyringe - Go dependency injection

gosyringe is a depency injection library for Go inspired in [tsyringe](https://github.com/microsoft/tsyringe).

## Dependency injection in Go? Why do I need this?

Well, if you don't want to waste your time setting up your application entrypoint by repeatedly passing repositories and services drilling down constructor calls, then this library is for you.

This is also very useful for test setups.

Don't take my word for it, go see the examples.

## Getting Started

### Prerequisites

gosyring requires Go version v1.24 or above.

### Installation

```
go get -u github.com/gin-gonic/gin
```

### Usage example

To use gosyringe, you register your services and configurations in a container, then you resolve the dependencies from the container to use them.

```go
package main

import (
	"fmt"

	"github.com/victormf2/gosyringe"
)

type Something struct {
	Id    int
	Title string
}

type IService interface {
	DoSomething(something Something) error
}

type ISomethingRepository interface {
	SaveSomething(something Something) error
}

type Service struct {
	repository ISomethingRepository
}

func NewService(repository ISomethingRepository) IService {
	return Service{
		repository,
	}
}

func (s Service) DoSomething(something Something) error {
	fmt.Printf("doing something %s", something.Title)
	return s.repository.SaveSomething(something)
}

type SomethingRepository struct{}

func NewSomethingReposiory() ISomethingRepository {
	return SomethingRepository{}
}

func (r SomethingRepository) SaveSomething(something Something) error {
	fmt.Printf("saving something %s", something.Title)
	return nil
}

func main() {
	c := gosyringe.NewContainer()

	gosyringe.RegisterSingleton[IService](c, NewService)
	gosyringe.RegisterSingleton[ISomethingRepository](c, NewSomethingReposiory)

	service, err := gosyringe.Resolve[IService](c)
	if err != nil {
		panic(fmt.Sprintf("failed to resolve service: %v", err))
	}

	something := Something{
		Id:    1,
		Title: "😏😏",
	}
	err = service.DoSomething(something)
	if err != nil {
		panic(err)
	}
}
```

## Dependency Lifetimes

### Transient

### Singleton

### Scoped

## Resolution

### Single

### Slice

### Injection tokens

Injection tokens are a way to enable us to register different constructors for the same type or interface. In other dependency injection frameworks it's usually done by providing a value (e.g. a string like "MyServiceToken") which maps to a constructor. Different values can provide the same instance type.

But in Go there is another option called [Type Definitions](https://go.dev/ref/spec#Type_definitions). Although it's more a Go feature itself than gosyringe, it is worth pointing out it's possible.

```go
package main

import (
	"fmt"

	"github.com/victormf2/gosyringe"
)

type IService interface {
	GetSomething() string
}

// This is the Type Definitions syntax.
// For Go, IOneService and IAnotherService are two distinct types.
// So when we call Resolve[IOneService] we get a different instance
// from Resolve[IAnotherService]
type IOneService IService
type IAnotherService IService

type OneService struct{}

func NewOneService() IOneService {
	return &OneService{}
}
func (s *OneService) GetSomething() string {
	return "one"
}

type AnotherService struct{}

func NewAnotherService() IAnotherService {
	return &AnotherService{}
}
func (s *AnotherService) GetSomething() string {
	return "another"
}

func main() {
	c := gosyringe.NewContainer()

	gosyringe.RegisterSingleton[IOneService](c, NewOneService)
	gosyringe.RegisterSingleton[IAnotherService](c, NewAnotherService)

	oneService, _ := gosyringe.Resolve[IOneService](c)
	anotherService, _ := gosyringe.Resolve[IAnotherService](c)

	fmt.Println(oneService.GetSomething())     // one
	fmt.Println(anotherService.GetSomething()) // another
}
```