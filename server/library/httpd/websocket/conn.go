package websocket

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
)

// WebSocket фреймы (определены в RFC 6455, раздел 11.8)
const (
	TextMessage   = 1
	BinaryMessage = 2
	CloseMessage  = 8
	PingMessage   = 9
	PongMessage   = 10
)

// Conn представляет WebSocket соединение
type Conn struct {
	conn   net.Conn
	reader *bufio.Reader
	writer *bufio.Writer
}

// Close закрывает WebSocket соединение
func (c *Conn) Close() error {
	return c.conn.Close()
}

// ReadMessage читает WebSocket сообщение
func (c *Conn) ReadMessage() (messageType int, p []byte, err error) {
	// Здесь будет логика для чтения WebSocket фреймов
	header := make([]byte, 2)
	_, err = io.ReadFull(c.reader, header)
	if err != nil {
		return 0, nil, err
	}

	// Определяем тип сообщения
	messageType = int(header[0] & 0xF)
	// Читаем оставшуюся часть фрейма, используя длину, закодированную в header

	return messageType, nil, nil // Будет возвращать данные сообщения
}

// WriteCloser представляет writer для WebSocket сообщений
type messageWriter struct {
	conn        *Conn
	messageType int
	buffer      *bytes.Buffer
	closed      bool
	mu          sync.Mutex
}

// Write записывает данные в буфер
func (w *messageWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return 0, errors.New("writer already closed")
	}
	return w.buffer.Write(p)
}

// Close завершает запись сообщения и отправляет его
func (w *messageWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.closed {
		return errors.New("writer already closed")
	}

	// Отправляем фрейм с данными через соединение
	err := w.conn.writeFrame(w.messageType, w.buffer.Bytes())
	w.closed = true
	return err
}

// NextWriter возвращает WriteCloser для записи WebSocket сообщений
func (c *Conn) NextWriter(messageType int) (io.WriteCloser, error) {
	// Создаем messageWriter с буфером для записи
	return &messageWriter{
		conn:        c,
		messageType: messageType,
		buffer:      &bytes.Buffer{},
	}, nil
}

// writeFrame записывает фрейм в соединение
func (c *Conn) writeFrame(messageType int, payload []byte) error {
	// Собираем заголовок WebSocket фрейма
	header := make([]byte, 2)
	header[0] = byte(0x80 | messageType) // FIN + тип сообщения

	// Определяем длину полезной нагрузки
	payloadLen := len(payload)
	if payloadLen <= 125 {
		header[1] = byte(payloadLen)
	} else if payloadLen < 65536 {
		header[1] = 126
		extendedLen := make([]byte, 2)
		binary.BigEndian.PutUint16(extendedLen, uint16(payloadLen))
		header = append(header, extendedLen...)
	} else {
		header[1] = 127
		extendedLen := make([]byte, 8)
		binary.BigEndian.PutUint64(extendedLen, uint64(payloadLen))
		header = append(header, extendedLen...)
	}

	// Записываем заголовок и полезную нагрузку в соединение
	if _, err := c.writer.Write(header); err != nil {
		return err
	}
	if _, err := c.writer.Write(payload); err != nil {
		return err
	}

	// Завершаем отправку данных
	return c.writer.Flush()
}

// Upgrader обновляет HTTP соединение до WebSocket
type Upgrader struct {
	ReadBufferSize    int
	WriteBufferSize   int
	EnableCompression bool
}

// Upgrade обновляет HTTP соединение до WebSocket
func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request, responseHeader http.Header) (*Conn, error) {
	// Проверяем заголовки
	if !isWebSocketRequest(r) {
		return nil, errors.New("not a websocket handshake request")
	}

	// Создаем ответ для WebSocket
	secWebSocketKey := r.Header.Get("Sec-WebSocket-Key")
	if secWebSocketKey == "" {
		return nil, errors.New("missing Sec-WebSocket-Key")
	}

	secWebSocketAccept := computeAcceptKey(secWebSocketKey)
	w.Header().Set("Upgrade", "websocket")
	w.Header().Set("Connection", "Upgrade")
	w.Header().Set("Sec-WebSocket-Accept", secWebSocketAccept)

	if len(responseHeader) > 0 {
		for k, v := range responseHeader {
			w.Header()[k] = v
		}
	}

	w.WriteHeader(http.StatusSwitchingProtocols)

	// Получаем соединение из http.ResponseWriter (используем для обмена сообщениями)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("response does not support hijacking")
	}

	conn, _, err := hijacker.Hijack()
	if err != nil {
		return nil, err
	}

	// Создаем WebSocket соединение
	return &Conn{
		conn:   conn,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
	}, nil
}

// Проверяет, является ли запрос WebSocket handshake
func isWebSocketRequest(r *http.Request) bool {
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	connection := strings.ToLower(r.Header.Get("Connection"))
	return upgrade == "websocket" && strings.Contains(connection, "upgrade")
}

// Вычисляет значение Sec-WebSocket-Accept
func computeAcceptKey(secWebSocketKey string) string {
	magicString := "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"
	hash := sha1.New()
	hash.Write([]byte(secWebSocketKey + magicString))
	return base64.StdEncoding.EncodeToString(hash.Sum(nil))
}
