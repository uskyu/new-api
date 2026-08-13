package channel

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	appcommon "github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/hpack"
)

func TestApplyUpstreamBodyMetadata(t *testing.T) {
	payload := []byte("replayable-body")
	storage, err := appcommon.CreateBodyStorage(payload)
	require.NoError(t, err)
	defer storage.Close()

	req, err := http.NewRequest(http.MethodPost, "https://example.com", appcommon.ReaderOnly(storage))
	require.NoError(t, err)
	require.Zero(t, req.ContentLength)
	require.Nil(t, req.GetBody)

	ApplyUpstreamBodyMetadata(req, &relaycommon.RelayInfo{
		UpstreamRequestBodySize: storage.Size(),
		UpstreamRequestGetBody:  storage.NewReader,
	})

	require.EqualValues(t, len(payload), req.ContentLength)
	require.NotNil(t, req.GetBody)
	for i := 0; i < 2; i++ {
		replay, err := req.GetBody()
		require.NoError(t, err)
		got, err := io.ReadAll(replay)
		require.NoError(t, err)
		require.NoError(t, replay.Close())
		require.Equal(t, payload, got)
	}
}

func TestApplyUpstreamBodyMetadataPreservesNativeMetadata(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "https://example.com", bytes.NewReader([]byte("native")))
	require.NoError(t, err)
	require.NotNil(t, req.GetBody)

	ApplyUpstreamBodyMetadata(req, &relaycommon.RelayInfo{
		UpstreamRequestBodySize: 99,
		UpstreamRequestGetBody: func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader([]byte("override"))), nil
		},
	})

	require.EqualValues(t, len("native"), req.ContentLength)
	replay, err := req.GetBody()
	require.NoError(t, err)
	defer replay.Close()
	got, err := io.ReadAll(replay)
	require.NoError(t, err)
	require.Equal(t, "native", string(got))
}

type getBodyTaskAdaptor struct {
	TaskAdaptor
	baseURL     string
	capturedReq *http.Request
}

func (a *getBodyTaskAdaptor) BuildRequestURL(*relaycommon.RelayInfo) (string, error) {
	return a.baseURL, nil
}

func (a *getBodyTaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	a.capturedReq = req
	return nil
}

func TestDoTaskApiRequestUsesCorrectReplayBody(t *testing.T) {
	service.InitHttpClient()
	payload := []byte(`{"model":"test","prompt":"hello"}`)

	received := make(chan []byte, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		received <- body
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", bytes.NewReader(payload))
	info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	adaptor := &getBodyTaskAdaptor{baseURL: server.URL}

	resp, err := DoTaskApiRequest(adaptor, c, info, bytes.NewReader(payload))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, payload, <-received)

	require.NotNil(t, adaptor.capturedReq)
	require.NotNil(t, adaptor.capturedReq.GetBody)
	for i := 0; i < 2; i++ {
		replay, err := adaptor.capturedReq.GetBody()
		require.NoError(t, err)
		got, err := io.ReadAll(replay)
		require.NoError(t, err)
		require.NoError(t, replay.Close())
		require.Equal(t, payload, got)
	}
}

func TestApplyUpstreamBodyMetadataHTTP2RetryAfterRSTStream(t *testing.T) {
	payload := []byte(`{"model":"test","prompt":"retry"}`)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	result := make(chan h2RetryResult, 1)
	go serveResetThenSuccess(listener, result)

	transport := &http2.Transport{
		AllowHTTP: true,
		DialTLSContext: func(ctx context.Context, network, _ string, _ *tls.Config) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, network, listener.Addr().String())
		},
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 15 * time.Second}

	storage, err := appcommon.CreateBodyStorage(payload)
	require.NoError(t, err)
	defer storage.Close()
	req, err := http.NewRequest(http.MethodPost, "http://upstream.test/v1/test", appcommon.ReaderOnly(storage))
	require.NoError(t, err)
	ApplyUpstreamBodyMetadata(req, &relaycommon.RelayInfo{
		UpstreamRequestBodySize: storage.Size(),
		UpstreamRequestGetBody:  storage.NewReader,
	})

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	select {
	case serverResult := <-result:
		require.NoError(t, serverResult.err)
		require.Equal(t, [][]byte{payload, payload}, serverResult.bodies)
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for HTTP/2 test server")
	}
}

type h2RetryResult struct {
	bodies [][]byte
	err    error
}

func serveResetThenSuccess(listener net.Listener, result chan<- h2RetryResult) {
	serverResult := h2RetryResult{}
	defer func() { result <- serverResult }()

	conn, err := listener.Accept()
	if err != nil {
		serverResult.err = err
		return
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(15 * time.Second))

	preface := make([]byte, len(http2.ClientPreface))
	if _, err = io.ReadFull(conn, preface); err != nil {
		serverResult.err = fmt.Errorf("read client preface: %w", err)
		return
	}
	if !bytes.Equal(preface, []byte(http2.ClientPreface)) {
		serverResult.err = fmt.Errorf("unexpected client preface")
		return
	}

	framer := http2.NewFramer(conn, conn)
	framer.ReadMetaHeaders = hpack.NewDecoder(4096, nil)
	if err = framer.WriteSettings(); err != nil {
		serverResult.err = err
		return
	}

	for attempt := 0; attempt < 2; attempt++ {
		streamID, body, readErr := readH2Request(framer)
		if readErr != nil {
			serverResult.err = readErr
			return
		}
		serverResult.bodies = append(serverResult.bodies, body)
		if attempt == 0 {
			if err = framer.WriteRSTStream(streamID, http2.ErrCodeRefusedStream); err != nil {
				serverResult.err = err
				return
			}
			continue
		}
		serverResult.err = writeH2OK(framer, streamID)
	}
}

func readH2Request(framer *http2.Framer) (uint32, []byte, error) {
	var streamID uint32
	var body []byte
	for {
		frame, err := framer.ReadFrame()
		if err != nil {
			return 0, nil, err
		}
		switch frame := frame.(type) {
		case *http2.SettingsFrame:
			if !frame.IsAck() {
				if err = framer.WriteSettingsAck(); err != nil {
					return 0, nil, err
				}
			}
		case *http2.MetaHeadersFrame:
			streamID = frame.Header().StreamID
			if frame.StreamEnded() {
				return streamID, body, nil
			}
		case *http2.DataFrame:
			if streamID == 0 {
				streamID = frame.Header().StreamID
			}
			if frame.Header().StreamID == streamID {
				body = append(body, frame.Data()...)
				if frame.StreamEnded() {
					return streamID, body, nil
				}
			}
		}
	}
}

func writeH2OK(framer *http2.Framer, streamID uint32) error {
	var headerBlock bytes.Buffer
	encoder := hpack.NewEncoder(&headerBlock)
	if err := encoder.WriteField(hpack.HeaderField{Name: ":status", Value: "200"}); err != nil {
		return err
	}
	if err := framer.WriteHeaders(http2.HeadersFrameParam{
		StreamID:      streamID,
		BlockFragment: headerBlock.Bytes(),
		EndHeaders:    true,
	}); err != nil {
		return err
	}
	return framer.WriteData(streamID, true, []byte(`{}`))
}
