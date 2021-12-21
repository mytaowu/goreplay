package codec

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"
)

// TestUnitEmptyCodec emptyCodec test execute
func TestUnitEmptyCodec(t *testing.T) {
	suite.Run(t, new(TestUnitEmptyCodecSuite))
}

// TestUnitEmptyCodecSuite emptyCodec test suite
type TestUnitEmptyCodecSuite struct {
	suite.Suite
}

// SetupTest which will run before each test in the suite.
func (t *TestUnitEmptyCodecSuite) SetupTest() {
}

// TestEmptyHeaderCodecDecode test EmptyHeaderCodecDecode method
func (t *TestUnitEmptyCodecSuite) TestEmptyHeaderCodecDecode() {
	tests := []struct {
		name    string
		reqBuf  []byte
		want    interface{}
		wantErr bool
	}{
		{
			name: "success",
			want: ProtocolHeader{
				ServiceName:   unknown,
				APIName:       unknown,
				InterfaceName: unknown,
				MethodName:    unknown,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			g := &emptyHeaderCodec{}
			got, err := g.Decode(tt.reqBuf, "")
			if (err != nil) != tt.wantErr {
				t.T().Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got, tt.want) {
				t.T().Errorf("Decode() got = %v, want %v", got, tt.want)
			}
		})
	}
}
