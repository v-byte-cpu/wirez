package command

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/v-byte-cpu/wirez/pkg/connect"
)

func TestParseProxyFile(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    []*connect.SocksAddr
		expectedErr bool
	}{
		{
			name:  "EmptyString",
			input: "",
		},
		{
			name:  "StringWithSpaces",
			input: "   ",
		},
		{
			name:  "Newline",
			input: "\n",
		},
		{
			name:  "TwoLinesWithSpaces",
			input: "  \n   ",
		},
		{
			name:  "CommentSign",
			input: "#",
		},
		{
			name:  "CommentSignWithSpaces",
			input: "  #  ",
		},
		{
			name:  "TwoLinesWithComments",
			input: " # \n#  ",
		},
		{
			name:     "OneIPPort",
			input:    "10.10.10.10:1111",
			expected: []*connect.SocksAddr{{Address: "10.10.10.10:1111"}},
		},
		{
			name:     "OneIPPortWithSpaces",
			input:    "  10.10.10.10:1111   ",
			expected: []*connect.SocksAddr{{Address: "10.10.10.10:1111"}},
		},
		{
			name:     "OneHostPort",
			input:    "example.com:1111",
			expected: []*connect.SocksAddr{{Address: "example.com:1111"}},
		},
		{
			name:     "TwoIPPortLines",
			input:    "10.10.10.10:1111\n20.20.20.20:2222",
			expected: []*connect.SocksAddr{{Address: "10.10.10.10:1111"}, {Address: "20.20.20.20:2222"}},
		},
		{
			name:     "TwoHostPortLines",
			input:    "example.com:1111\nexample.org:2222",
			expected: []*connect.SocksAddr{{Address: "example.com:1111"}, {Address: "example.org:2222"}},
		},
		{
			name:     "OneIPPortWithUsername",
			input:    "abc@10.10.10.10:1111",
			expected: []*connect.SocksAddr{{Address: "10.10.10.10:1111", Auth: url.User("abc")}},
		},
		{
			name:     "OneIPPortWithUsernamePassword",
			input:    "abc:def@10.10.10.10:1111",
			expected: []*connect.SocksAddr{{Address: "10.10.10.10:1111", Auth: url.UserPassword("abc", "def")}},
		},
		{
			name:     "OneHostPortWithUsernamePassword",
			input:    "abc:def@example.com:1111",
			expected: []*connect.SocksAddr{{Address: "example.com:1111", Auth: url.UserPassword("abc", "def")}},
		},
		{
			name:     "WithSocks5Scheme",
			input:    "socks5://10.10.10.10:1111",
			expected: []*connect.SocksAddr{{Address: "10.10.10.10:1111"}},
		},
		{
			name:     "TwoWithSocks5Schemes",
			input:    "socks5://10.10.10.10:1111\nsocks5://20.20.20.20:2221",
			expected: []*connect.SocksAddr{{Address: "10.10.10.10:1111"}, {Address: "20.20.20.20:2221"}},
		},
		{
			name:        "WithInvalidScheme",
			input:       "socks3://10.10.10.10:1111",
			expectedErr: true,
		},
		{
			name:        "OneIPPortWithInvalidColons",
			input:       "abc@def:10.10.10.10:1111",
			expectedErr: true,
		},
		{
			name:        "OneIPPortWithComment",
			input:       "10.10.10.10:1111 #hello",
			expectedErr: true,
		},
		{
			name:        "OneIPPortWithInvalidPort",
			input:       "10.10.10.10:abc",
			expectedErr: true,
		},
		{
			name:        "OneIP",
			input:       "10.10.10.10",
			expectedErr: true,
		},
		{
			name:        "ParsingErrorAfterOneValid",
			input:       "10.10.10.10:1111\n10.10.10.13",
			expectedErr: true,
		},
		{
			name:        "ParsingErrorBeforeOneValid",
			input:       " 10.10.10.13\n10.10.10.10:1111",
			expectedErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			socksAddrs, err := parseProxyFile(strings.NewReader(tt.input))
			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expected, socksAddrs)
		})
	}
}
