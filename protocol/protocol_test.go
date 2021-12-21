package protocol

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/suite"
)

// TestUnitProtocol protocol test execute
func TestUnitProtocol(t *testing.T) {
	suite.Run(t, new(testUnitProtocolSuite))
}

// testUnitProtocolSuite protocol test suite
type testUnitProtocolSuite struct {
	suite.Suite
}

// TestPayloadHeader test PayloadHeader method
func (t *testUnitProtocolSuite) TestPayloadHeader() {
	tests := []struct {
		name        string
		payloadType byte
		uuid        []byte
		timing      int64
		latency     int64
		want        string
	}{
		{
			name:        "test1",
			payloadType: 3,
			uuid:        []byte("f45590522cd1838b4a0d5c5aab80b77929dea3b3"),
			timing:      13923489726487326,
			latency:     1231,
			want:        "\u0003 f45590522cd1838b4a0d5c5aab80b77929dea3b3 13923489726487326 1231\n",
		},
		{
			name:        "test2",
			payloadType: 3,
			uuid:        []byte("s\ns"),
			timing:      13923489726487326,
			latency:     1231,
			want:        "\u0003 s\ns 13923489726487326 1231\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			got := PayloadHeader(tt.payloadType, tt.uuid, tt.timing, tt.latency)

			if !reflect.DeepEqual(string(got), tt.want) {
				t.T().Errorf("PayloadMeta() got = %v, want %v", string(got), tt.want)
			}
		})
	}
}

// TestPayloadMeta test UUID method
func (t *testUnitProtocolSuite) TestUUID() {
	tests := []struct {
		name    string
		wantLen int
	}{
		{
			name:    "test1",
			wantLen: 24,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			got := UUID()

			if !reflect.DeepEqual(len(got), tt.wantLen) {
				t.T().Errorf("UUID() got = %v, want %v", len(got), tt.wantLen)
			}
		})
	}
}

// TestPayloadMetaWithBody test PayloadMetaWithBody method
func (t *testUnitProtocolSuite) TestPayloadMetaWithBody() {
	tests := []struct {
		name string
		req  []byte
		want []byte
	}{
		{
			name: "test1",
			req:  []byte("test1"),
			want: []byte("test1"),
		},
		{
			name: "test2",
			req:  []byte("test2\ntest2"),
			want: []byte("test2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			_, got := PayloadMetaWithBody(tt.req)

			if !reflect.DeepEqual(got, tt.want) {
				t.T().Errorf("PayloadMetaWithBody() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsOriginPayload test IsOriginPayload method
func (t *testUnitProtocolSuite) TestIsOriginPayload() {
	tests := []struct {
		name string
		req  []byte
		want bool
	}{
		{
			name: "test1",
			req:  []byte("1test1"),
			want: true,
		},
		{
			name: "test2",
			req:  []byte("2test2"),
			want: true,
		},
		{
			name: "test3",
			req:  []byte("8test2"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			got := IsOriginPayload(tt.req)

			if !reflect.DeepEqual(got, tt.want) {
				t.T().Errorf("IsOriginPayload() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestIsRequestPayload test IsRequestPayload method
func (t *testUnitProtocolSuite) TestIsRequestPayload() {
	tests := []struct {
		name string
		req  []byte
		want bool
	}{
		{
			name: "test1",
			req:  []byte("1test1"),
			want: true,
		},
		{
			name: "test2",
			req:  []byte("2test2"),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			got := IsRequestPayload(tt.req)

			if !reflect.DeepEqual(got, tt.want) {
				t.T().Errorf("IsRequestPayload() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPayloadMeta test PayloadMeta method
func (t *testUnitProtocolSuite) TestPayloadMeta() {
	tests := []struct {
		name string
		req  []byte
		want []byte
	}{
		{
			name: "test1",
			req:  []byte("test1"),
			want: []byte(nil),
		},
		{
			name: "test2",
			req:  []byte("te st2\ntest2"),
			want: []byte("st2"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func() {
			got := PayloadID(tt.req)

			if !reflect.DeepEqual(got, tt.want) {
				t.T().Errorf("PayloadID() got = %v, want %v", got, tt.want)
			}
		})
	}
}
