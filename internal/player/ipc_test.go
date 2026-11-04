package player

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

func startFakeServer(t *testing.T, handler func(*bufio.Reader, net.Conn)) (net.Conn, net.Conn) {
	t.Helper()
	client, server := net.Pipe()
	go handler(bufio.NewReader(server), server)
	return client, server
}

func TestIPCCommandRoundTrip(t *testing.T) {
	clientConn, serverConn := startFakeServer(t, func(reader *bufio.Reader, conn net.Conn) {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		var request struct {
			Command   []any `json:"command"`
			RequestID int   `json:"request_id"`
		}
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if request.RequestID != 1 {
			t.Errorf("request id = %d, want 1", request.RequestID)
		}
		if len(request.Command) != 2 || request.Command[0] != "get_property" {
			t.Errorf("command = %#v", request.Command)
		}
		fmt.Fprintf(conn, `{"request_id":%d,"error":"success","data":55}`+"\n", request.RequestID)
	})
	defer clientConn.Close()
	defer serverConn.Close()
	client := newIPC(clientConn)
	defer client.Close()

	raw, err := client.Call(context.Background(), "get_property", "volume")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if string(raw) != "55" {
		t.Fatalf("data = %s", raw)
	}
}

func TestIPCCommandError(t *testing.T) {
	clientConn, serverConn := startFakeServer(t, func(reader *bufio.Reader, conn net.Conn) {
		line, _ := reader.ReadString('\n')
		var request struct {
			RequestID int `json:"request_id"`
		}
		_ = json.Unmarshal([]byte(line), &request)
		fmt.Fprintf(conn, `{"request_id":%d,"error":"property unavailable"}`+"\n", request.RequestID)
	})
	defer clientConn.Close()
	defer serverConn.Close()
	client := newIPC(clientConn)
	defer client.Close()

	_, err := client.Call(context.Background(), "get_property", "time-pos")
	if err == nil || !strings.Contains(err.Error(), "property unavailable") {
		t.Fatalf("error = %v", err)
	}
}

func TestIPCEvents(t *testing.T) {
	clientConn, serverConn := startFakeServer(t, func(reader *bufio.Reader, conn net.Conn) {
		fmt.Fprint(conn, `{"event":"file-loaded"}`+"\n")
		fmt.Fprint(conn, `{"event":"end-file","reason":"error"}`+"\n")
		_, _ = reader.ReadString('\n')
	})
	defer clientConn.Close()
	defer serverConn.Close()
	client := newIPC(clientConn)
	defer client.Close()

	select {
	case event := <-client.Events():
		if event.Kind != EventFileLoaded {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for file-loaded")
	}
	select {
	case event := <-client.Events():
		if event.Kind != EventEnded || event.Reason != "error" || event.Err == nil {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for end-file")
	}
}

func TestIPCTimeout(t *testing.T) {
	stop := make(chan struct{})
	clientConn, serverConn := startFakeServer(t, func(reader *bufio.Reader, conn net.Conn) {
		_, _ = reader.ReadString('\n')
		<-stop
	})
	defer close(stop)
	defer serverConn.Close()
	client := newIPC(clientConn)
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()
	_, err := client.Call(ctx, "get_property", "volume")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("error = %v, want deadline exceeded", err)
	}
}

func TestIPCCloseUnblocksCall(t *testing.T) {
	stop := make(chan struct{})
	clientConn, serverConn := startFakeServer(t, func(reader *bufio.Reader, conn net.Conn) {
		_, _ = reader.ReadString('\n')
		<-stop
	})
	defer close(stop)
	defer serverConn.Close()
	client := newIPC(clientConn)

	done := make(chan error, 1)
	go func() {
		_, err := client.Call(context.Background(), "get_property", "volume")
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	_ = client.Close()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("expected an error after close")
		}
	case <-time.After(time.Second):
		t.Fatal("Call did not unblock after Close")
	}
}

func TestIPCRequestIDsIncrease(t *testing.T) {
	ids := make(chan int, 2)
	clientConn, serverConn := startFakeServer(t, func(reader *bufio.Reader, conn net.Conn) {
		for i := 0; i < 2; i++ {
			line, err := reader.ReadString('\n')
			if err != nil {
				return
			}
			var request struct {
				RequestID int `json:"request_id"`
			}
			_ = json.Unmarshal([]byte(line), &request)
			ids <- request.RequestID
			fmt.Fprintf(conn, `{"request_id":%d,"error":"success"}`+"\n", request.RequestID)
		}
	})
	defer clientConn.Close()
	defer serverConn.Close()
	client := newIPC(clientConn)
	defer client.Close()

	if _, err := client.Call(context.Background(), "stop"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Call(context.Background(), "stop"); err != nil {
		t.Fatal(err)
	}
	if first, second := <-ids, <-ids; second != first+1 {
		t.Fatalf("request ids = %d, %d", first, second)
	}
}
