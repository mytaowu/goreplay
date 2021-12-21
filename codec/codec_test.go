package codec

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"
)

// TestUnitCodec codec test execute
func TestUnitCodec(t *testing.T) {
	suite.Run(t, new(TestUnitCodecSuite))
}

// TestUnitCodecSuite codec test suite
type TestUnitCodecSuite struct {
	suite.Suite
}

// SetupTest which will run before each test in the suite.
func (s *TestUnitCodecSuite) SetupTest() {
	RegisterHeaderCodec("test Codec", &emptyHeaderCodecBuilder{})
}

// TestHeaderCodecGetAndRegisterHeaderCodec test the GetAndRegisterHeaderCodec method
func (s *TestUnitCodecSuite) TestHeaderCodecGetAndRegisterHeaderCodec() {
	tests := []struct {
		name               string
		protoName          string
		headerCodec        HeaderCodec
		headerCodecBuilder HeaderCodecBuilder
	}{
		{
			name:               "success",
			protoName:          "test Codec",
			headerCodecBuilder: &emptyHeaderCodecBuilder{},
		},
		{
			name:               "success",
			protoName:          "test Codec2",
			headerCodecBuilder: &emptyHeaderCodecBuilder{},
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			headerCodec := GetHeaderCodec(tt.protoName)
			if !reflect.DeepEqual(headerCodec, tt.headerCodecBuilder.New()) {
				s.T().Errorf("Decode() got = %v, want %v", headerCodec, tt.headerCodecBuilder.New())
			}
		})
	}
}
