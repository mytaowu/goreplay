package byteutils

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/suite"
)

// TestUnitByteUtils test execute
func TestUnitByteUtils(t *testing.T) {
	suite.Run(t, new(byteUtilsSuite))
}

// byteUtilsSuite test suite
type byteUtilsSuite struct {
	suite.Suite
}

// TestCut test cut byte slice
func (s *byteUtilsSuite) TestCut() {
	tests := []struct {
		name   string
		source []byte
		form   int
		to     int
		want   []byte
	}{
		{
			name:   "success",
			source: []byte("123456"),
			form:   2,
			to:     4,
			want:   []byte("1256"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if !bytes.Equal(Cut(tt.source, tt.form, tt.to), tt.want) {
				s.T().Error("should properly cut")
			}
		})
	}

}

// TestInsert test byte slice insert element
func (s *byteUtilsSuite) TestInsert() {
	tests := []struct {
		name   string
		source []byte
		dest   []byte
		index  int
		want   []byte
	}{
		{
			name:   "success",
			source: []byte("123456"),
			dest:   []byte("abcd"),
			index:  2,
			want:   []byte("12abcd3456"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if !bytes.Equal(Insert(tt.source, tt.index, tt.dest), tt.want) {
				s.T().Error("should insert into middle of slice")
			}
		})
	}

}

// TestReplace test byte slice replace element
func (s *byteUtilsSuite) TestReplace() {
	tests := []struct {
		name   string
		source []byte
		dest   []byte
		from   int
		to     int
		want   []byte
	}{
		{
			name:   "success",
			source: []byte("123456"),
			dest:   []byte("ab"),
			from:   2,
			to:     4,
			want:   []byte("12ab56"),
		},
		{
			name:   "success",
			source: []byte("123456"),
			dest:   []byte("abcd"),
			from:   2,
			to:     4,
			want:   []byte("12abcd56"),
		},
		{
			name:   "success",
			source: []byte("123456"),
			dest:   []byte("ab"),
			from:   2,
			to:     5,
			want:   []byte("12ab6"),
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			if !bytes.Equal(Replace(tt.source, tt.from, tt.to, tt.dest), tt.want) {
				s.T().Error("replace when replacement length bigger")
			}
		})
	}
}

// TestStringToSlice test byte slice to string
func (s *byteUtilsSuite) TestSliceToString() {
	s.Run("to string", func() {
		buf := []byte("123456")
		str := SliceToString(buf[:])
		s.T().Logf("str: %s", str)
	})
}

// BenchmarkSliceToString byte slice to string benchmark test
func BenchmarkSliceToString(b *testing.B) {
	var s string
	var buf [1 << 20]byte
	for i := 0; i < b.N; i++ {
		s = SliceToString(buf[:])
	}
	_ = s // avoid gc to optimize away the loop body
}
