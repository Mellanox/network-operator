// Code generated by mockery v2.43.2. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"

	staticconfig "github.com/Mellanox/network-operator/pkg/staticconfig"
)

// Provider is an autogenerated mock type for the Provider type
type Provider struct {
	mock.Mock
}

type Provider_Expecter struct {
	mock *mock.Mock
}

func (_m *Provider) EXPECT() *Provider_Expecter {
	return &Provider_Expecter{mock: &_m.Mock}
}

// GetStaticConfig provides a mock function with given fields:
func (_m *Provider) GetStaticConfig() staticconfig.StaticConfig {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetStaticConfig")
	}

	var r0 staticconfig.StaticConfig
	if rf, ok := ret.Get(0).(func() staticconfig.StaticConfig); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(staticconfig.StaticConfig)
	}

	return r0
}

// Provider_GetStaticConfig_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetStaticConfig'
type Provider_GetStaticConfig_Call struct {
	*mock.Call
}

// GetStaticConfig is a helper method to define mock.On call
func (_e *Provider_Expecter) GetStaticConfig() *Provider_GetStaticConfig_Call {
	return &Provider_GetStaticConfig_Call{Call: _e.mock.On("GetStaticConfig")}
}

func (_c *Provider_GetStaticConfig_Call) Run(run func()) *Provider_GetStaticConfig_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *Provider_GetStaticConfig_Call) Return(_a0 staticconfig.StaticConfig) *Provider_GetStaticConfig_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *Provider_GetStaticConfig_Call) RunAndReturn(run func() staticconfig.StaticConfig) *Provider_GetStaticConfig_Call {
	_c.Call.Return(run)
	return _c
}

// NewProvider creates a new instance of Provider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewProvider(t interface {
	mock.TestingT
	Cleanup(func())
}) *Provider {
	mock := &Provider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
