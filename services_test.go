package gosyringe

import "github.com/google/uuid"

type IService interface {
	GetValue() int
}
type Service struct {
	value string
}

func (s Service) GetValue() int {
	return 12
}
func NewService() IService {
	return &Service{
		value: uuid.NewString(),
	}
}
func NewServiceUnsafe() (IService, error) {
	return &Service{value: uuid.NewString()}, nil
}

type CustomError struct{}

func (c CustomError) Error() string {
	return "custom error"
}

var customError = &CustomError{}

func NewServiceError() (IService, error) {
	return nil, customError
}

type OtherService struct{}

func (s OtherService) GetValue() int {
	return 13
}
func NewOtherService() IService {
	return &OtherService{}
}

type IServiceOne interface {
	GetValueOne() int
}
type ServiceOne struct{}

func NewServiceOne() IServiceOne {
	return &ServiceOne{}
}

func (s ServiceOne) GetValueOne() int {
	return 1
}

type IServiceTwo interface {
	GetValueTwo() int
}
type ServiceTwo struct{}

func NewServiceTwo() (IServiceTwo, error) {
	return &ServiceTwo{}, nil
}

func (s ServiceTwo) GetValueTwo() int {
	return 2
}

type IServiceThree interface {
	GetValueThree() int
}
type ServiceThree struct {
	serviceOne IServiceOne
	serviceTwo IServiceTwo
}

func NewServiceThree(serviceOne IServiceOne, serviceTwo IServiceTwo) IServiceThree {
	return &ServiceThree{
		serviceOne,
		serviceTwo,
	}
}

func (s ServiceThree) GetValueThree() int {
	return s.serviceOne.GetValueOne() + s.serviceTwo.GetValueTwo()
}

type IServiceFive interface {
	GetValueFive() int
}
type ServiceFive struct {
	serviceTwo   IServiceTwo
	serviceThree IServiceThree
}

func NewServiceFive(serviceOne IServiceTwo, serviceThree IServiceThree) IServiceFive {
	return &ServiceFive{
		serviceOne,
		serviceThree,
	}
}

func (s ServiceFive) GetValueFive() int {
	return s.serviceTwo.GetValueTwo() + s.serviceThree.GetValueThree()
}

type MultiServiceInjection struct {
	services []IService
}

func NewMultiServiceInjection(services []IService) MultiServiceInjection {
	return MultiServiceInjection{
		services,
	}
}
func (m MultiServiceInjection) GetMultiValue() []int {
	values := []int{}
	for _, service := range m.services {
		values = append(values, service.GetValue())
	}
	return values
}

type SeeminglyHarmlessService struct {
	circularDependency CircularDependency
}

func NewSeeminglyHarmlessService(circularDependency CircularDependency) IService {
	return &SeeminglyHarmlessService{
		circularDependency,
	}
}
func (s SeeminglyHarmlessService) GetValue() int {
	return 13
}

type CircularDependency struct {
	service IService
}

func NewCircularDependency(service IService) CircularDependency {
	return CircularDependency{
		service,
	}
}
