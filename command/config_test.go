package command

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/v-byte-cpu/wirez/pkg/connect"
)

func TestParseProxyURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    *connect.SocksAddr
		expectedErr bool
	}{
		{
			name:        "EmptyString",
			input:       "",
			expectedErr: true,
		},
		{
			name:        "StringWithSpaces",
			input:       "   ",
			expectedErr: true,
		},
		{
			name:        "CommentSign",
			input:       "#",
			expectedErr: true,
		},
		{
			name:     "OneIPPort",
			input:    "10.10.10.10:1111",
			expected: &connect.SocksAddr{Address: "10.10.10.10:1111"},
		},
		{
			name:     "OneIPPortWithSpaces",
			input:    "  10.10.10.10:1111   ",
			expected: &connect.SocksAddr{Address: "10.10.10.10:1111"},
		},
		{
			name:     "OneHostPort",
			input:    "example.com:1111",
			expected: &connect.SocksAddr{Address: "example.com:1111"},
		},
		{
			name:     "OneIPPortWithUsername",
			input:    "abc@10.10.10.10:1111",
			expected: &connect.SocksAddr{Address: "10.10.10.10:1111", Auth: url.User("abc")},
		},
		{
			name:     "OneIPPortWithUsernamePassword",
			input:    "abc:def@10.10.10.10:1111",
			expected: &connect.SocksAddr{Address: "10.10.10.10:1111", Auth: url.UserPassword("abc", "def")},
		},
		{
			name:     "OneHostPortWithUsernamePassword",
			input:    "abc:def@example.com:1111",
			expected: &connect.SocksAddr{Address: "example.com:1111", Auth: url.UserPassword("abc", "def")},
		},
		{
			name:     "WithSocks5Scheme",
			input:    "socks5://10.10.10.10:1111",
			expected: &connect.SocksAddr{Address: "10.10.10.10:1111"},
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			socksAddr, err := parseProxyURL(tt.input)
			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expected, socksAddr)
		})
	}
}

func TestParseProxyURLs(t *testing.T) {
	tests := []struct {
		name        string
		input       []string
		expected    []*connect.SocksAddr
		expectedErr bool
	}{
		{
			name:     "OneAddress",
			input:    []string{"10.10.10.10:1111"},
			expected: []*connect.SocksAddr{{Address: "10.10.10.10:1111"}},
		},
		{
			name:     "TwoAddresses",
			input:    []string{"10.10.10.10:1111", "socks5://20.20.20.20:2221"},
			expected: []*connect.SocksAddr{{Address: "10.10.10.10:1111"}, {Address: "20.20.20.20:2221"}},
		},
		{
			name:        "Error",
			input:       []string{"socks3://10.10.10.10:1111"},
			expectedErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			socksAddrs, err := parseProxyURLs(tt.input)
			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expected, socksAddrs)
		})
	}
}

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

func TestParseMapping(t *testing.T) {
	t.Run("OneFullUDPMapping", func(t *testing.T) {
		network, fromAddress, toAddress, err := parseMapping("1.1.1.1:53:127.0.0.1:5341/udp")
		require.NoError(t, err)
		require.Equal(t, "udp", network)
		require.Equal(t, "1.1.1.1:53", fromAddress)
		require.Equal(t, "127.0.0.1:5341", toAddress)
	})
	t.Run("OneFullTCPMapping", func(t *testing.T) {
		network, fromAddress, toAddress, err := parseMapping("1.1.1.1:53:127.0.0.1:5341/tcp")
		require.NoError(t, err)
		require.Equal(t, "tcp", network)
		require.Equal(t, "1.1.1.1:53", fromAddress)
		require.Equal(t, "127.0.0.1:5341", toAddress)
	})
	t.Run("OnePortUDPMapping", func(t *testing.T) {
		network, fromAddress, toAddress, err := parseMapping("53:127.0.0.1:5341/udp")
		require.NoError(t, err)
		require.Equal(t, "udp", network)
		require.Equal(t, ":53", fromAddress)
		require.Equal(t, "127.0.0.1:5341", toAddress)
	})
	t.Run("InvalidTargetPortMapping", func(t *testing.T) {
		_, _, _, err := parseMapping("53:127.0.0.1:abc/udp")
		require.Error(t, err)
	})
	t.Run("InvalidFromPortMapping", func(t *testing.T) {
		_, _, _, err := parseMapping("1.1.1.1:abc:127.0.0.1:5353/udp")
		require.Error(t, err)
	})
	t.Run("InvalidEmptyFromPortMapping", func(t *testing.T) {
		_, _, _, err := parseMapping("127.0.0.1:5353/udp")
		require.Error(t, err)
	})
	t.Run("OneMappingWithoutNetwork", func(t *testing.T) {
		network, fromAddress, toAddress, err := parseMapping("2.2.2.2:8080:127.0.0.1:5341")
		require.NoError(t, err)
		require.Equal(t, "tcp", network)
		require.Equal(t, "2.2.2.2:8080", fromAddress)
		require.Equal(t, "127.0.0.1:5341", toAddress)
	})
	t.Run("InvalidMappingWithDoubledSourceIP", func(t *testing.T) {
		_, _, _, err := parseMapping("2.2.2.2:2.2.2.2:8080:127.0.0.1:5341")
		require.Error(t, err)
	})
	t.Run("OneTargetIPv6Mapping", func(t *testing.T) {
		network, fromAddress, toAddress, err := parseMapping("1.1.1.1:53:[::1]:5353/udp")
		require.NoError(t, err)
		require.Equal(t, "udp", network)
		require.Equal(t, "1.1.1.1:53", fromAddress)
		require.Equal(t, "[::1]:5353", toAddress)
	})
	t.Run("OneSourceIPv6Mapping", func(t *testing.T) {
		network, fromAddress, toAddress, err := parseMapping("[2001:db8::2:1]:5353:1.1.1.1:53/udp")
		require.NoError(t, err)
		require.Equal(t, "udp", network)
		require.Equal(t, "[2001:db8::2:1]:5353", fromAddress)
		require.Equal(t, "1.1.1.1:53", toAddress)
	})
	t.Run("InvalidTargetEmptyMapping", func(t *testing.T) {
		_, _, _, err := parseMapping("1.1.1.1:53::5353/udp")
		require.Error(t, err)
	})
	t.Run("InvalidTargetIPv6MissingBracketMapping", func(t *testing.T) {
		_, _, _, err := parseMapping("1.1.1.1:53:1]:5353/udp")
		require.Error(t, err)
	})
	t.Run("InvalidTargetIPv6Mapping", func(t *testing.T) {
		_, _, _, err := parseMapping("1.1.1.1:53:[abc]:5353/udp")
		require.Error(t, err)
	})
	t.Run("InvalidTargetEmptyIPv6Mapping", func(t *testing.T) {
		_, _, _, err := parseMapping("1.1.1.1:53:[]:5353/udp")
		require.Error(t, err)
	})
	t.Run("MissingColonBetweenFromPortAndTargetHostIPv6", func(t *testing.T) {
		_, _, _, err := parseMapping("1.1.1.1:53[2001:db8::2:1]:5353/udp")
		require.Error(t, err)
	})
	t.Run("InvalidSourceIPv6MissingBracketMapping", func(t *testing.T) {
		_, _, _, err := parseMapping("1]:5353:1.1.1.1:53/udp")
		require.Error(t, err)
	})
	t.Run("InvalidSourceIPv6Mapping", func(t *testing.T) {
		_, _, _, err := parseMapping("[abc]:5353:1.1.1.1:53/udp")
		require.Error(t, err)
	})
	t.Run("InvalidSourceEmptyIPv6Mapping", func(t *testing.T) {
		_, _, _, err := parseMapping("[]:5353:1.1.1.1:53/udp")
		require.Error(t, err)
	})
}

func TestParseAddressMapper(t *testing.T) {
	t.Run("EmptyMapper", func(t *testing.T) {
		m, err := parseAddressMapper(nil)
		require.NoError(t, err)
		_, exists := m.MapAddress("udp", "8.8.8.8:53")
		require.False(t, exists)
	})
	t.Run("OneFullUDPMapping", func(t *testing.T) {
		m, err := parseAddressMapper([]string{"8.8.8.8:53:127.0.0.1:5341/udp"})
		require.NoError(t, err)
		targetAddress, exists := m.MapAddress("udp", "8.8.8.8:53")
		require.True(t, exists)
		require.Equal(t, "127.0.0.1:5341", targetAddress)
	})
	t.Run("OneFullTCPMapping", func(t *testing.T) {
		m, err := parseAddressMapper([]string{"1.1.1.1:53:127.0.0.1:5341/tcp"})
		require.NoError(t, err)
		targetAddress, exists := m.MapAddress("tcp", "1.1.1.1:53")
		require.True(t, exists)
		require.Equal(t, "127.0.0.1:5341", targetAddress)
	})
	t.Run("OnePortUDPMapping", func(t *testing.T) {
		m, err := parseAddressMapper([]string{"53:127.0.0.1:5341/udp"})
		require.NoError(t, err)
		targetAddress, exists := m.MapAddress("udp", "8.8.8.8:53")
		require.True(t, exists)
		require.Equal(t, "127.0.0.1:5341", targetAddress)
	})
	t.Run("OneMappingWithError", func(t *testing.T) {
		_, err := parseAddressMapper([]string{"1.1.1.1:abc:127.0.0.1:5353/udp"})
		require.Error(t, err)
	})
	t.Run("TwoFullAddressMappings", func(t *testing.T) {
		m, err := parseAddressMapper([]string{"1.1.1.1:53:127.0.0.1:5341/udp", "2.2.2.2:53:1.1.1.1:5341/udp"})
		require.NoError(t, err)
		targetAddress, exists := m.MapAddress("udp", "1.1.1.1:53")
		require.True(t, exists)
		require.Equal(t, "127.0.0.1:5341", targetAddress)
		targetAddress, exists = m.MapAddress("udp", "2.2.2.2:53")
		require.True(t, exists)
		require.Equal(t, "1.1.1.1:5341", targetAddress)
	})
	t.Run("TwoFullUDPAndTCPSameAddressMappings", func(t *testing.T) {
		m, err := parseAddressMapper([]string{"1.1.1.1:53:127.0.0.1:5341/udp", "1.1.1.1:53:8.8.8.8:1234/tcp"})
		require.NoError(t, err)
		targetAddress, exists := m.MapAddress("udp", "1.1.1.1:53")
		require.True(t, exists)
		require.Equal(t, "127.0.0.1:5341", targetAddress)
		targetAddress, exists = m.MapAddress("tcp", "1.1.1.1:53")
		require.True(t, exists)
		require.Equal(t, "8.8.8.8:1234", targetAddress)
	})
	t.Run("TwoFullAddressMappingsWithError", func(t *testing.T) {
		_, err := parseAddressMapper([]string{"1.1.1.1:abc:127.0.0.1:5341/udp", "2.2.2.2:53:1.1.1.1:5341/udp"})
		require.Error(t, err)
	})
	t.Run("TwoPortMappings", func(t *testing.T) {
		m, err := parseAddressMapper([]string{"53:127.0.0.1:5341/udp", "8080:127.0.0.1:4444/udp"})
		require.NoError(t, err)
		targetAddress, exists := m.MapAddress("udp", "1.1.1.1:53")
		require.True(t, exists)
		require.Equal(t, "127.0.0.1:5341", targetAddress)
		targetAddress, exists = m.MapAddress("udp", "2.2.2.2:8080")
		require.True(t, exists)
		require.Equal(t, "127.0.0.1:4444", targetAddress)
	})
}
