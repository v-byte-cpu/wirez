package command

import (
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/v-byte-cpu/wirez/pkg/connect"
)

var bothProtocols = []string{"tcp", "udp"}

func TestParseProxyURL(t *testing.T) {
	t.Parallel()

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
			t.Parallel()

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
	t.Parallel()

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
			t.Parallel()

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
	t.Parallel()

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
			t.Parallel()

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
	t.Parallel()

	t.Run("OneFullUDPMapping", func(t *testing.T) {
		t.Parallel()

		network, fromAddress, toAddress, err := parseMapping("1.1.1.1:53:127.0.0.1:5341/udp")
		require.NoError(t, err)
		require.Equal(t, "udp", network)
		require.Equal(t, "1.1.1.1:53", fromAddress)
		require.Equal(t, "127.0.0.1:5341", toAddress)
	})
	t.Run("OneFullTCPMapping", func(t *testing.T) {
		t.Parallel()

		network, fromAddress, toAddress, err := parseMapping("1.1.1.1:53:127.0.0.1:5341/tcp")
		require.NoError(t, err)
		require.Equal(t, "tcp", network)
		require.Equal(t, "1.1.1.1:53", fromAddress)
		require.Equal(t, "127.0.0.1:5341", toAddress)
	})
	t.Run("OnePortUDPMapping", func(t *testing.T) {
		t.Parallel()

		network, fromAddress, toAddress, err := parseMapping("53:127.0.0.1:5341/udp")
		require.NoError(t, err)
		require.Equal(t, "udp", network)
		require.Equal(t, ":53", fromAddress)
		require.Equal(t, "127.0.0.1:5341", toAddress)
	})
	t.Run("InvalidTargetPortMapping", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("53:127.0.0.1:abc/udp")
		require.Error(t, err)
	})
	t.Run("InvalidFromPortMapping", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("1.1.1.1:abc:127.0.0.1:5353/udp")
		require.Error(t, err)
	})
	t.Run("InvalidEmptyFromPortMapping", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("127.0.0.1:5353/udp")
		require.Error(t, err)
	})
	t.Run("OneMappingWithoutNetwork", func(t *testing.T) {
		t.Parallel()

		network, fromAddress, toAddress, err := parseMapping("2.2.2.2:8080:127.0.0.1:5341")
		require.NoError(t, err)
		require.Equal(t, "tcp", network)
		require.Equal(t, "2.2.2.2:8080", fromAddress)
		require.Equal(t, "127.0.0.1:5341", toAddress)
	})
	t.Run("InvalidMappingWithDoubledSourceIP", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("2.2.2.2:2.2.2.2:8080:127.0.0.1:5341")
		require.Error(t, err)
	})
	t.Run("OneTargetIPv6Mapping", func(t *testing.T) {
		t.Parallel()

		network, fromAddress, toAddress, err := parseMapping("1.1.1.1:53:[::1]:5353/udp")
		require.NoError(t, err)
		require.Equal(t, "udp", network)
		require.Equal(t, "1.1.1.1:53", fromAddress)
		require.Equal(t, "[::1]:5353", toAddress)
	})
	t.Run("OneSourceIPv6Mapping", func(t *testing.T) {
		t.Parallel()

		network, fromAddress, toAddress, err := parseMapping("[2001:db8::2:1]:5353:1.1.1.1:53/udp")
		require.NoError(t, err)
		require.Equal(t, "udp", network)
		require.Equal(t, "[2001:db8::2:1]:5353", fromAddress)
		require.Equal(t, "1.1.1.1:53", toAddress)
	})
	t.Run("InvalidTargetEmptyMapping", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("1.1.1.1:53::5353/udp")
		require.Error(t, err)
	})
	t.Run("InvalidTargetIPv6MissingBracketMapping", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("1.1.1.1:53:1]:5353/udp")
		require.Error(t, err)
	})
	t.Run("InvalidTargetIPv6Mapping", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("1.1.1.1:53:[abc]:5353/udp")
		require.Error(t, err)
	})
	t.Run("InvalidTargetEmptyIPv6Mapping", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("1.1.1.1:53:[]:5353/udp")
		require.Error(t, err)
	})
	t.Run("MissingColonBetweenFromPortAndTargetHostIPv6", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("1.1.1.1:53[2001:db8::2:1]:5353/udp")
		require.Error(t, err)
	})
	t.Run("InvalidSourceIPv6MissingBracketMapping", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("1]:5353:1.1.1.1:53/udp")
		require.Error(t, err)
	})
	t.Run("InvalidSourceIPv6Mapping", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("[abc]:5353:1.1.1.1:53/udp")
		require.Error(t, err)
	})
	t.Run("InvalidSourceEmptyIPv6Mapping", func(t *testing.T) {
		t.Parallel()

		_, _, _, err := parseMapping("[]:5353:1.1.1.1:53/udp")
		require.Error(t, err)
	})
}

func TestParseAddressMapper(t *testing.T) {
	t.Parallel()

	t.Run("EmptyMapper", func(t *testing.T) {
		t.Parallel()

		m, err := parseAddressMapper(nil)
		require.NoError(t, err)
		_, exists := m.MapAddress("udp", "8.8.8.8:53")
		require.False(t, exists)
	})
	t.Run("OneFullUDPMapping", func(t *testing.T) {
		t.Parallel()

		m, err := parseAddressMapper([]string{"8.8.8.8:53:127.0.0.1:5341/udp"})
		require.NoError(t, err)
		targetAddress, exists := m.MapAddress("udp", "8.8.8.8:53")
		require.True(t, exists)
		require.Equal(t, "127.0.0.1:5341", targetAddress)
	})
	t.Run("OneFullTCPMapping", func(t *testing.T) {
		t.Parallel()

		m, err := parseAddressMapper([]string{"1.1.1.1:53:127.0.0.1:5341/tcp"})
		require.NoError(t, err)
		targetAddress, exists := m.MapAddress("tcp", "1.1.1.1:53")
		require.True(t, exists)
		require.Equal(t, "127.0.0.1:5341", targetAddress)
	})
	t.Run("OnePortUDPMapping", func(t *testing.T) {
		t.Parallel()

		m, err := parseAddressMapper([]string{"53:127.0.0.1:5341/udp"})
		require.NoError(t, err)
		targetAddress, exists := m.MapAddress("udp", "8.8.8.8:53")
		require.True(t, exists)
		require.Equal(t, "127.0.0.1:5341", targetAddress)
	})
	t.Run("OneMappingWithError", func(t *testing.T) {
		t.Parallel()

		_, err := parseAddressMapper([]string{"1.1.1.1:abc:127.0.0.1:5353/udp"})
		require.Error(t, err)
	})
	t.Run("TwoFullAddressMappings", func(t *testing.T) {
		t.Parallel()

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
		t.Parallel()

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
		t.Parallel()

		_, err := parseAddressMapper([]string{"1.1.1.1:abc:127.0.0.1:5341/udp", "2.2.2.2:53:1.1.1.1:5341/udp"})
		require.Error(t, err)
	})
	t.Run("TwoPortMappings", func(t *testing.T) {
		t.Parallel()

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

func TestParseDNSServers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       []string
		expected    []dnsServer
		expectedErr bool
	}{
		{
			name: "EmptyList",
		},
		{
			name:     "OneIPv4WithoutPort",
			input:    []string{"1.1.1.1"},
			expected: []dnsServer{{address: "1.1.1.1:53", networks: bothProtocols}},
		},
		{
			name:     "OneIPv4WithPort",
			input:    []string{"1.1.1.1:5353"},
			expected: []dnsServer{{address: "1.1.1.1:5353", networks: bothProtocols}},
		},
		{
			name:     "OneIPv4WithSpaces",
			input:    []string{"  1.1.1.1  "},
			expected: []dnsServer{{address: "1.1.1.1:53", networks: bothProtocols}},
		},
		{
			name:  "TwoIPv4",
			input: []string{"1.1.1.1", "8.8.8.8:5353"},
			expected: []dnsServer{
				{address: "1.1.1.1:53", networks: bothProtocols},
				{address: "8.8.8.8:5353", networks: bothProtocols},
			},
		},
		{
			name:     "OneIPv6WithoutPort",
			input:    []string{"2001:db8::1"},
			expected: []dnsServer{{address: "[2001:db8::1]:53", networks: bothProtocols}},
		},
		{
			name:     "OneIPv6WithPort",
			input:    []string{"[2001:db8::1]:5353"},
			expected: []dnsServer{{address: "[2001:db8::1]:5353", networks: bothProtocols}},
		},
		{
			name:     "OneIPv4WithTCPProto",
			input:    []string{"1.1.1.1/tcp"},
			expected: []dnsServer{{address: "1.1.1.1:53", networks: []string{"tcp"}}},
		},
		{
			name:     "OneIPv4WithUDPProto",
			input:    []string{"1.1.1.1/udp"},
			expected: []dnsServer{{address: "1.1.1.1:53", networks: []string{"udp"}}},
		},
		{
			name:     "OneIPv4WithPortAndTCPProto",
			input:    []string{"1.1.1.1:5353/tcp"},
			expected: []dnsServer{{address: "1.1.1.1:5353", networks: []string{"tcp"}}},
		},
		{
			name:     "OneIPv6WithUDPProto",
			input:    []string{"[2001:db8::1]/udp"},
			expected: []dnsServer{{address: "[2001:db8::1]:53", networks: []string{"udp"}}},
		},
		{
			name:        "EmptyString",
			input:       []string{""},
			expectedErr: true,
		},
		{
			name:        "InvalidHost",
			input:       []string{"dns.example.com"},
			expectedErr: true,
		},
		{
			name:        "InvalidPort",
			input:       []string{"1.1.1.1:abc"},
			expectedErr: true,
		},
		{
			name:        "InvalidPortOutOfRange",
			input:       []string{"1.1.1.1:70000"},
			expectedErr: true,
		},
		{
			name:        "InvalidProto",
			input:       []string{"1.1.1.1/sctp"},
			expectedErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			servers, err := parseDNSServers(tt.input)
			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expected, servers)
		})
	}
}

func TestParseDNSForceServer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		expected    string
		expectedErr bool
	}{
		{
			name: "EmptyString",
		},
		{
			name:     "IPv4WithoutPort",
			input:    "1.1.1.1",
			expected: "1.1.1.1:53",
		},
		{
			name:     "IPv4WithPort",
			input:    "1.1.1.1:5353",
			expected: "1.1.1.1:5353",
		},
		{
			name:     "IPv4WithSpaces",
			input:    "  1.1.1.1  ",
			expected: "1.1.1.1:53",
		},
		{
			name:     "IPv6WithoutPort",
			input:    "2001:db8::1",
			expected: "[2001:db8::1]:53",
		},
		{
			name:        "InvalidHost",
			input:       "dns.example.com",
			expectedErr: true,
		},
		{
			name:        "InvalidPort",
			input:       "1.1.1.1:abc",
			expectedErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			addr, err := parseDNSForceServer(tt.input)
			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.expected, addr)
		})
	}
}

func TestDNSForceMappings(t *testing.T) {
	t.Parallel()

	t.Run("EmptyServer", func(t *testing.T) {
		t.Parallel()

		require.Nil(t, dnsForceMappings(""))
	})
	t.Run("OneServer", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, []string{":53:1.1.1.1:53/udp", ":53:1.1.1.1:53/tcp"},
			dnsForceMappings("1.1.1.1:53"))
	})
	t.Run("MappedByMapper", func(t *testing.T) {
		t.Parallel()

		m, err := parseAddressMapper(dnsForceMappings("1.1.1.1:53"))
		require.NoError(t, err)
		for _, network := range []string{"udp", "tcp"} {
			targetAddress, exists := m.MapAddress(network, "8.8.8.8:53")
			require.True(t, exists)
			require.Equal(t, "1.1.1.1:53", targetAddress)
		}
	})
}

func TestDNSServerMappings(t *testing.T) {
	t.Parallel()

	t.Run("EmptyList", func(t *testing.T) {
		t.Parallel()

		require.Nil(t, dnsServerMappings(nil))
	})
	t.Run("BothProtocolsByDefault", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, []string{":53:8.8.8.8:53/tcp", ":53:8.8.8.8:53/udp"},
			dnsServerMappings([]dnsServer{{address: "8.8.8.8:53", networks: bothProtocols}}))
	})
	t.Run("TCPOnly", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, []string{":53:8.8.8.8:5353/tcp"},
			dnsServerMappings([]dnsServer{{address: "8.8.8.8:5353", networks: []string{"tcp"}}}))
	})
	t.Run("UDPOnly", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, []string{":53:8.8.8.8:53/udp"},
			dnsServerMappings([]dnsServer{{address: "8.8.8.8:53", networks: []string{"udp"}}}))
	})
	t.Run("MappedByMapper", func(t *testing.T) {
		t.Parallel()

		m, err := parseAddressMapper(dnsServerMappings([]dnsServer{{address: "8.8.8.8:5353", networks: []string{"udp"}}}))
		require.NoError(t, err)
		targetAddress, exists := m.MapAddress("udp", "9.9.9.9:53")
		require.True(t, exists)
		require.Equal(t, "8.8.8.8:5353", targetAddress)
		// only the configured protocol is matched
		_, exists = m.MapAddress("tcp", "9.9.9.9:53")
		require.False(t, exists)
	})
}

func TestDNSServerAddresses(t *testing.T) {
	t.Parallel()

	t.Run("EmptyList", func(t *testing.T) {
		t.Parallel()

		require.Nil(t, dnsServerAddresses(nil))
	})
	t.Run("TwoServers", func(t *testing.T) {
		t.Parallel()

		require.Equal(t, []string{"1.1.1.1:53", "8.8.8.8:5353"}, dnsServerAddresses([]dnsServer{
			{address: "1.1.1.1:53", networks: bothProtocols},
			{address: "8.8.8.8:5353", networks: []string{"tcp"}},
		}))
	})
}

func TestDNSUseVC(t *testing.T) {
	t.Parallel()

	t.Run("EmptyList", func(t *testing.T) {
		t.Parallel()

		require.False(t, dnsUseVC(nil))
	})
	t.Run("DefaultBothProtocols", func(t *testing.T) {
		t.Parallel()

		require.True(t, dnsUseVC([]dnsServer{{address: "8.8.8.8:53", networks: bothProtocols}}))
	})
	t.Run("TCPOnly", func(t *testing.T) {
		t.Parallel()

		require.True(t, dnsUseVC([]dnsServer{{address: "8.8.8.8:53", networks: []string{"tcp"}}}))
	})
	t.Run("UDPOnly", func(t *testing.T) {
		t.Parallel()

		require.False(t, dnsUseVC([]dnsServer{{address: "8.8.8.8:53", networks: []string{"udp"}}}))
	})
	t.Run("MixedPrefersTCP", func(t *testing.T) {
		t.Parallel()

		require.True(t, dnsUseVC([]dnsServer{
			{address: "1.1.1.1:53", networks: []string{"udp"}},
			{address: "8.8.8.8:53", networks: []string{"tcp"}},
		}))
	})
}
