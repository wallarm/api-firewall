// Code generated by MockGen. DO NOT EDIT.
// Source: ./internal/platform/proxy/dnscache.go

// Package proxy is a generated GoMock package.
package proxy

import (
	context "context"
	net "net"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
)

// MockDNSCache is a mock of DNSCache interface.
type MockDNSCache struct {
	ctrl     *gomock.Controller
	recorder *MockDNSCacheMockRecorder
}

// MockDNSCacheMockRecorder is the mock recorder for MockDNSCache.
type MockDNSCacheMockRecorder struct {
	mock *MockDNSCache
}

// NewMockDNSCache creates a new mock instance.
func NewMockDNSCache(ctrl *gomock.Controller) *MockDNSCache {
	mock := &MockDNSCache{ctrl: ctrl}
	mock.recorder = &MockDNSCacheMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockDNSCache) EXPECT() *MockDNSCacheMockRecorder {
	return m.recorder
}

// LookupIPAddr mocks base method.
func (m *MockDNSCache) LookupIPAddr(arg0 context.Context, arg1 string) ([]net.IPAddr, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "LookupIPAddr", arg0, arg1)
	ret0, _ := ret[0].([]net.IPAddr)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// LookupIPAddr indicates an expected call of LookupIPAddr.
func (mr *MockDNSCacheMockRecorder) LookupIPAddr(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "LookupIPAddr", reflect.TypeOf((*MockDNSCache)(nil).LookupIPAddr), arg0, arg1)
}

// Refresh mocks base method.
func (m *MockDNSCache) Refresh() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Refresh")
}

// Refresh indicates an expected call of Refresh.
func (mr *MockDNSCacheMockRecorder) Refresh() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Refresh", reflect.TypeOf((*MockDNSCache)(nil).Refresh))
}

// Stop mocks base method.
func (m *MockDNSCache) Stop() {
	m.ctrl.T.Helper()
	m.ctrl.Call(m, "Stop")
}

// Stop indicates an expected call of Stop.
func (mr *MockDNSCacheMockRecorder) Stop() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Stop", reflect.TypeOf((*MockDNSCache)(nil).Stop))
}
