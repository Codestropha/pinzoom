package ws

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	TextMessage   = 1
	BinaryMessage = 2
	CloseMessage  = 8
	PingMessage   = 9
	PongMessage   = 10
)

type WebSocket struct {
	conn net.Conn

	// Write fields
	mu            chan struct{} // used as mutex to protect write to conn
	writeBuf      []byte        // frame is constructed in this buffer.
	writeDeadline time.Time
	writer        io.WriteCloser // the current writer returned to the application
	isWriting     bool           // for best-effort concurrent write detection
	writeErrMu    sync.Mutex
	writeErr      error

	// Read fields
	reader  io.ReadCloser // the current reader returned to the application
	readErr error
	br      *bufio.Reader
	// bytes remaining in current frame.
	// set setReadRemaining to safely update this value and prevent overflow
	readRemaining int64
	readFinal     bool  // true the current message has more frames.
	readLength    int64 // Message size.
	readLimit     int64 // Maximum message size.
	readMaskPos   int
	readMaskKey   [4]byte
	messageReader *wsFrameReader // the current low-level reader

	handlePong  func(string) error
	handlePing  func(string) error
	handleClose func(int, string) error

	readDecompress         bool // whether last read frame had RSV1 set
	newDecompressionReader func(io.Reader) io.ReadCloser
}

func NewWebSocket(conn net.Conn, r *http.Request, w http.ResponseWriter) (*WebSocket, error) {
	if err := Upgrade(r, w); err != nil {
		return nil, err
	}
	br := bufio.NewReaderSize(conn, defaultReadBufferSize)
	writeBufferSize := defaultWriteBufferSize + maxFrameHeaderSize
	writeBuf := make([]byte, writeBufferSize)

	mu := make(chan struct{}, 1)
	mu <- struct{}{}
	webSocket := &WebSocket{
		br:        br,
		conn:      conn,
		mu:        mu,
		readFinal: true,
		writeBuf:  writeBuf,
	}
	webSocket.SetPongHandler(nil)
	return webSocket, nil
}

func (ws *WebSocket) Close() error {
	return ws.conn.Close()
}

func (ws *WebSocket) SetWriteDeadline(t time.Time) error {
	ws.writeDeadline = t
	return nil
}

func (ws *WebSocket) SetReadDeadline(t time.Time) error {
	return ws.conn.SetReadDeadline(t)
}

func (ws *WebSocket) SetReadLimit(limit int64) {
	ws.readLimit = limit
}

func (ws *WebSocket) SetPongHandler(handler func(string) error) {
	if handler == nil {
		handler = func(string) error { return nil }
	}
	ws.handlePong = handler
}

func (ws *WebSocket) ReadMessage() (messageType int, p []byte, err error) {
	var r io.Reader
	messageType, r, err = ws.NextReader()
	if err != nil {
		return messageType, nil, err
	}
	p, err = io.ReadAll(r)
	return messageType, p, err
}

// NextReader returns the next data message received from the peer. The
// returned messageType is either TextMessage or BinaryMessage.
//
// There can be at most one open reader on a connection. NextReader discards
// the previous message if the application has not already consumed it.
//
// Applications must break out of the application's read loop when this method
// returns a non-nil error value. Errors returned from this method are
// permanent. Once this method returns a non-nil error, all subsequent calls to
// this method return the same error.
func (ws *WebSocket) NextReader() (messageType int, r io.Reader, err error) {
	// Close previous reader, only relevant for decompression.
	if ws.reader != nil {
		ws.reader.Close()
		ws.reader = nil
	}

	ws.messageReader = nil
	ws.readLength = 0

	for ws.readErr == nil {
		frameType, err := ws.advanceFrame()
		if err != nil {
			ws.readErr = err
			break
		}

		if frameType == TextMessage || frameType == BinaryMessage {
			ws.messageReader = &wsFrameReader{ws}
			ws.reader = ws.messageReader
			if ws.readDecompress {
				ws.reader = ws.newDecompressionReader(ws.reader)
			}
			return frameType, ws.reader, nil
		}
	}

	return noFrame, nil, ws.readErr
}

func (ws *WebSocket) WriteJSON(v interface{}) error {
	w, err := ws.NextWriter(TextMessage)
	if err != nil {
		return err
	}
	err1 := json.NewEncoder(w).Encode(v)
	err2 := w.Close()
	if err1 != nil {
		return err1
	}
	return err2
}

func (ws *WebSocket) NextWriter(messageType int) (io.WriteCloser, error) {
	var mw wsFrameWriter
	if err := ws.beginMessage(&mw, messageType); err != nil {
		return nil, err
	}
	ws.writer = &mw
	return ws.writer, nil
}

func (ws *WebSocket) WriteMessage(messageType int, data []byte) error {
	w, err := ws.NextWriter(messageType)
	if err != nil {
		return err
	}
	if _, err = w.Write(data); err != nil {
		return err
	}
	return w.Close()
}

func (ws *WebSocket) beginMessage(mw *wsFrameWriter, messageType int) error {
	// Close previous writer if not already closed by the application. It's
	// probably better to return an error in this situation, but we cannot
	// change this without breaking existing applications.
	if ws.writer != nil {
		ws.writer.Close()
		ws.writer = nil
	}

	if !isControl(messageType) && !isData(messageType) {
		return errBadWriteOpCode
	}

	ws.writeErrMu.Lock()
	err := ws.writeErr
	ws.writeErrMu.Unlock()
	if err != nil {
		return err
	}

	mw.c = ws
	mw.frameType = messageType
	mw.pos = maxFrameHeaderSize
	return nil
}

func (ws *WebSocket) writeFatal(err error) error {
	ws.writeErrMu.Lock()
	if ws.writeErr == nil {
		ws.writeErr = err
	}
	ws.writeErrMu.Unlock()
	return err
}

func (ws *WebSocket) write(frameType int, deadline time.Time, buf0, buf1 []byte) error {
	<-ws.mu
	defer func() { ws.mu <- struct{}{} }()

	ws.writeErrMu.Lock()
	err := ws.writeErr
	ws.writeErrMu.Unlock()
	if err != nil {
		return err
	}

	if err := ws.conn.SetWriteDeadline(deadline); err != nil {
		return ws.writeFatal(err)
	}
	if len(buf1) == 0 {
		_, err = ws.conn.Write(buf0)
	} else {
		err = ws.writeBufs(buf0, buf1)
	}
	if err != nil {
		return ws.writeFatal(err)
	}
	if frameType == CloseMessage {
		_ = ws.writeFatal(errCloseSent)
	}
	return nil
}

func (ws *WebSocket) writeBufs(bufs ...[]byte) error {
	b := net.Buffers(bufs)
	_, err := b.WriteTo(ws.conn)
	return err
}

// WriteControl writes a control message with the given deadline. The allowed
// message types are CloseMessage, PingMessage and PongMessage.
func (ws *WebSocket) WriteControl(messageType int, data []byte, deadline time.Time) error {
	if !isControl(messageType) {
		return errBadWriteOpCode
	}
	if len(data) > maxControlFramePayloadSize {
		return errInvalidControlFrame
	}

	b0 := byte(messageType) | finalBit
	b1 := byte(len(data))

	buf := make([]byte, 0, maxFrameHeaderSize+maxControlFramePayloadSize)
	buf = append(buf, b0, b1)
	buf = append(buf, data...)

	if deadline.IsZero() {
		// No timeout for zero time.
		<-ws.mu
	} else {
		d := time.Until(deadline)
		if d < 0 {
			return errWriteTimeout
		}
		select {
		case <-ws.mu:
		default:
			timer := time.NewTimer(d)
			select {
			case <-ws.mu:
				timer.Stop()
			case <-timer.C:
				return errWriteTimeout
			}
		}
	}

	defer func() { ws.mu <- struct{}{} }()

	ws.writeErrMu.Lock()
	err := ws.writeErr
	ws.writeErrMu.Unlock()
	if err != nil {
		return err
	}

	if err := ws.conn.SetWriteDeadline(deadline); err != nil {
		return ws.writeFatal(err)
	}
	if _, err = ws.conn.Write(buf); err != nil {
		return ws.writeFatal(err)
	}
	if messageType == CloseMessage {
		_ = ws.writeFatal(errCloseSent)
	}
	return err
}

func (ws *WebSocket) setReadRemaining(n int64) error {
	if n < 0 {
		return errReadLimit
	}

	ws.readRemaining = n
	return nil
}

func (ws *WebSocket) read(n int) ([]byte, error) {
	p, err := ws.br.Peek(n)
	if err == io.EOF {
		err = errUnexpectedEOF
	}
	// Discard is guaranteed to succeed because the number of bytes to discard
	// is less than or equal to the number of bytes buffered.
	_, _ = ws.br.Discard(len(p))
	return p, err
}

func (ws *WebSocket) advanceFrame() (int, error) {
	// 1. Skip remainder of previous frame.

	if ws.readRemaining > 0 {
		if _, err := io.CopyN(io.Discard, ws.br, ws.readRemaining); err != nil {
			return noFrame, err
		}
	}

	// 2. Read and parse first two bytes of frame header.
	// To aid debugging, collect and report all errors in the first two bytes
	// of the header.

	var errors []string

	p, err := ws.read(2)
	if err != nil {
		return noFrame, err
	}

	frameType := int(p[0] & 0xf)
	final := p[0]&finalBit != 0
	rsv1 := p[0]&rsv1Bit != 0
	rsv2 := p[0]&rsv2Bit != 0
	rsv3 := p[0]&rsv3Bit != 0
	mask := p[1]&maskBit != 0
	_ = ws.setReadRemaining(int64(p[1] & 0x7f)) // will not fail because argument is >= 0

	ws.readDecompress = false
	if rsv1 {
		if ws.newDecompressionReader != nil {
			ws.readDecompress = true
		} else {
			errors = append(errors, "RSV1 set")
		}
	}

	if rsv2 {
		errors = append(errors, "RSV2 set")
	}

	if rsv3 {
		errors = append(errors, "RSV3 set")
	}

	switch frameType {
	case CloseMessage, PingMessage, PongMessage:
		if ws.readRemaining > maxControlFramePayloadSize {
			errors = append(errors, "len > 125 for control")
		}
		if !final {
			errors = append(errors, "FIN not set on control")
		}
	case TextMessage, BinaryMessage:
		if !ws.readFinal {
			errors = append(errors, "data before FIN")
		}
		ws.readFinal = final
	case continuationFrame:
		if ws.readFinal {
			errors = append(errors, "continuation after FIN")
		}
		ws.readFinal = final
	default:
		errors = append(errors, "bad opcode "+strconv.Itoa(frameType))
	}
	if !mask {
		errors = append(errors, "bad MASK")
	}

	if len(errors) > 0 {
		return noFrame, ws.handleProtocolError(strings.Join(errors, ", "))
	}

	// 3. Read and parse frame length as per
	// https://tools.ietf.org/html/rfc6455#section-5.2
	//
	// The length of the "Payload data", in bytes: if 0-125, that is the payload
	// length.
	// - If 126, the following 2 bytes interpreted as a 16-bit unsigned
	// integer are the payload length.
	// - If 127, the following 8 bytes interpreted as
	// a 64-bit unsigned integer (the most significant bit MUST be 0) are the
	// payload length. Multibyte length quantities are expressed in network byte
	// order.

	switch ws.readRemaining {
	case 126:
		p, err := ws.read(2)
		if err != nil {
			return noFrame, err
		}

		if err := ws.setReadRemaining(int64(binary.BigEndian.Uint16(p))); err != nil {
			return noFrame, err
		}
	case 127:
		p, err := ws.read(8)
		if err != nil {
			return noFrame, err
		}

		if err := ws.setReadRemaining(int64(binary.BigEndian.Uint64(p))); err != nil {
			return noFrame, err
		}
	}

	// 4. Handle frame masking.

	if mask {
		ws.readMaskPos = 0
		p, err := ws.read(len(ws.readMaskKey))
		if err != nil {
			return noFrame, err
		}
		copy(ws.readMaskKey[:], p)
	}

	// 5. For text and binary messages, enforce read limit and return.

	if frameType == continuationFrame || frameType == TextMessage || frameType == BinaryMessage {

		ws.readLength += ws.readRemaining
		// Don't allow readLength to overflow in the presence of a large readRemaining
		// counter.
		if ws.readLength < 0 {
			return noFrame, errReadLimit
		}

		if ws.readLimit > 0 && ws.readLength > ws.readLimit {
			// Make a best effort to send a close message describing the problem.
			_ = ws.WriteControl(CloseMessage, FormatCloseMessage(CloseMessageTooBig, ""), time.Now().Add(writeWait))
			return noFrame, errReadLimit
		}

		return frameType, nil
	}

	// 6. Read control frame payload.

	var payload []byte
	if ws.readRemaining > 0 {
		payload, err = ws.read(int(ws.readRemaining))
		_ = ws.setReadRemaining(0) // will not fail because argument is >= 0
		if err != nil {
			return noFrame, err
		}
		maskBytes(ws.readMaskKey, 0, payload)
	}

	// 7. Process control frame payload.

	switch frameType {
	case PongMessage:
		if err := ws.handlePong(string(payload)); err != nil {
			return noFrame, err
		}
	case PingMessage:
		if err := ws.handlePing(string(payload)); err != nil {
			return noFrame, err
		}
	case CloseMessage:
		closeCode := CloseNoStatusReceived
		closeText := ""
		if len(payload) >= 2 {
			closeCode = int(binary.BigEndian.Uint16(payload))
			if !isValidReceivedCloseCode(closeCode) {
				return noFrame, ws.handleProtocolError("bad close code " + strconv.Itoa(closeCode))
			}
			closeText = string(payload[2:])
			if !utf8.ValidString(closeText) {
				return noFrame, ws.handleProtocolError("invalid utf8 payload in close frame")
			}
		}
		if err := ws.handleClose(closeCode, closeText); err != nil {
			return noFrame, err
		}
		return noFrame, &CloseError{Code: closeCode, Text: closeText}
	}

	return frameType, nil
}

func (ws *WebSocket) handleProtocolError(message string) error {
	data := FormatCloseMessage(CloseProtocolError, message)
	if len(data) > maxControlFramePayloadSize {
		data = data[:maxControlFramePayloadSize]
	}
	// Make a best effor to send a close message describing the problem.
	_ = ws.WriteControl(CloseMessage, data, time.Now().Add(writeWait))
	return errors.New("websocket: " + message)
}
