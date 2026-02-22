package feishu

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultPingInterval     = 120 * time.Second
	defaultReconnectNonce   = 30 // seconds max jitter
	defaultReconnectWait    = 120 * time.Second
	frameTypeControl        = 0
	frameTypeData           = 1
	fragmentBufferTTL       = 5 * time.Second
)

// WSEventHandler processes incoming WebSocket events.
type WSEventHandler interface {
	HandleEvent(ctx context.Context, payload []byte) error
}

// WSClient is a native Feishu/Lark WebSocket client.
// Connects via the Lark WebSocket endpoint, handles protobuf frames,
// ping/pong, auto-reconnect, and fragment reassembly.
type WSClient struct {
	appID     string
	appSecret string
	baseURL   string
	handler   WSEventHandler

	conn         *websocket.Conn
	connMu       sync.Mutex
	serviceID    int32
	pingInterval time.Duration
	reconnectMax int // -1 = infinite

	stopCh  chan struct{}
	stopped bool
	mu      sync.Mutex

	// Fragment buffer: messageID → fragments
	fragments   map[string]*fragmentBuffer
	fragmentsMu sync.Mutex
}

type fragmentBuffer struct {
	total    int
	received map[int][]byte
	created  time.Time
}

type wsEndpointResp struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		URL          string `json:"URL"`
		ClientConfig struct {
			ReconnectCount    int `json:"ReconnectCount"`
			ReconnectInterval int `json:"ReconnectInterval"`
			ReconnectNonce    int `json:"ReconnectNonce"`
			PingInterval      int `json:"PingInterval"`
		} `json:"ClientConfig"`
	} `json:"data"`
}

// NewWSClient creates a native Lark WebSocket client.
func NewWSClient(appID, appSecret, baseURL string, handler WSEventHandler) *WSClient {
	return &WSClient{
		appID:        appID,
		appSecret:    appSecret,
		baseURL:      baseURL,
		handler:      handler,
		pingInterval: defaultPingInterval,
		reconnectMax: -1, // infinite
		fragments:    make(map[string]*fragmentBuffer),
	}
}

// Start connects and begins receiving events. Blocks until stopped or context cancelled.
func (c *WSClient) Start(ctx context.Context) error {
	c.mu.Lock()
	c.stopCh = make(chan struct{})
	c.stopped = false
	c.mu.Unlock()

	return c.connectAndRun(ctx)
}

// Stop shuts down the WebSocket connection.
func (c *WSClient) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return
	}
	c.stopped = true
	close(c.stopCh)

	c.connMu.Lock()
	if c.conn != nil {
		c.conn.Close()
	}
	c.connMu.Unlock()
}

func (c *WSClient) connectAndRun(ctx context.Context) error {
	for {
		select {
		case <-c.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		wsURL, err := c.getWSEndpoint(ctx)
		if err != nil {
			slog.Error("lark ws: get endpoint failed", "error", err)
			c.waitReconnect()
			continue
		}

		slog.Info("lark ws: connecting", "url_len", len(wsURL))

		conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if err != nil {
			slog.Error("lark ws: dial failed", "error", err)
			c.waitReconnect()
			continue
		}

		c.connMu.Lock()
		c.conn = conn
		c.connMu.Unlock()

		slog.Info("lark ws: connected")

		// Start ping loop
		pingDone := make(chan struct{})
		go c.pingLoop(pingDone)

		// Receive loop (blocking)
		err = c.receiveLoop(ctx)

		close(pingDone)

		c.connMu.Lock()
		if c.conn != nil {
			c.conn.Close()
			c.conn = nil
		}
		c.connMu.Unlock()

		if err != nil {
			slog.Warn("lark ws: disconnected", "error", err)
		}

		// Check if stopped
		select {
		case <-c.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
			c.waitReconnect()
		}
	}
}

func (c *WSClient) getWSEndpoint(ctx context.Context) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"AppID":     c.appID,
		"AppSecret": c.appSecret,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/callback/ws/endpoint", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("ws endpoint request: %w", err)
	}
	defer resp.Body.Close()

	var result wsEndpointResp
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("ws endpoint decode: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("ws endpoint error: code=%d msg=%s", result.Code, result.Msg)
	}

	// Apply config
	cfg := result.Data.ClientConfig
	if cfg.PingInterval > 0 {
		c.pingInterval = time.Duration(cfg.PingInterval) * time.Second
	}
	if cfg.ReconnectCount != 0 {
		c.reconnectMax = cfg.ReconnectCount
	}
	c.serviceID = 0 // will be set from endpoint metadata if available

	return result.Data.URL, nil
}

func (c *WSClient) waitReconnect() {
	jitter := time.Duration(rand.Intn(defaultReconnectNonce*1000)) * time.Millisecond
	wait := defaultReconnectWait + jitter
	slog.Info("lark ws: reconnecting", "wait", wait)

	select {
	case <-time.After(wait):
	case <-c.stopCh:
	}
}

// --- Receive loop ---

func (c *WSClient) receiveLoop(ctx context.Context) error {
	for {
		select {
		case <-c.stopCh:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		c.connMu.Lock()
		conn := c.conn
		c.connMu.Unlock()

		if conn == nil {
			return fmt.Errorf("connection closed")
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		frame, err := unmarshalFrame(message)
		if err != nil {
			slog.Debug("lark ws: unmarshal frame failed", "error", err)
			continue
		}

		c.handleFrame(ctx, frame)
	}
}

func (c *WSClient) handleFrame(ctx context.Context, f *wsFrame) {
	headers := f.headerMap()
	frameType := headers["type"]

	switch {
	case f.Method == frameTypeControl && frameType == "pong":
		// Pong — optionally update config from payload
		if len(f.Payload) > 0 {
			var cfg struct {
				PingInterval int `json:"PingInterval"`
			}
			if json.Unmarshal(f.Payload, &cfg) == nil && cfg.PingInterval > 0 {
				c.pingInterval = time.Duration(cfg.PingInterval) * time.Second
			}
		}

	case f.Method == frameTypeData:
		msgID := headers["message_id"]
		sumStr := headers["sum"]
		seqStr := headers["seq"]

		sum, _ := strconv.Atoi(sumStr)
		seq, _ := strconv.Atoi(seqStr)

		payload := f.Payload

		// Fragment reassembly
		if sum > 1 {
			payload = c.reassemble(msgID, sum, seq, payload)
			if payload == nil {
				return // waiting for more fragments
			}
		}

		// Dispatch event
		if c.handler != nil {
			if err := c.handler.HandleEvent(ctx, payload); err != nil {
				slog.Debug("lark ws: event handler error", "error", err)
			}
		}

		// Send response
		c.sendResponse(f, headers)
	}
}

func (c *WSClient) sendResponse(original *wsFrame, headers map[string]string) {
	respHeaders := make([]wsHeader, 0, len(original.Headers)+1)
	for _, h := range original.Headers {
		respHeaders = append(respHeaders, h)
	}
	respHeaders = append(respHeaders, wsHeader{Key: "biz_rt", Value: "0"})

	respPayload, _ := json.Marshal(map[string]interface{}{
		"code": 0,
		"msg":  "success",
	})

	resp := &wsFrame{
		Method:  frameTypeData,
		Service: original.Service,
		Headers: respHeaders,
		Payload: respPayload,
	}

	data := marshalFrame(resp)

	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()

	if conn != nil {
		conn.WriteMessage(websocket.BinaryMessage, data)
	}
}

// --- Ping loop ---

func (c *WSClient) pingLoop(done chan struct{}) {
	ticker := time.NewTicker(c.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case <-c.stopCh:
			return
		case <-ticker.C:
			c.sendPing()
			ticker.Reset(c.pingInterval)
		}
	}
}

func (c *WSClient) sendPing() {
	f := &wsFrame{
		Method:  frameTypeControl,
		Service: c.serviceID,
		Headers: []wsHeader{{Key: "type", Value: "ping"}},
	}
	data := marshalFrame(f)

	c.connMu.Lock()
	conn := c.conn
	c.connMu.Unlock()

	if conn != nil {
		if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
			slog.Debug("lark ws: ping failed", "error", err)
		}
	}
}

// --- Fragment reassembly ---

func (c *WSClient) reassemble(msgID string, total, seq int, data []byte) []byte {
	c.fragmentsMu.Lock()
	defer c.fragmentsMu.Unlock()

	buf, ok := c.fragments[msgID]
	if !ok {
		buf = &fragmentBuffer{
			total:    total,
			received: make(map[int][]byte),
			created:  time.Now(),
		}
		c.fragments[msgID] = buf

		// Auto-cleanup after TTL
		go func() {
			time.Sleep(fragmentBufferTTL)
			c.fragmentsMu.Lock()
			delete(c.fragments, msgID)
			c.fragmentsMu.Unlock()
		}()
	}

	buf.received[seq] = data

	if len(buf.received) < buf.total {
		return nil // still waiting
	}

	// Assemble in order
	var result []byte
	for i := 0; i < buf.total; i++ {
		result = append(result, buf.received[i]...)
	}

	delete(c.fragments, msgID)
	return result
}

// --- Minimal protobuf wire format ---
// Implements just enough protobuf encoding/decoding for the Frame message.
// No external protobuf library needed.

type wsHeader struct {
	Key   string
	Value string
}

type wsFrame struct {
	SeqID           uint64
	LogID           uint64
	Service         int32
	Method          int32
	Headers         []wsHeader
	PayloadEncoding string
	PayloadType     string
	Payload         []byte
	LogIDNew        string
}

func (f *wsFrame) headerMap() map[string]string {
	m := make(map[string]string, len(f.Headers))
	for _, h := range f.Headers {
		m[h.Key] = h.Value
	}
	return m
}

// marshalFrame encodes a wsFrame to protobuf wire format.
func marshalFrame(f *wsFrame) []byte {
	var buf bytes.Buffer

	if f.SeqID != 0 {
		pbWriteVarintField(&buf, 1, f.SeqID)
	}
	if f.LogID != 0 {
		pbWriteVarintField(&buf, 2, f.LogID)
	}
	if f.Service != 0 {
		pbWriteVarintField(&buf, 3, uint64(f.Service))
	}
	if f.Method != 0 {
		pbWriteVarintField(&buf, 4, uint64(f.Method))
	}
	for _, h := range f.Headers {
		// Embedded message: Header { key=1, value=2 }
		var hbuf bytes.Buffer
		pbWriteBytesField(&hbuf, 1, []byte(h.Key))
		pbWriteBytesField(&hbuf, 2, []byte(h.Value))
		pbWriteBytesField(&buf, 5, hbuf.Bytes())
	}
	if f.PayloadEncoding != "" {
		pbWriteBytesField(&buf, 6, []byte(f.PayloadEncoding))
	}
	if f.PayloadType != "" {
		pbWriteBytesField(&buf, 7, []byte(f.PayloadType))
	}
	if len(f.Payload) > 0 {
		pbWriteBytesField(&buf, 8, f.Payload)
	}
	if f.LogIDNew != "" {
		pbWriteBytesField(&buf, 9, []byte(f.LogIDNew))
	}

	return buf.Bytes()
}

// unmarshalFrame decodes a protobuf wire-format message into wsFrame.
func unmarshalFrame(data []byte) (*wsFrame, error) {
	f := &wsFrame{}
	r := bytes.NewReader(data)

	for r.Len() > 0 {
		tag, err := binary.ReadUvarint(r)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("read tag: %w", err)
		}

		fieldNum := tag >> 3
		wireType := tag & 0x7

		switch wireType {
		case 0: // varint
			val, err := binary.ReadUvarint(r)
			if err != nil {
				return nil, fmt.Errorf("read varint field %d: %w", fieldNum, err)
			}
			switch fieldNum {
			case 1:
				f.SeqID = val
			case 2:
				f.LogID = val
			case 3:
				f.Service = int32(val)
			case 4:
				f.Method = int32(val)
			}

		case 2: // length-delimited
			length, err := binary.ReadUvarint(r)
			if err != nil {
				return nil, fmt.Errorf("read length field %d: %w", fieldNum, err)
			}
			buf := make([]byte, length)
			if _, err := io.ReadFull(r, buf); err != nil {
				return nil, fmt.Errorf("read bytes field %d: %w", fieldNum, err)
			}
			switch fieldNum {
			case 5: // Header (embedded message)
				h, err := unmarshalHeader(buf)
				if err == nil {
					f.Headers = append(f.Headers, h)
				}
			case 6:
				f.PayloadEncoding = string(buf)
			case 7:
				f.PayloadType = string(buf)
			case 8:
				f.Payload = buf
			case 9:
				f.LogIDNew = string(buf)
			}

		default:
			return nil, fmt.Errorf("unsupported wire type %d for field %d", wireType, fieldNum)
		}
	}

	return f, nil
}

func unmarshalHeader(data []byte) (wsHeader, error) {
	var h wsHeader
	r := bytes.NewReader(data)

	for r.Len() > 0 {
		tag, err := binary.ReadUvarint(r)
		if err != nil {
			break
		}
		fieldNum := tag >> 3
		wireType := tag & 0x7

		if wireType != 2 {
			return h, fmt.Errorf("header: unexpected wire type %d", wireType)
		}

		length, err := binary.ReadUvarint(r)
		if err != nil {
			return h, err
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(r, buf); err != nil {
			return h, err
		}

		switch fieldNum {
		case 1:
			h.Key = string(buf)
		case 2:
			h.Value = string(buf)
		}
	}

	return h, nil
}

// --- Protobuf encoding helpers ---

func pbWriteVarintField(w *bytes.Buffer, fieldNum int, val uint64) {
	tag := uint64(fieldNum<<3 | 0) // wire type 0 = varint
	pbWriteUvarint(w, tag)
	pbWriteUvarint(w, val)
}

func pbWriteBytesField(w *bytes.Buffer, fieldNum int, data []byte) {
	tag := uint64(fieldNum<<3 | 2) // wire type 2 = length-delimited
	pbWriteUvarint(w, tag)
	pbWriteUvarint(w, uint64(len(data)))
	w.Write(data)
}

func pbWriteUvarint(w *bytes.Buffer, val uint64) {
	var buf [10]byte
	n := binary.PutUvarint(buf[:], val)
	w.Write(buf[:n])
}
