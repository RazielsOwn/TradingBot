// Code generated by mockery 2.10.4. DO NOT EDIT.

package mocks

import mock "github.com/stretchr/testify/mock"

// ILogger is an autogenerated mock type for the ILogger type
type ILogger struct {
	mock.Mock
}

// Debug provides a mock function with given fields: message, args
func (_m *ILogger) Debug(message interface{}, args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, message)
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// Error provides a mock function with given fields: message, args
func (_m *ILogger) Error(message interface{}, args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, message)
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// Fatal provides a mock function with given fields: message, args
func (_m *ILogger) Fatal(message interface{}, args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, message)
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// Info provides a mock function with given fields: message, args
func (_m *ILogger) Info(message string, args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, message)
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}

// Warn provides a mock function with given fields: message, args
func (_m *ILogger) Warn(message string, args ...interface{}) {
	var _ca []interface{}
	_ca = append(_ca, message)
	_ca = append(_ca, args...)
	_m.Called(_ca...)
}