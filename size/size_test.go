package size

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

// TestUnitSize size test execute
func TestUnitSize(t *testing.T) {
	suite.Run(t, new(testUnitSizeSuite))
}

// testUnitSizeSuite size test suite
type testUnitSizeSuite struct {
	suite.Suite
}

// TestSet test set method
func (t *testUnitSizeSuite) TestSet() {
	tests := getSetTestList()

	for _, tt := range tests {
		t.Run(tt.name, func() {
			var buf Size
			err := buf.Set(tt.req)

			t.T().Logf("buf str: %s\n", buf.String())
			if err != nil || buf != Size(tt.want) {
				t.T().Errorf("Error parsing %s: %v", tt.req, err)
			}
		})
	}
}

// testSetCase set test struct
type testSetCase struct {
	name    string
	req     string
	want    int
	wantErr bool
}

func getSetTestList() []testSetCase {
	var d = map[string]int{
		"":                     0,
		"42mb":                 42 << 20,
		"4_2":                  42,
		"00":                   0,
		"0":                    0,
		"0_600tb":              384 << 40,
		"0600Tb":               384 << 40,
		"0o12Mb":               10 << 20,
		"0b_10010001111_1kb":   2335 << 10,
		"1024":                 1 << 10,
		"0b111":                7,
		"0x12gB":               18 << 30,
		"0x_67_7a_2f_cc_40_c6": 113774485586118,
		"121562380192901":      121562380192901,
	}

	arr := make([]testSetCase, len(d))

	index := 0
	for key, value := range d {
		arr[index] = testSetCase{
			name:    fmt.Sprintf("test%d", index+1),
			req:     key,
			want:    value,
			wantErr: false,
		}
		index++
	}

	return arr
}
