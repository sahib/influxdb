// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/influxdata/influxdb/v2/replications (interfaces: DurableQueueManager)

// Package mock is a generated GoMock package.
package mock

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	platform "github.com/influxdata/influxdb/v2/kit/platform"
)

// MockDurableQueueManager is a mock of DurableQueueManager interface.
type MockDurableQueueManager struct {
	ctrl     *gomock.Controller
	recorder *MockDurableQueueManagerMockRecorder
}

// MockDurableQueueManagerMockRecorder is the mock recorder for MockDurableQueueManager.
type MockDurableQueueManagerMockRecorder struct {
	mock *MockDurableQueueManager
}

// NewMockDurableQueueManager creates a new mock instance.
func NewMockDurableQueueManager(ctrl *gomock.Controller) *MockDurableQueueManager {
	mock := &MockDurableQueueManager{ctrl: ctrl}
	mock.recorder = &MockDurableQueueManagerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDurableQueueManager) EXPECT() *MockDurableQueueManagerMockRecorder {
	return m.recorder
}

// CloseAll mocks base method.
func (m *MockDurableQueueManager) CloseAll() error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CloseAll")
	ret0, _ := ret[0].(error)
	return ret0
}

// CloseAll indicates an expected call of CloseAll.
func (mr *MockDurableQueueManagerMockRecorder) CloseAll() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CloseAll", reflect.TypeOf((*MockDurableQueueManager)(nil).CloseAll))
}

// CurrentQueueSizes mocks base method.
func (m *MockDurableQueueManager) CurrentQueueSizes(arg0 []platform.ID) (map[platform.ID]int64, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CurrentQueueSizes", arg0)
	ret0, _ := ret[0].(map[platform.ID]int64)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CurrentQueueSizes indicates an expected call of CurrentQueueSizes.
func (mr *MockDurableQueueManagerMockRecorder) CurrentQueueSizes(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CurrentQueueSizes", reflect.TypeOf((*MockDurableQueueManager)(nil).CurrentQueueSizes), arg0)
}

// DeleteQueue mocks base method.
func (m *MockDurableQueueManager) DeleteQueue(arg0 platform.ID) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteQueue", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteQueue indicates an expected call of DeleteQueue.
func (mr *MockDurableQueueManagerMockRecorder) DeleteQueue(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteQueue", reflect.TypeOf((*MockDurableQueueManager)(nil).DeleteQueue), arg0)
}

// EnqueueData mocks base method.
func (m *MockDurableQueueManager) EnqueueData(arg0 platform.ID, arg1 []byte, arg2 int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "EnqueueData", arg0, arg1, arg2)
	ret0, _ := ret[0].(error)
	return ret0
}

// EnqueueData indicates an expected call of EnqueueData.
func (mr *MockDurableQueueManagerMockRecorder) EnqueueData(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "EnqueueData", reflect.TypeOf((*MockDurableQueueManager)(nil).EnqueueData), arg0, arg1, arg2)
}

// InitializeQueue mocks base method.
func (m *MockDurableQueueManager) InitializeQueue(arg0 platform.ID, arg1 int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "InitializeQueue", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// InitializeQueue indicates an expected call of InitializeQueue.
func (mr *MockDurableQueueManagerMockRecorder) InitializeQueue(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "InitializeQueue", reflect.TypeOf((*MockDurableQueueManager)(nil).InitializeQueue), arg0, arg1)
}

// StartReplicationQueues mocks base method.
func (m *MockDurableQueueManager) StartReplicationQueues(arg0 map[platform.ID]int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "StartReplicationQueues", arg0)
	ret0, _ := ret[0].(error)
	return ret0
}

// StartReplicationQueues indicates an expected call of StartReplicationQueues.
func (mr *MockDurableQueueManagerMockRecorder) StartReplicationQueues(arg0 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "StartReplicationQueues", reflect.TypeOf((*MockDurableQueueManager)(nil).StartReplicationQueues), arg0)
}

// UpdateMaxQueueSize mocks base method.
func (m *MockDurableQueueManager) UpdateMaxQueueSize(arg0 platform.ID, arg1 int64) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateMaxQueueSize", arg0, arg1)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateMaxQueueSize indicates an expected call of UpdateMaxQueueSize.
func (mr *MockDurableQueueManagerMockRecorder) UpdateMaxQueueSize(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateMaxQueueSize", reflect.TypeOf((*MockDurableQueueManager)(nil).UpdateMaxQueueSize), arg0, arg1)
}
