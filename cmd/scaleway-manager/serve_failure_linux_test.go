package main

import (
	"net"
	"net/http"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// listeningFD finds the listening socket of this process bound to 127.0.0.1:port.
func listeningFD(t *testing.T, port int) int {
	t.Helper()
	for fd := 3; fd < 4096; fd++ {
		sa, err := syscall.Getsockname(fd)
		if err != nil {
			continue
		}
		in, ok := sa.(*syscall.SockaddrInet4)
		if !ok || in.Port != port || in.Addr != [4]byte{127, 0, 0, 1} {
			continue
		}
		if v, err := syscall.GetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_ACCEPTCONN); err != nil || v != 1 {
			continue
		}
		return fd
	}
	t.Fatalf("no listening socket on 127.0.0.1:%d", port)
	return -1
}

// Matrix row Serve failure: Serve returns an error other than http.ErrServerClosed
// before any signal. The test breaks the listener run uses from outside: shutdown(2)
// on a listening socket makes accept fail with EINVAL, a non-temporary error, so
// Serve returns it. This holds whatever way run obtains its listener.
func TestServeFailure(t *testing.T) {
	r := start(t, http.NotFoundHandler(), 2*time.Second)

	_, portStr, err := net.SplitHostPort(r.addr)
	if err != nil {
		t.Fatalf("addr %q: %v", r.addr, err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		t.Fatalf("addr port %q: %v", portStr, err)
	}
	if err := syscall.Shutdown(listeningFD(t, port), syscall.SHUT_RD); err != nil {
		t.Fatalf("shutdown of the listening socket: %v", err)
	}

	if code := r.wait(t, 5*time.Second); code != 1 {
		t.Errorf("exit code %d, want 1", code)
	}
	ls := lines(t, r.out)
	errs := withLevel(ls, "ERROR")
	if len(errs) != 1 || errs[0].str("msg") != "serve failed" || errs[0].str("error") == "" {
		t.Errorf("want exactly one ERROR line \"serve failed\" with a field error; stdout:\n%s", r.out.String())
	}
	if n := len(withMsg(ls, "stopped")); n != 0 {
		t.Errorf("%d stopped line(s), want 0", n)
	}
	if n := len(withMsg(ls, "listening")); n != 1 {
		t.Errorf("%d listening line(s), want 1", n)
	}
}
