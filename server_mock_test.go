package adb

import (
	"bytes"
	"io"
	"net"
	"strings"
	"time"

	"DomaphoneS-Next/backend/goadb/wire"
)

// MockServer implements Server, Scanner, and Sender.
type MockServer struct {
	mockConn
	// Each time an operation is performed, if this slice is non-empty, the head element
	// of this slice is returned and removed from the slice. If the head is nil, it is removed
	// but not returned.
	Errs []error

	Status string

	// Messages are returned from read calls in order, each preceded by a length header.
	Messages     []string
	nextMsgIndex int

	// Each message passed to a send call is appended to this slice.
	Requests []string

	// Each time an operation is performed, its name is appended to this slice.
	Trace []string
}

var _ server = &MockServer{
	mockConn: mockConn{
		Buffer: bytes.NewBuffer(nil),
		rbuf:   bytes.NewBuffer(nil),
	},
}

func (s *MockServer) Dial() (wire.IConn, error) {
	s.logMethod("Dial")
	if err := s.getNextErrToReturn(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *MockServer) Start() error {
	s.logMethod("Start")
	return nil
}

func (s *MockServer) RoundTripSingleResponse(req []byte) (resp []byte, err error) {
	if err = s.SendMessage(req); err != nil {
		return nil, err
	}

	if _, err = s.ReadStatus(string(req)); err != nil {
		return nil, err
	}

	return s.ReadMessage()
}

func (s *MockServer) ReadStatus(req string) (string, error) {
	s.logMethod("ReadStatus")
	if err := s.getNextErrToReturn(); err != nil {
		return "", err
	}
	return s.Status, nil
}

func (s *MockServer) ReadMessage() ([]byte, error) {
	s.logMethod("ReadMessage")
	if err := s.getNextErrToReturn(); err != nil {
		return nil, err
	}
	if s.nextMsgIndex >= len(s.Messages) {
		return nil, io.EOF
	}

	s.nextMsgIndex++
	return []byte(s.Messages[s.nextMsgIndex-1]), nil
}

func (s *MockServer) ReadUntilEof() ([]byte, error) {
	s.logMethod("ReadUntilEof")
	if err := s.getNextErrToReturn(); err != nil {
		return nil, err
	}

	var data []string
	for ; s.nextMsgIndex < len(s.Messages); s.nextMsgIndex++ {
		data = append(data, s.Messages[s.nextMsgIndex])
	}
	return []byte(strings.Join(data, "")), nil
}

func (s *MockServer) SendMessage(msg []byte) error {
	s.logMethod("SendMessage")
	if err := s.getNextErrToReturn(); err != nil {
		return err
	}
	s.Requests = append(s.Requests, string(msg))
	return nil
}

func (s *MockServer) Close() error {
	s.logMethod("Close")
	if err := s.getNextErrToReturn(); err != nil {
		return err
	}
	return nil
}

func (s *MockServer) getNextErrToReturn() (err error) {
	if len(s.Errs) > 0 {
		err = s.Errs[0]
		s.Errs = s.Errs[1:]
	}
	return
}

func (s *MockServer) logMethod(name string) {
	s.Trace = append(s.Trace, name)
}

// mockConn is a wrapper around a bytes.Buffer that implements net.Conn
type mockConn struct {
	// io.ReadWriter
	*bytes.Buffer // write buffer
	rbuf          *bytes.Buffer
}

func (b *mockConn) Read(p []byte) (n int, err error) {
	if b.rbuf != nil {
		return b.rbuf.Read(p)
	}

	return b.Buffer.Read(p)
}

func (b *mockConn) Write(p []byte) (n int, err error) {
	return b.Buffer.Write(p)
}

func (b *mockConn) Close() error {
	// No-op.
	return nil
}

func (b *mockConn) LocalAddr() net.Addr {
	return nil
}
func (b *mockConn) RemoteAddr() net.Addr {
	return nil
}

func (b *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (b *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (b *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}
