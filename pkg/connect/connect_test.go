package connect

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddressMapper(t *testing.T) {
	t.Run("EmptyMapper", func(t *testing.T) {
		m := NewAddressMapper()
		_, ok := m.MapAddress("tcp", "1.1.1.1:53")
		require.False(t, ok)
	})
	t.Run("OneFullAddressMapped", func(t *testing.T) {
		m := NewAddressMapper()
		err := m.AddAddressMapping("tcp", "1.1.1.1:53", "127.0.0.1:5353")
		require.NoError(t, err)

		mappedAddress, ok := m.MapAddress("tcp", "1.1.1.1:53")
		require.True(t, ok)
		require.Equal(t, "127.0.0.1:5353", mappedAddress)
	})
	t.Run("OneFullAddressNotMapped", func(t *testing.T) {
		m := NewAddressMapper()
		err := m.AddAddressMapping("tcp", "1.1.1.1:53", "127.0.0.1:5353")
		require.NoError(t, err)

		_, ok := m.MapAddress("tcp", "2.2.2.2:53")
		require.False(t, ok)
	})
	t.Run("TwoFullAddressesMapped", func(t *testing.T) {
		m := NewAddressMapper()
		err := m.AddAddressMapping("tcp", "1.1.1.1:53", "127.0.0.1:5353")
		require.NoError(t, err)
		err = m.AddAddressMapping("tcp", "8.8.8.8:1031", "2.2.2.2:5454")
		require.NoError(t, err)

		mappedAddress, ok := m.MapAddress("tcp", "1.1.1.1:53")
		require.True(t, ok)
		require.Equal(t, "127.0.0.1:5353", mappedAddress)
		mappedAddress, ok = m.MapAddress("tcp", "8.8.8.8:1031")
		require.True(t, ok)
		require.Equal(t, "2.2.2.2:5454", mappedAddress)
	})
	t.Run("TCPAndUDPFullAddressesMapped", func(t *testing.T) {
		m := NewAddressMapper()
		err := m.AddAddressMapping("tcp", "1.1.1.1:53", "127.0.0.1:5353")
		require.NoError(t, err)
		err = m.AddAddressMapping("udp", "1.1.1.1:53", "2.2.2.2:5454")
		require.NoError(t, err)

		mappedAddress, ok := m.MapAddress("tcp", "1.1.1.1:53")
		require.True(t, ok)
		require.Equal(t, "127.0.0.1:5353", mappedAddress)
		mappedAddress, ok = m.MapAddress("udp", "1.1.1.1:53")
		require.True(t, ok)
		require.Equal(t, "2.2.2.2:5454", mappedAddress)
	})
	t.Run("TCPPortMapped", func(t *testing.T) {
		m := NewAddressMapper()
		err := m.AddAddressMapping("tcp", "53", "127.0.0.1:5353")
		require.NoError(t, err)

		mappedAddress, ok := m.MapAddress("tcp", "1.1.1.1:53")
		require.True(t, ok)
		require.Equal(t, "127.0.0.1:5353", mappedAddress)
		mappedAddress, ok = m.MapAddress("tcp", "2.2.2.2:53")
		require.True(t, ok)
		require.Equal(t, "127.0.0.1:5353", mappedAddress)
	})
	t.Run("TCPPortWithColonMapped", func(t *testing.T) {
		m := NewAddressMapper()
		err := m.AddAddressMapping("tcp", ":53", "127.0.0.1:5353")
		require.NoError(t, err)

		mappedAddress, ok := m.MapAddress("tcp", "1.1.1.1:53")
		require.True(t, ok)
		require.Equal(t, "127.0.0.1:5353", mappedAddress)
		mappedAddress, ok = m.MapAddress("tcp", "2.2.2.2:53")
		require.True(t, ok)
		require.Equal(t, "127.0.0.1:5353", mappedAddress)
	})
	t.Run("TCPPortWithAnyIPMapped", func(t *testing.T) {
		m := NewAddressMapper()
		err := m.AddAddressMapping("tcp", "0.0.0.0:53", "127.0.0.1:5353")
		require.NoError(t, err)

		mappedAddress, ok := m.MapAddress("tcp", "1.1.1.1:53")
		require.True(t, ok)
		require.Equal(t, "127.0.0.1:5353", mappedAddress)
		mappedAddress, ok = m.MapAddress("tcp", "2.2.2.2:53")
		require.True(t, ok)
		require.Equal(t, "127.0.0.1:5353", mappedAddress)
	})
	t.Run("TCPAndUDPPortMapped", func(t *testing.T) {
		m := NewAddressMapper()
		err := m.AddAddressMapping("tcp", "53", "127.0.0.1:5353")
		require.NoError(t, err)
		err = m.AddAddressMapping("udp", "53", "1.2.3.4:5454")
		require.NoError(t, err)

		mappedAddress, ok := m.MapAddress("tcp", "1.1.1.1:53")
		require.True(t, ok)
		require.Equal(t, "127.0.0.1:5353", mappedAddress)
		mappedAddress, ok = m.MapAddress("udp", "2.2.2.2:53")
		require.True(t, ok)
		require.Equal(t, "1.2.3.4:5454", mappedAddress)
	})
	t.Run("InvalidAddressAndPortError", func(t *testing.T) {
		m := NewAddressMapper()
		err := m.AddAddressMapping("tcp", "1.1.1.1:53:53", "127.0.0.1:5353")
		require.Error(t, err)
		err = m.AddAddressMapping("tcp", "1.1.1.1:abc", "127.0.0.1:5353")
		require.Error(t, err)
		err = m.AddAddressMapping("tcp", "abc", "127.0.0.1:5353")
		require.Error(t, err)
	})
	t.Run("OneFullIPv6AddressMapped", func(t *testing.T) {
		m := NewAddressMapper()
		err := m.AddAddressMapping("tcp", "[2001:db8::2:1]:53", "127.0.0.1:5353")
		require.NoError(t, err)

		mappedAddress, ok := m.MapAddress("tcp", "[2001:db8::2:1]:53")
		require.True(t, ok)
		require.Equal(t, "127.0.0.1:5353", mappedAddress)
	})

}
