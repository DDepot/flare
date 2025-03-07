// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	prometheus "github.com/prometheus/client_golang/prometheus"
	
	manager "github.com/flare-foundation/flare/database/manager"
)

// Manager is an autogenerated mock type for the Manager type
type Manager struct {
	mock.Mock
}

// Close provides a mock function with given fields:
func (_m *Manager) Close() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Current provides a mock function with given fields:
func (_m *Manager) Current() *manager.VersionedDatabase {
	ret := _m.Called()

	var r0 *manager.VersionedDatabase
	if rf, ok := ret.Get(0).(func() *manager.VersionedDatabase); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*manager.VersionedDatabase)
		}
	}

	return r0
}

// CurrentDBBootstrapped provides a mock function with given fields:
func (_m *Manager) CurrentDBBootstrapped() (bool, error) {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func() error); ok {
		r1 = rf()
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDatabases provides a mock function with given fields:
func (_m *Manager) GetDatabases() []*manager.VersionedDatabase {
	ret := _m.Called()

	var r0 []*manager.VersionedDatabase
	if rf, ok := ret.Get(0).(func() []*manager.VersionedDatabase); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]*manager.VersionedDatabase)
		}
	}

	return r0
}

// MarkCurrentDBBootstrapped provides a mock function with given fields:
func (_m *Manager) MarkCurrentDBBootstrapped() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewCompleteMeterDBManager provides a mock function with given fields: namespace, registerer
func (_m *Manager) NewCompleteMeterDBManager(namespace string, registerer prometheus.Registerer) (manager.Manager, error) {
	ret := _m.Called(namespace, registerer)

	var r0 manager.Manager
	if rf, ok := ret.Get(0).(func(string, prometheus.Registerer) manager.Manager); ok {
		r0 = rf(namespace, registerer)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(manager.Manager)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, prometheus.Registerer) error); ok {
		r1 = rf(namespace, registerer)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewMeterDBManager provides a mock function with given fields: namespace, registerer
func (_m *Manager) NewMeterDBManager(namespace string, registerer prometheus.Registerer) (manager.Manager, error) {
	ret := _m.Called(namespace, registerer)

	var r0 manager.Manager
	if rf, ok := ret.Get(0).(func(string, prometheus.Registerer) manager.Manager); ok {
		r0 = rf(namespace, registerer)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(manager.Manager)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, prometheus.Registerer) error); ok {
		r1 = rf(namespace, registerer)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// NewNestedPrefixDBManager provides a mock function with given fields: prefix
func (_m *Manager) NewNestedPrefixDBManager(prefix []byte) manager.Manager {
	ret := _m.Called(prefix)

	var r0 manager.Manager
	if rf, ok := ret.Get(0).(func([]byte) manager.Manager); ok {
		r0 = rf(prefix)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(manager.Manager)
		}
	}

	return r0
}

// NewPrefixDBManager provides a mock function with given fields: prefix
func (_m *Manager) NewPrefixDBManager(prefix []byte) manager.Manager {
	ret := _m.Called(prefix)

	var r0 manager.Manager
	if rf, ok := ret.Get(0).(func([]byte) manager.Manager); ok {
		r0 = rf(prefix)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(manager.Manager)
		}
	}

	return r0
}

// Previous provides a mock function with given fields:
func (_m *Manager) Previous() (*manager.VersionedDatabase, bool) {
	ret := _m.Called()

	var r0 *manager.VersionedDatabase
	if rf, ok := ret.Get(0).(func() *manager.VersionedDatabase); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*manager.VersionedDatabase)
		}
	}

	var r1 bool
	if rf, ok := ret.Get(1).(func() bool); ok {
		r1 = rf()
	} else {
		r1 = ret.Get(1).(bool)
	}

	return r0, r1
}
