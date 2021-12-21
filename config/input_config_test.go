package config

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestUnitInputConfig(t *testing.T) {
	suite.Run(t, new(TestInputConfigSuite))
}

type TestInputConfigSuite struct {
	suite.Suite
}

func (s *TestInputConfigSuite) TestEngineType() {
	tests := []struct {
		name    string
		eng     EngineType
		value   string
		want    string
		wantErr bool
	}{
		{
			name: "libpcap",
			eng:  EnginePcap,
			value: func() string {
				eng := EnginePcap
				return eng.String()
			}(),
			want: "libpcap",
		},
		{
			name: "pcap_file",
			eng:  EnginePcapFile,
			value: func() string {
				eng := EnginePcapFile
				return eng.String()
			}(),
			want: "pcap_file",
		},
		{
			name: "raw_socket",
			eng:  EngineRawSocket,
			value: func() string {
				eng := EngineRawSocket
				return eng.String()
			}(),
			want: "raw_socket",
		},
		{
			name:    "invalid engine",
			eng:     EngineType(10),
			value:   "invalid",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			err := tt.eng.Set(tt.value)
			s.Equal(tt.wantErr, err != nil)

			str := tt.eng.String()
			s.Equal(tt.want, str)
		})
	}
}
