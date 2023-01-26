// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/openservicemesh/osm/pkg/debugger (interfaces: CertificateManagerDebugger,MeshCatalogDebugger)

// Package debugger is a generated GoMock package.
package debugger

import (
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	certificate "github.com/openservicemesh/osm/pkg/certificate"
	identity "github.com/openservicemesh/osm/pkg/identity"
	v1alpha3 "github.com/servicemeshinterface/smi-sdk-go/pkg/apis/access/v1alpha3"
	v1alpha4 "github.com/servicemeshinterface/smi-sdk-go/pkg/apis/specs/v1alpha4"
	v1alpha40 "github.com/servicemeshinterface/smi-sdk-go/pkg/apis/split/v1alpha4"
)

// MockCertificateManagerDebugger is a mock of CertificateManagerDebugger interface.
type MockCertificateManagerDebugger struct {
	ctrl     *gomock.Controller
	recorder *MockCertificateManagerDebuggerMockRecorder
}

// MockCertificateManagerDebuggerMockRecorder is the mock recorder for MockCertificateManagerDebugger.
type MockCertificateManagerDebuggerMockRecorder struct {
	mock *MockCertificateManagerDebugger
}

// NewMockCertificateManagerDebugger creates a new mock instance.
func NewMockCertificateManagerDebugger(ctrl *gomock.Controller) *MockCertificateManagerDebugger {
	mock := &MockCertificateManagerDebugger{ctrl: ctrl}
	mock.recorder = &MockCertificateManagerDebuggerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockCertificateManagerDebugger) EXPECT() *MockCertificateManagerDebuggerMockRecorder {
	return m.recorder
}

// ListIssuedCertificates mocks base method.
func (m *MockCertificateManagerDebugger) ListIssuedCertificates() []*certificate.Certificate {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListIssuedCertificates")
	ret0, _ := ret[0].([]*certificate.Certificate)
	return ret0
}

// ListIssuedCertificates indicates an expected call of ListIssuedCertificates.
func (mr *MockCertificateManagerDebuggerMockRecorder) ListIssuedCertificates() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListIssuedCertificates", reflect.TypeOf((*MockCertificateManagerDebugger)(nil).ListIssuedCertificates))
}

// MockMeshCatalogDebugger is a mock of MeshCatalogDebugger interface.
type MockMeshCatalogDebugger struct {
	ctrl     *gomock.Controller
	recorder *MockMeshCatalogDebuggerMockRecorder
}

// MockMeshCatalogDebuggerMockRecorder is the mock recorder for MockMeshCatalogDebugger.
type MockMeshCatalogDebuggerMockRecorder struct {
	mock *MockMeshCatalogDebugger
}

// NewMockMeshCatalogDebugger creates a new mock instance.
func NewMockMeshCatalogDebugger(ctrl *gomock.Controller) *MockMeshCatalogDebugger {
	mock := &MockMeshCatalogDebugger{ctrl: ctrl}
	mock.recorder = &MockMeshCatalogDebuggerMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockMeshCatalogDebugger) EXPECT() *MockMeshCatalogDebuggerMockRecorder {
	return m.recorder
}

// ListSMIPolicies mocks base method.
func (m *MockMeshCatalogDebugger) ListSMIPolicies() ([]*v1alpha40.TrafficSplit, []identity.K8sServiceAccount, []*v1alpha4.HTTPRouteGroup, []*v1alpha3.TrafficTarget) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListSMIPolicies")
	ret0, _ := ret[0].([]*v1alpha40.TrafficSplit)
	ret1, _ := ret[1].([]identity.K8sServiceAccount)
	ret2, _ := ret[2].([]*v1alpha4.HTTPRouteGroup)
	ret3, _ := ret[3].([]*v1alpha3.TrafficTarget)
	return ret0, ret1, ret2, ret3
}

// ListSMIPolicies indicates an expected call of ListSMIPolicies.
func (mr *MockMeshCatalogDebuggerMockRecorder) ListSMIPolicies() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListSMIPolicies", reflect.TypeOf((*MockMeshCatalogDebugger)(nil).ListSMIPolicies))
}
