# gosyringe - Go dependency injection

gosyringe is a depency injection library for Go inspired in [tsyringe](https://github.com/microsoft/tsyringe).

## Why do I need this?

This is good for:
- Application setups like wiring services constructor calls automatically
- Centralized configuration
- Makes it easy writing tests with mocks

Don't take my word for it, go see the [examples](./internal/examples/README.md).

## Getting Started

### Prerequisites

gosyringe requires Go version v1.24 or above.

### Installation

```
go get -u github.com/victormf2/gosyringe
```

### Usage example

To use gosyringe, you register your services and configurations in a container, then you resolve a service from the container to use it.

The service instantiation, as well its parameters is done automatically.

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

type IRepository interface {
	SaveSomething(something Something) error
}

type Service struct {
	repository IRepository
}

func NewService(repository IRepository) IService {
	return Service{
		repository,
	}
}

func (s Service) DoSomething(something Something) error {
	fmt.Printf("doing something %s", something.Title)
	return s.repository.SaveSomething(something)
}

type Repository struct{}

func NewRepository() IRepository {
	return Repository{}
}

func (r Repository) SaveSomething(something Something) error {
	fmt.Printf("saving something %s", something.Title)
	return nil
}

func main() {
	// First create a Container.
	c := gosyringe.NewContainer()

  // You can register the services in any order.
	gosyringe.RegisterSingleton[IService](c, NewService)
	gosyringe.RegisterSingleton[IRepository](c, NewRepository)

	// Resolving the service automativally instantiate its dependencies,
	// a IRepository in this example.
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

## Registration

To register a dependency you call one of the `Register*` functions.

For `Register*` functions that take a constructor argument, you can provide any function that return a single value or a (value, error) tuple.

Constructors can have any number of parameters, you just need to register the parameter types as well. You can register the dependencies in any order.

### RegisterTransient

When a dependency is registered with a Transient lifetime, each call to `Resolve` will create a new instance.

```go
func main() {
	c := gosyringe.NewContainer()

	gosyringe.RegisterTransient[IService](c, NewService)

	instance1, _ := gosyringe.Resolve[IService](c)
	instance2, _ := gosyringe.Resolve[IService](c)

	fmt.Println(instance1 == instance2) // false
}
```

### RegisterSingleton

When a depdendency is registered with a Singleton lifetime, every call to `Resolve` will return the same instance.

`RegisterSingleton` can only be called on a root `Container`.

```go
func main() {
	c := gosyringe.NewContainer()

	gosyringe.RegisterSingleton[IService](c, NewService)

	instance1, _ := gosyringe.Resolve[IService](c)
	instance2, _ := gosyringe.Resolve[IService](c)

	fmt.Println(instance1 == instance2) // true
}
```

### RegisterScoped

When a dependency is registered with a Scoped lifetime, each call to `Resolve` will return the same instance for the same `Container`, but different instances for different `Containers`.

```go
func main() {
	c := gosyringe.NewContainer()

	gosyringe.RegisterScoped[IService](c, NewService)

	c1 := gosyringe.CreateChildContainer(c)
	c2 := gosyringe.CreateChildContainer(c)

	instance1, _ := gosyringe.Resolve[IService](c1)
	instance2, _ := gosyringe.Resolve[IService](c1)
	instance3, _ := gosyringe.Resolve[IService](c2)

	fmt.Println(instance1 == instance2) // true (resolved from same container)
	fmt.Println(instance1 == instance3) // false (resolved from different containers)
}
```

### RegisterValue

When you need to manually create an instance, or you already have a value and want it to be injected, you can use `RegisterValue`.

`RegisterValue` called in a root `Container` will register the dependency as Singleton. `RegisterValue` called in a child `Container` will register the dependency as Scoped.

> [!WARNING]
> Be careful with the type inference of generics.
>
> If you provide an instance with a concrete type, but would like to resolve with the interface type, then you should explicitly provide the type parameter (e.g. `RegisterValue[IService](c, instanceWithConcreteType)`).

```go
func main() {
	c := gosyringe.NewContainer()

	instance := NewService()

	gosyringe.RegisterValue(c, instance)

	resolvedInstance, _ := gosyringe.Resolve[IService](c)

	fmt.Println(instance == resolvedInstance) // true
}
```

## Resolution

After registering your dependencies, you can instantiate them by calling the `Resolve` function.

`Resolve` behaves differently depending on the registration lifetime and the provided type parameter.

### Single instance

When resolving an instance of a single type, the instantiation will occur with the last registerd constructor (or value) for that type.

```go
func main() {
	c := gosyringe.NewContainer()

	instance1 := NewService()
	instance2 := NewService()
	instance3 := NewService()

	gosyringe.RegisterValue(c, instance1)
	gosyringe.RegisterValue(c, instance2)
	gosyringe.RegisterValue(c, instance3)

	resolvedInstance, _ := gosyringe.Resolve[IService](c)

	fmt.Println(instance1 == resolvedInstance) // false
	fmt.Println(instance2 == resolvedInstance) // false
	fmt.Println(instance3 == resolvedInstance) // true
}
```

### Slice of instances

When resolving a slice instance, the instantiation will occur for every regsitered constructor (or value) for that type in order.

```go
func main() {
	c := gosyringe.NewContainer()

	instance1 := NewService()
	instance2 := NewService()
	instance3 := NewService()

	gosyringe.RegisterValue(c, instance1)
	gosyringe.RegisterValue(c, instance2)
	gosyringe.RegisterValue(c, instance3)

	resolvedInstances, _ := gosyringe.Resolve[[]IService](c)

	fmt.Println(instance1 == resolvedInstances[0]) // true
	fmt.Println(instance2 == resolvedInstances[1]) // true
	fmt.Println(instance3 == resolvedInstances[2]) // true
}
```

### Injection lifetime rules

Transient and Singleton dependencies can be injected in a dependency of any other lifetime.

Scoped dependencies can be injected in Transient and other Scoped dependencies, but cannot be injected in a Singleton dependency.

```go
package main

import (
	"fmt"

	"github.com/victormf2/gosyringe"
)

type Root struct{}
type InjectedInRoot struct{}
type InjectedInChild struct{}

func NewRoot(dependency InjectedInRoot) Root {
	return Root{}
}
func NewInjectedInRoot(dependency InjectedInChild) InjectedInRoot {
	return InjectedInRoot{}
}
func NewInjectedInChild() InjectedInChild {
	return InjectedInChild{}
}

func main() {
	// This first example demonstrates an error caused by indirectly
	// injecting a Scoped dependency in a Singleton through a Transient.
	//
	// Scoped dependencies cannot be injected in Singletons, even indirectly.
	c1 := gosyringe.NewContainer()

	gosyringe.RegisterScoped[InjectedInChild](c1, NewInjectedInChild)
	gosyringe.RegisterTransient[InjectedInRoot](c1, NewInjectedInRoot)
	gosyringe.RegisterSingleton[Root](c1, NewRoot)

	c1Child := gosyringe.CreateChildContainer(c1)

	_, err := gosyringe.Resolve[Root](c1Child)
	fmt.Println(err != nil) // true

	// This second example demonstrates an error caused by directly
	// injecting a Scoped dependency in a Singleton.
	c2 := gosyringe.NewContainer()

	gosyringe.RegisterScoped[InjectedInChild](c2, NewInjectedInChild)
	gosyringe.RegisterSingleton[InjectedInRoot](c2, NewInjectedInRoot)

	c2Child := gosyringe.CreateChildContainer(c2)

	_, err = gosyringe.Resolve[InjectedInRoot](c2Child)
	fmt.Println(err != nil) // true
}
```

### Container injection

The most common way to use gosyringe is to leverage the automatic dependency injection using constructor parameters. But sometimes you need to instantiate your dependencies outside the constructor, during application logic execution.

For this, you can inject a `*Container` pointer in your constructor, and then use it to manually call `Resolve` in another moment. The injected `*Container` will be the same that resolved the dependency.

```go
package main

import (
	"fmt"
	"time"

	"github.com/victormf2/gosyringe"
)

type IService interface {
	GetValue() (string, error)
}
type Service struct {
	container *gosyringe.Container
}

func (s *Service) GetValue() (string, error) {
	clock, err := gosyringe.Resolve[IClock](s.container)
	if err != nil {
		return "", err
	}

	return clock.Now().Format("02/01/2006"), nil
}

// In this example, c is the child container that was used to Resolve IService
func NewService(c *gosyringe.Container) IService {
	return &Service{
		container: c,
	}
}

type IClock interface {
	Now() time.Time
}
type Clock struct{}

func (c *Clock) Now() time.Time {
	return time.Now()
}
func NewClock() IClock {
	return &Clock{}
}

func main() {
	c := gosyringe.NewContainer()

	gosyringe.RegisterScoped[IService](c, NewService)
	gosyringe.RegisterSingleton[IClock](c, NewClock)

	service, _ := gosyringe.Resolve[IService](gosyringe.CreateChildContainer(c))

	value, _ := service.GetValue()

	fmt.Println(value) // 11/04/2025
}

```

### Resolution keys

All registration methods have an alternative `RegisterXxxWithKey` version. Registration with keys cannot be directly injected in constructor parameters, but can be resolved with a injected `Container`. They are useful if you want to implement the strategy pattern.

```go
package main

import (
	"fmt"

	"github.com/victormf2/gosyringe"
)

type IPayment interface {
	Pay(value int)
}

type CreditCardPayment struct{}

func (p *CreditCardPayment) Pay(value int) {
	fmt.Printf("paying with credit card: %v\n", value)
}
func NewCreditCardPayment() IPayment {
	return &CreditCardPayment{}
}

type CashPayment struct{}

func (p *CashPayment) Pay(value int) {
	fmt.Printf("paying with cash: %v\n", value)
}
func NewCashPayment() IPayment {
	return &CashPayment{}
}

type Service struct {
	container *gosyringe.Container
}

func (s *Service) MakePayment(paymentMethod string, value int) {
	paymentService, _ := gosyringe.ResolveWithKey[IPayment](s.container, paymentMethod)
	paymentService.Pay(value)
}
func NewService(container *gosyringe.Container) *Service {
	return &Service{
		container: container,
	}
}

func main() {
	c := gosyringe.NewContainer()

	gosyringe.RegisterSingleton[*Service](c, NewService)
	gosyringe.RegisterSingletonWithKey[IPayment](c, "credit_card", NewCreditCardPayment)
	gosyringe.RegisterSingletonWithKey[IPayment](c, "cash", NewCashPayment)

	service, _ := gosyringe.Resolve[*Service](c)

	service.MakePayment("credit_card", 21) // paying with credit card: 21
	service.MakePayment("cash", 200_000_000) // paying with cash: 200000000
}

```

### Injection tokens

Injection tokens is a way to register different constructors for the same type or interface. In other dependency injection frameworks it's usually done by providing a value (e.g. a string like "MyServiceToken") which maps to a constructor. Different values can provide the same instance type.

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
// So when you call Resolve[IOneService](c) you get a different instance
// from Resolve[IAnotherService](c)
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

## Dispose

gosyringe provides a way to run cleanup code for registered dependencies.

When calling `gosyringe.Dispose(c)` all registered dispose actions with `gosyringe.OnDispose` will be called for resolved instances.

Singleton dependencies can be only disposed with root container. All other instances can be only disposed with the container they were resolved from.

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/victormf2/gosyringe"
)

type Service struct{}

func NewService() *Service {
	return &Service{}
}

func (s *Service) DoSomething() {
	fmt.Println("doing stuff")
}

func (s *Service) Close() {
	fmt.Println("releasing allocated resources")
}

func main() {
	ctx := context.Background()
	c := gosyringe.NewContainer()

	gosyringe.RegisterSingleton[*Service](c, NewService)
	gosyringe.OnDispose(c, func(ctx context.Context, service *Service) {
		service.Close()
	})

	service, _ := gosyringe.Resolve[*Service](c)

	service.DoSomething() // doing stuff

	// it's usually good to give a timeout for Dispose,
	// specially when it's done on application SIGTERM or SIGINT
	disposeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	gosyringe.Dispose(disposeCtx, c) // releasing allocated resources
}
```