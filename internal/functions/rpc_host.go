package functions

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// rpcHost implements HostAPI by forwarding calls to the parent process.
type rpcHost struct {
	conn net.Conn
	ctx  context.Context
	mu   sync.Mutex
	seq  atomic.Int64
}

func newRPCHost(conn net.Conn, ctx context.Context) HostAPI {
	if ctx == nil {
		ctx = context.Background()
	}
	h := &rpcHost{conn: conn, ctx: ctx}
	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()
	return h
}

func (h *rpcHost) call(method string, params interface{}) ([]byte, error) {
	if err := h.ctx.Err(); err != nil {
		return nil, err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if err := h.ctx.Err(); err != nil {
		return nil, err
	}
	if dl, ok := h.ctx.Deadline(); ok {
		_ = h.conn.SetDeadline(dl)
	} else {
		_ = h.conn.SetDeadline(time.Time{})
	}

	id := int(h.seq.Add(1))
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	msg := RPCMessage{Type: "call", ID: id, Method: method, Params: paramsJSON}
	if err := writeRPC(h.conn, msg); err != nil {
		return nil, err
	}
	resp, err := readRPC(h.conn)
	_ = h.conn.SetDeadline(time.Time{})
	if err != nil {
		if h.ctx.Err() != nil {
			return nil, h.ctx.Err()
		}
		return nil, err
	}
	if resp.Type == "err" {
		return nil, fmt.Errorf("%s", resp.Error)
	}
	return resp.Result, nil
}

func (h *rpcHost) DBGet(collection, id string) (map[string]interface{}, error) {
	raw, err := h.call("db.get", map[string]string{"collection": collection, "id": id})
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *rpcHost) DBCreate(collection string, data map[string]interface{}) (map[string]interface{}, error) {
	raw, err := h.call("db.create", map[string]interface{}{"collection": collection, "data": data})
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *rpcHost) DBUpdate(collection, id string, data map[string]interface{}) (map[string]interface{}, error) {
	raw, err := h.call("db.update", map[string]interface{}{"collection": collection, "id": id, "data": data})
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *rpcHost) DBDelete(collection, id string) error {
	_, err := h.call("db.delete", map[string]string{"collection": collection, "id": id})
	return err
}

func (h *rpcHost) Fetch(ctx context.Context, method, url string, headers map[string]string, body interface{}) (int, []byte, error) {
	raw, err := h.call("fetch", map[string]interface{}{
		"method": method, "url": url, "headers": headers, "body": body,
	})
	if err != nil {
		return 0, nil, err
	}
	var out struct {
		Status int    `json:"status"`
		Body   []byte `json:"body"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return 0, nil, err
	}
	return out.Status, out.Body, nil
}

func (h *rpcHost) StorageUpload(ctx context.Context, bucket, path, contentType string, data []byte) (map[string]interface{}, error) {
	if len(data) > StorageRPCMaxBytes {
		return nil, fmt.Errorf("storage: payload exceeds %d bytes", StorageRPCMaxBytes)
	}
	raw, err := h.call("storage.upload", map[string]interface{}{
		"bucket": bucket, "path": path, "content_type": contentType,
		"data": base64.StdEncoding.EncodeToString(data),
	})
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (h *rpcHost) StorageDownload(ctx context.Context, bucket, path string) ([]byte, map[string]interface{}, error) {
	raw, err := h.call("storage.download", map[string]string{"bucket": bucket, "path": path})
	if err != nil {
		return nil, nil, err
	}
	var out struct {
		Data     string                 `json:"data"`
		Metadata map[string]interface{} `json:"metadata"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, nil, err
	}
	data, err := base64.StdEncoding.DecodeString(out.Data)
	if err != nil {
		return nil, nil, err
	}
	if len(data) > StorageRPCMaxBytes {
		return nil, nil, fmt.Errorf("storage: download exceeds %d bytes", StorageRPCMaxBytes)
	}
	return data, out.Metadata, nil
}

func (h *rpcHost) StorageDelete(ctx context.Context, bucket, path string) error {
	_, err := h.call("storage.delete", map[string]string{"bucket": bucket, "path": path})
	return err
}

func (h *rpcHost) StorageList(ctx context.Context, bucket, prefix string) ([]map[string]interface{}, error) {
	raw, err := h.call("storage.list", map[string]string{"bucket": bucket, "prefix": prefix})
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func writeRPC(conn net.Conn, msg RPCMessage) error {
	b, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	_, err = conn.Write(append(b, '\n'))
	return err
}

func readRPC(conn net.Conn) (RPCMessage, error) {
	var msg RPCMessage
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return msg, err
		}
		return msg, fmt.Errorf("rpc: connection closed")
	}
	if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
		return msg, err
	}
	return msg, nil
}

// serveHostRPC handles host API calls from the worker on a single connection.
func serveHostRPC(conn net.Conn, host HostAPI) {
	defer conn.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var msg RPCMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		if msg.Type != "call" {
			continue
		}
		resp := dispatchHostCall(ctx, host, msg)
		_ = writeRPC(conn, resp)
	}
}

func dispatchHostCall(ctx context.Context, host HostAPI, msg RPCMessage) RPCMessage {
	if host == nil {
		host = DefaultHost
	}
	host = withContext(ctx, host)

	switch msg.Method {
	case "db.get":
		var p struct {
			Collection string `json:"collection"`
			ID         string `json:"id"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		v, err := host.DBGet(p.Collection, p.ID)
		if err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		b, _ := json.Marshal(v)
		return RPCMessage{Type: "resp", ID: msg.ID, Result: b}
	case "db.create":
		var p struct {
			Collection string                 `json:"collection"`
			Data       map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		v, err := host.DBCreate(p.Collection, p.Data)
		if err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		b, _ := json.Marshal(v)
		return RPCMessage{Type: "resp", ID: msg.ID, Result: b}
	case "db.update":
		var p struct {
			Collection string                 `json:"collection"`
			ID         string                 `json:"id"`
			Data       map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		v, err := host.DBUpdate(p.Collection, p.ID, p.Data)
		if err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		b, _ := json.Marshal(v)
		return RPCMessage{Type: "resp", ID: msg.ID, Result: b}
	case "db.delete":
		var p struct {
			Collection string `json:"collection"`
			ID         string `json:"id"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		if err := host.DBDelete(p.Collection, p.ID); err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		return RPCMessage{Type: "resp", ID: msg.ID, Result: []byte("{}")}
	case "fetch":
		var p struct {
			Method  string            `json:"method"`
			URL     string            `json:"url"`
			Headers map[string]string `json:"headers"`
			Body    interface{}       `json:"body"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		status, body, err := host.Fetch(ctx, p.Method, p.URL, p.Headers, p.Body)
		if err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		b, _ := json.Marshal(map[string]interface{}{"status": status, "body": body})
		return RPCMessage{Type: "resp", ID: msg.ID, Result: b}
	case "storage.upload":
		var p struct {
			Bucket      string `json:"bucket"`
			Path        string `json:"path"`
			ContentType string `json:"content_type"`
			Data        string `json:"data"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		data, err := DecodeStorageData(p.Data)
		if err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		v, err := host.StorageUpload(ctx, p.Bucket, p.Path, p.ContentType, data)
		if err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		b, _ := json.Marshal(v)
		return RPCMessage{Type: "resp", ID: msg.ID, Result: b}
	case "storage.download":
		var p struct {
			Bucket string `json:"bucket"`
			Path   string `json:"path"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		data, meta, err := host.StorageDownload(ctx, p.Bucket, p.Path)
		if err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		if len(data) > StorageRPCMaxBytes {
			return RPCMessage{Type: "err", ID: msg.ID, Error: fmt.Sprintf("storage: download exceeds %d bytes", StorageRPCMaxBytes)}
		}
		b, _ := json.Marshal(map[string]interface{}{
			"data":     base64.StdEncoding.EncodeToString(data),
			"metadata": meta,
		})
		return RPCMessage{Type: "resp", ID: msg.ID, Result: b}
	case "storage.delete":
		var p struct {
			Bucket string `json:"bucket"`
			Path   string `json:"path"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		if err := host.StorageDelete(ctx, p.Bucket, p.Path); err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		return RPCMessage{Type: "resp", ID: msg.ID, Result: []byte("[]")}
	case "storage.list":
		var p struct {
			Bucket string `json:"bucket"`
			Prefix string `json:"prefix"`
		}
		if err := json.Unmarshal(msg.Params, &p); err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		v, err := host.StorageList(ctx, p.Bucket, p.Prefix)
		if err != nil {
			return RPCMessage{Type: "err", ID: msg.ID, Error: err.Error()}
		}
		b, _ := json.Marshal(v)
		return RPCMessage{Type: "resp", ID: msg.ID, Result: b}
	default:
		return RPCMessage{Type: "err", ID: msg.ID, Error: "unknown method " + msg.Method}
	}
}
