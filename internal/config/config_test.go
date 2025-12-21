package config

import (
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseConfig(t *testing.T) {
	type want struct {
		serverAddress string
		baseURL       string
		shouldError   bool
	}

	tests := []struct {
		name    string
		envVars map[string]string
		flags   []string
		want    want
	}{
		{
			name:    "default values",
			envVars: map[string]string{},
			flags:   []string{},
			want: want{
				serverAddress: "localhost:8080",
				baseURL:       "http://localhost:8080",
				shouldError:   false,
			},
		},
		{
			name: "environment variables only",
			envVars: map[string]string{
				"SERVER_ADDRESS": "localhost:8888",
				"BASE_URL":       "http://example.com",
			},
			flags: []string{},
			want: want{
				serverAddress: "localhost:8888",
				baseURL:       "http://example.com",
				shouldError:   false,
			},
		},
		{
			name:    "flags only",
			envVars: map[string]string{},
			flags:   []string{"-a", "localhost:9999", "-b", "http://myserver.com"},
			want: want{
				serverAddress: "localhost:9999",
				baseURL:       "http://myserver.com",
				shouldError:   false,
			},
		},
		{
			name: "environment variables override flags",
			envVars: map[string]string{
				"SERVER_ADDRESS": "env-server:7777",
				"BASE_URL":       "http://env-url.com",
			},
			flags: []string{"-a", "flag-server:8888", "-b", "http://flag-url.com"},
			want: want{
				serverAddress: "env-server:7777",
				baseURL:       "http://env-url.com",
				shouldError:   false,
			},
		},
		{
			name: "empty values",
			envVars: map[string]string{
				"SERVER_ADDRESS": "",
				"BASE_URL":       "",
			},
			flags: []string{"-a", "", "-b", ""},
			want: want{
				serverAddress: "localhost:8080",
				baseURL:       "http://localhost:8080",
				shouldError:   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

			for key, value := range tt.envVars {
				os.Setenv(key, value)
			}

			os.Args = append([]string{"test"}, tt.flags...)

			cfg, err := ParseFlags()

			if tt.want.shouldError {
				require.Error(t, err, "expected error but got none")
				assert.Contains(t, err.Error(), "cannot be empty")
			} else {
				require.NoError(t, err, "unexpected error")

				assert.Equal(t, tt.want.serverAddress, cfg.ServerAddress,
					"server address mismatch")
				assert.Equal(t, tt.want.baseURL, cfg.BaseURL,
					"base URL mismatch")
			}
		})
	}
}
