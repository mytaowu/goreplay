package plugins

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"goreplay/config"
)

type pluginsSuite struct {
	suite.Suite
}

// TestUnitPlugins plugins unit test execute
func TestUnitPlugins(t *testing.T) {
	suite.Run(t, new(pluginsSuite))
}

func (s pluginsSuite) TestPluginsRegistrationHost() {
	for _, tt := range []struct {
		name     string
		settings Settings
		input    int
	}{
		{
			name:     "success",
			settings: InitPluginSettings(),
			input:    0,
		},
		{
			name: "success",
			settings: Settings{
				OutputHTTP: config.MultiOption{"www.example.com|10"},
				InputFile:  config.MultiOption{"/dev/null"},
				ModifierConfig: config.HTTPModifierConfig{
					Headers: config.HTTPHeaders{},
				},
			},
			input: 2,
		},
	} {
		s.Run(tt.name, func() {
			_ = tt.settings.ModifierConfig.Headers.Set("Host:value")
			plugins := NewPlugins(tt.settings)
			s.Equal(len(plugins.Inputs), tt.input)
		})
	}

}
