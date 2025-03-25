// Code generated by mockery v2.50.0. DO NOT EDIT.

package mocks

import (
	context "context"

	executor "github.com/fyerfyer/fyer-scheduler/pkg/worker/executor"
	mock "github.com/stretchr/testify/mock"
)

// IExecutor is an autogenerated mock type for the IExecutor type
type IExecutor struct {
	mock.Mock
}

type IExecutor_Expecter struct {
	mock *mock.Mock
}

func (_m *IExecutor) EXPECT() *IExecutor_Expecter {
	return &IExecutor_Expecter{mock: &_m.Mock}
}

// Execute provides a mock function with given fields: ctx, execCtx
func (_m *IExecutor) Execute(ctx context.Context, execCtx *executor.ExecutionContext) (*executor.ExecutionResult, error) {
	ret := _m.Called(ctx, execCtx)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 *executor.ExecutionResult
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, *executor.ExecutionContext) (*executor.ExecutionResult, error)); ok {
		return rf(ctx, execCtx)
	}
	if rf, ok := ret.Get(0).(func(context.Context, *executor.ExecutionContext) *executor.ExecutionResult); ok {
		r0 = rf(ctx, execCtx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*executor.ExecutionResult)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, *executor.ExecutionContext) error); ok {
		r1 = rf(ctx, execCtx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// IExecutor_Execute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Execute'
type IExecutor_Execute_Call struct {
	*mock.Call
}

// Execute is a helper method to define mock.On call
//   - ctx context.Context
//   - execCtx *executor.ExecutionContext
func (_e *IExecutor_Expecter) Execute(ctx interface{}, execCtx interface{}) *IExecutor_Execute_Call {
	return &IExecutor_Execute_Call{Call: _e.mock.On("Execute", ctx, execCtx)}
}

func (_c *IExecutor_Execute_Call) Run(run func(ctx context.Context, execCtx *executor.ExecutionContext)) *IExecutor_Execute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(*executor.ExecutionContext))
	})
	return _c
}

func (_c *IExecutor_Execute_Call) Return(_a0 *executor.ExecutionResult, _a1 error) *IExecutor_Execute_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *IExecutor_Execute_Call) RunAndReturn(run func(context.Context, *executor.ExecutionContext) (*executor.ExecutionResult, error)) *IExecutor_Execute_Call {
	_c.Call.Return(run)
	return _c
}

// GetExecutionStatus provides a mock function with given fields: executionID
func (_m *IExecutor) GetExecutionStatus(executionID string) (*executor.ExecutionStatus, bool) {
	ret := _m.Called(executionID)

	if len(ret) == 0 {
		panic("no return value specified for GetExecutionStatus")
	}

	var r0 *executor.ExecutionStatus
	var r1 bool
	if rf, ok := ret.Get(0).(func(string) (*executor.ExecutionStatus, bool)); ok {
		return rf(executionID)
	}
	if rf, ok := ret.Get(0).(func(string) *executor.ExecutionStatus); ok {
		r0 = rf(executionID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*executor.ExecutionStatus)
		}
	}

	if rf, ok := ret.Get(1).(func(string) bool); ok {
		r1 = rf(executionID)
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}

// IExecutor_GetExecutionStatus_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetExecutionStatus'
type IExecutor_GetExecutionStatus_Call struct {
	*mock.Call
}

// GetExecutionStatus is a helper method to define mock.On call
//   - executionID string
func (_e *IExecutor_Expecter) GetExecutionStatus(executionID interface{}) *IExecutor_GetExecutionStatus_Call {
	return &IExecutor_GetExecutionStatus_Call{Call: _e.mock.On("GetExecutionStatus", executionID)}
}

func (_c *IExecutor_GetExecutionStatus_Call) Run(run func(executionID string)) *IExecutor_GetExecutionStatus_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *IExecutor_GetExecutionStatus_Call) Return(_a0 *executor.ExecutionStatus, _a1 bool) *IExecutor_GetExecutionStatus_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *IExecutor_GetExecutionStatus_Call) RunAndReturn(run func(string) (*executor.ExecutionStatus, bool)) *IExecutor_GetExecutionStatus_Call {
	_c.Call.Return(run)
	return _c
}

// GetRunningExecutions provides a mock function with no fields
func (_m *IExecutor) GetRunningExecutions() map[string]*executor.ExecutionStatus {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for GetRunningExecutions")
	}

	var r0 map[string]*executor.ExecutionStatus
	if rf, ok := ret.Get(0).(func() map[string]*executor.ExecutionStatus); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(map[string]*executor.ExecutionStatus)
		}
	}

	return r0
}

// IExecutor_GetRunningExecutions_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'GetRunningExecutions'
type IExecutor_GetRunningExecutions_Call struct {
	*mock.Call
}

// GetRunningExecutions is a helper method to define mock.On call
func (_e *IExecutor_Expecter) GetRunningExecutions() *IExecutor_GetRunningExecutions_Call {
	return &IExecutor_GetRunningExecutions_Call{Call: _e.mock.On("GetRunningExecutions")}
}

func (_c *IExecutor_GetRunningExecutions_Call) Run(run func()) *IExecutor_GetRunningExecutions_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *IExecutor_GetRunningExecutions_Call) Return(_a0 map[string]*executor.ExecutionStatus) *IExecutor_GetRunningExecutions_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *IExecutor_GetRunningExecutions_Call) RunAndReturn(run func() map[string]*executor.ExecutionStatus) *IExecutor_GetRunningExecutions_Call {
	_c.Call.Return(run)
	return _c
}

// Kill provides a mock function with given fields: executionID
func (_m *IExecutor) Kill(executionID string) error {
	ret := _m.Called(executionID)

	if len(ret) == 0 {
		panic("no return value specified for Kill")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(executionID)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// IExecutor_Kill_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Kill'
type IExecutor_Kill_Call struct {
	*mock.Call
}

// Kill is a helper method to define mock.On call
//   - executionID string
func (_e *IExecutor_Expecter) Kill(executionID interface{}) *IExecutor_Kill_Call {
	return &IExecutor_Kill_Call{Call: _e.mock.On("Kill", executionID)}
}

func (_c *IExecutor_Kill_Call) Run(run func(executionID string)) *IExecutor_Kill_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(string))
	})
	return _c
}

func (_c *IExecutor_Kill_Call) Return(_a0 error) *IExecutor_Kill_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *IExecutor_Kill_Call) RunAndReturn(run func(string) error) *IExecutor_Kill_Call {
	_c.Call.Return(run)
	return _c
}

// NewIExecutor creates a new instance of IExecutor. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewIExecutor(t interface {
	mock.TestingT
	Cleanup(func())
}) *IExecutor {
	mock := &IExecutor{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
