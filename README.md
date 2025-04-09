# gosyringe - Go dependency injection

gosyringe is a depency injection library for Go inspired in [tsyringe](https://github.com/microsoft/tsyringe).

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

	gosyringe.RegisterSingleton(c, NewService)
	gosyringe.RegisterSingleton(c, NewSomethingReposiory)

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