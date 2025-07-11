package gosyringe

import "testing"

func BenchmarkNormalInstantiation(b *testing.B) {
	for b.Loop() {
		config := &Config{
			SomeUrl: "123",
		}
		services := [4]*HandlerService{}
		for i := range 4 {
			subServices := [4]*SubHandlerService{}
			for j := range 4 {
				subServices[j] = NewSubHandlerService(config)
			}
			services[i] = NewHandlerService(subServices[0], subServices[1], subServices[2], subServices[3])
		}
		NewHandler(services[0], services[1], services[2], services[3])
	}
}

func BenchmarkGosyringe(b *testing.B) {
	c := NewContainer()

	RegisterValue(c, "123")
	RegisterSingleton[*Config](c, NewConfig)
	RegisterTransient[*Handler](c, NewHandler)
	RegisterTransient[*HandlerService](c, NewHandlerService)
	RegisterTransient[*SubHandlerService](c, NewSubHandlerService)

	for b.Loop() {
		Resolve[*Handler](c)
	}
}

type Handler struct {
	service0 *HandlerService
	service1 *HandlerService
	service2 *HandlerService
	service3 *HandlerService
}

func NewHandler(service0 *HandlerService, service1 *HandlerService, service2 *HandlerService, service3 *HandlerService) *Handler {
	return &Handler{
		service0: service0,
		service1: service1,
		service2: service2,
		service3: service3,
	}
}

type HandlerService struct {
	subService0 *SubHandlerService
	subService1 *SubHandlerService
	subService2 *SubHandlerService
	subService3 *SubHandlerService
}

func NewHandlerService(subService0 *SubHandlerService, subService1 *SubHandlerService, subService2 *SubHandlerService, subService3 *SubHandlerService) *HandlerService {
	return &HandlerService{
		subService0: subService0,
		subService1: subService1,
		subService2: subService2,
		subService3: subService3,
	}
}

type SubHandlerService struct {
	config *Config
}

func NewSubHandlerService(config *Config) *SubHandlerService {
	return &SubHandlerService{
		config: config,
	}
}

type Config struct {
	SomeUrl string
}

func NewConfig(url string) *Config {
	return &Config{
		SomeUrl: url,
	}
}
