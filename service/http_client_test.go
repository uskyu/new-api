package service

import (
	"math"
	"net/http"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRelayResponseHeaderTimeout(t *testing.T) {
	tests := []struct {
		name    string
		seconds int
		want    time.Duration
	}{
		{name: "default", seconds: 1800, want: 1800 * time.Second},
		{name: "disabled", seconds: 0, want: 0},
		{name: "negative", seconds: -1, want: 0},
		{name: "overflow clamped", seconds: maxTimeoutSeconds + 1, want: time.Duration(maxTimeoutSeconds) * time.Second},
		{name: "max int clamped", seconds: math.MaxInt, want: time.Duration(maxTimeoutSeconds) * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, relayResponseHeaderTimeout(tt.seconds))
		})
	}
}

func TestApplyRelayResponseHeaderTimeout(t *testing.T) {
	previous := common.RelayResponseHeaderTimeout
	t.Cleanup(func() {
		common.RelayResponseHeaderTimeout = previous
	})

	common.RelayResponseHeaderTimeout = 1800
	transport := &http.Transport{}
	applyRelayResponseHeaderTimeout(transport)
	require.Equal(t, 1800*time.Second, transport.ResponseHeaderTimeout)

	common.RelayResponseHeaderTimeout = 0
	transport = &http.Transport{}
	applyRelayResponseHeaderTimeout(transport)
	require.Zero(t, transport.ResponseHeaderTimeout)

	common.RelayResponseHeaderTimeout = maxTimeoutSeconds + 1
	transport = &http.Transport{}
	applyRelayResponseHeaderTimeout(transport)
	require.Equal(t, time.Duration(maxTimeoutSeconds)*time.Second, transport.ResponseHeaderTimeout)
	require.Positive(t, transport.ResponseHeaderTimeout)
}

func TestRelayTransportsCloneInsecureTLSConfig(t *testing.T) {
	previousSkip := common.TLSInsecureSkipVerify
	previousClient := httpClient
	common.TLSInsecureSkipVerify = true
	InitHttpClient()
	t.Cleanup(func() {
		common.TLSInsecureSkipVerify = previousSkip
		httpClient = previousClient
		proxyClientLock.Lock()
		delete(proxyClients, "http://proxy-one.example:8080")
		delete(proxyClients, "http://proxy-two.example:8080")
		proxyClientLock.Unlock()
	})

	directTransport := GetHttpClient().Transport.(*http.Transport)
	firstClient, err := NewProxyHttpClient("http://proxy-one.example:8080")
	require.NoError(t, err)
	secondClient, err := NewProxyHttpClient("http://proxy-two.example:8080")
	require.NoError(t, err)
	firstTransport := firstClient.Transport.(*http.Transport)
	secondTransport := secondClient.Transport.(*http.Transport)

	for _, transport := range []*http.Transport{directTransport, firstTransport, secondTransport} {
		require.NotNil(t, transport.TLSClientConfig)
		assert.NotSame(t, common.InsecureTLSConfig, transport.TLSClientConfig)
		assert.True(t, transport.TLSClientConfig.InsecureSkipVerify)
	}

	assert.NotSame(t, firstTransport.TLSClientConfig, secondTransport.TLSClientConfig)
	firstTransport.TLSClientConfig.InsecureSkipVerify = false
	assert.True(t, secondTransport.TLSClientConfig.InsecureSkipVerify)
	assert.True(t, directTransport.TLSClientConfig.InsecureSkipVerify)
	assert.True(t, common.InsecureTLSConfig.InsecureSkipVerify)
}
