package events

import (
	"testing"

	"github.com/stretchr/testify/mock"
	api "github.com/datastax/cass-operator/operator/pkg/apis/cassandra/v1beta1"
)

type MockInfoLogger struct {
	mock.Mock
}


func (l *MockInfoLogger) Info(msg string, keysAndValues ...interface{}) {
	_ = l.Called(msg, append([]interface{}{msg}, keysAndValues...))
}

func (l *MockInfoLogger) Enabled() bool {
	args := l.Called()
  	return args.Bool(0)
}

func TestLoggingEventRecorder(t *testing.T) {
	logger := &MockInfoLogger{}
	recorder := &LoggingEventRecorder{ReqLogger: logger}
	dc := &api.CassandraDatacenter{}

	logger.On("Info", 
		"Some event message", 
		mock.AnythingOfType("string"), 
		mock.AnythingOfType("string"), 
		mock.AnythingOfType("string"), 
		mock.AnythingOfType("string"))
	
	recorder.Eventf(dc, "SomeType", "SomeReason", "Some %s message", "event")

	logger.AssertExpectations(t)
}
