package wire

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestReadStatusOkay(t *testing.T) {
	s := newEofReader("OKAYd")
	status, err := readStatusFailureAsError(s, nil, "")
	assert.NoError(t, err)
	assert.False(t, isFailureStatus(status))
	assertNotEof(t, s)
}

func TestReadIncompleteStatus(t *testing.T) {
	s := newEofReader("oka")
	_, err := readStatusFailureAsError(s, nil, "")
	assert.Contains(t, err.Error(), "error reading status for ")
	assert.Equal(t, errors.Unwrap(err), errIncompleteMessage("", 3, 4))
	assertEof(t, s)
}

func TestReadFailureIncompleteStatus(t *testing.T) {
	s := newEofReader("FAIL")
	_, err := readStatusFailureAsError(s, nil, "req")
	assert.Contains(t, err.Error(), "server returned error for req, but couldn't read the error message")
	assertEof(t, s)
}

func TestReadFailureEmptyStatus(t *testing.T) {
	s := newEofReader("FAIL0000")
	_, err := readStatusFailureAsError(s, nil, "")
	assert.EqualError(t, err, "AdbError: request , server error: ")
	assertEof(t, s)
}

func TestReadFailureStatus(t *testing.T) {
	s := newEofReader("FAIL0004fail")
	_, err := readStatusFailureAsError(s, nil, "")
	assert.EqualError(t, err, "AdbError: request , server error: fail")
	assertEof(t, s)
}

func TestReadMessage(t *testing.T) {
	s := newEofReader("0005hello")
	msg, err := readMessage(s, nil)
	assert.NoError(t, err)
	assert.Len(t, msg, 5)
	assert.Equal(t, "hello", string(msg))
	assertEof(t, s)
}

func TestReadMessageWithExtraData(t *testing.T) {
	s := newEofReader("0005hellothere")
	msg, err := readMessage(s, nil)
	assert.NoError(t, err)
	assert.Len(t, msg, 5)
	assert.Equal(t, "hello", string(msg))
	assertNotEof(t, s)
}

func TestReadLongerMessage(t *testing.T) {
	s := newEofReader("001b192.168.56.101:5555	device\n")
	msg, err := readMessage(s, nil)
	assert.NoError(t, err)
	assert.Len(t, msg, 27)
	assert.Equal(t, "192.168.56.101:5555	device\n", string(msg))
	assertEof(t, s)
}

func TestReadEmptyMessage(t *testing.T) {
	s := newEofReader("0000")
	msg, err := readMessage(s, nil)
	assert.NoError(t, err)
	assert.Equal(t, "", string(msg))
	assertEof(t, s)
}

func TestReadIncompleteMessage(t *testing.T) {
	s := newEofReader("0005hel")
	msg, err := readMessage(s, nil)
	assert.Error(t, err)
	assert.Equal(t, errIncompleteMessage("message data", 3, 5), err)
	assert.Equal(t, "hel", string(msg))
	assertEof(t, s)
}

func TestReadLength(t *testing.T) {
	s := newEofReader("000a")
	l, err := readHexLength(s, make([]byte, 4))
	assert.NoError(t, err)
	assert.Equal(t, 10, l)
	assertEof(t, s)
}

func TestReadLengthIncompleteLength(t *testing.T) {
	s := newEofReader("aaa")
	_, err := readHexLength(s, make([]byte, 4))
	assert.Equal(t, errIncompleteMessage("length", 3, 4), err)
	assertEof(t, s)
}

func assertEof(t *testing.T, r io.Reader) {
	msg, err := readMessage(r, nil)
	assert.True(t, errors.Is(err, ErrConnectionReset))
	assert.Nil(t, msg)
}

func assertNotEof(t *testing.T, r io.Reader) {
	n, err := r.Read(make([]byte, 1))
	assert.Equal(t, 1, n)
	assert.NoError(t, err)
}

// newEofBuffer returns a bytes.Buffer of str that returns an EOF error
// at the end of input, instead of just returning 0 bytes read.
func newEofReader(str string) io.ReadCloser {
	limitReader := io.LimitReader(bytes.NewBufferString(str), int64(len(str)))
	bufReader := bufio.NewReader(limitReader)
	return io.NopCloser(bufReader)
}

func TestWriteMessage(t *testing.T) {
	s, b := newTestSender()
	err := s.SendMessage([]byte("hello"))
	assert.NoError(t, err)
	assert.Equal(t, "0005hello", b.String())
}

func TestWriteEmptyMessage(t *testing.T) {
	s, b := newTestSender()
	err := s.SendMessage([]byte(""))
	assert.NoError(t, err)
	assert.Equal(t, "0000", b.String())
}

func newTestSender() (Sender, *mockConn) {
	var buf bytes.Buffer
	w := makeMockConnBuf(&buf)
	return NewConn(w), w
}

// mockConn is a wrapper around a bytes.Buffer that implements io.Closer.
type mockConn struct {
	*bytes.Buffer
}

func makeMockConnStr(str string) *mockConn {
	w := &mockConn{
		Buffer: bytes.NewBufferString(str),
	}
	return w
}

func makeMockConnBuf(buf *bytes.Buffer) *mockConn {
	w := &mockConn{
		Buffer: buf,
	}
	return w
}

func makeMockConnBytes(b []byte) *mockConn {
	w := &mockConn{
		Buffer: bytes.NewBuffer(b),
	}
	return w
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
