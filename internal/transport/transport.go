// Package transport exposes Agent Commons over authenticated local RPC and MCP.
package transport

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"agentcommons/internal/core"
)

const maxBody = 2 << 20

type request struct {
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}
type response struct {
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

func handler(s *core.Service) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fail := func(code int, msg string) { w.WriteHeader(code); _ = json.NewEncoder(w).Encode(response{Error: msg}) }
		if r.URL.Path != "/rpc" || r.Method != "POST" {
			fail(http.StatusNotFound, "POST /rpc required")
			return
		}
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") {
			fail(http.StatusUnauthorized, "authentication required")
			return
		}
		actor, ok := s.Authenticate(strings.TrimPrefix(auth, "Bearer "))
		if !ok {
			fail(http.StatusUnauthorized, "invalid credentials")
			return
		}
		dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBody))
		dec.DisallowUnknownFields()
		var req request
		if err := dec.Decode(&req); err != nil {
			fail(http.StatusBadRequest, "invalid request")
			return
		}
		var extra any
		if dec.Decode(&extra) != io.EOF {
			fail(http.StatusBadRequest, "one request required")
			return
		}
		var result any
		var err error
		if req.Method == "methods.list" {
			result = Methods()
		} else {
			result, err = s.Call(actor, req.Method, req.Params)
		}
		if err != nil {
			fail(http.StatusBadRequest, err.Error())
			return
		}
		// Operational evidence records only authenticated identity and operation.
		// Never log request parameters, returned content, or credentials.
		log.Printf("rpc actor=%q method=%q", actor, req.Method)
		data, err := json.Marshal(result)
		if err != nil {
			fail(http.StatusInternalServerError, "encoding failure")
			return
		}
		_ = json.NewEncoder(w).Encode(response{Result: data})
	})
}

// Serve refuses to replace an existing socket: a stale endpoint requires explicit cleanup.
func Serve(ctx context.Context, s *core.Service, socket string) error {
	dir := filepath.Dir(socket)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	info, err := os.Stat(dir)
	if err != nil {
		return err
	}
	if info.Mode().Perm()&0077 != 0 {
		return errors.New("socket directory must have mode 0700")
	}
	ln, err := net.Listen("unix", socket)
	if err != nil {
		return err
	}
	defer ln.Close()
	if err = os.Chmod(socket, 0600); err != nil {
		return err
	}
	srv := &http.Server{Handler: handler(s), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, MaxHeaderBytes: 8192}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = srv.Close()
		case <-done:
		}
	}()
	err = srv.Serve(ln)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

func Call(ctx context.Context, socket, token, method string, params json.RawMessage) (json.RawMessage, error) {
	body, err := json.Marshal(request{Method: method, Params: params})
	if err != nil {
		return nil, err
	}
	if len(body) > maxBody {
		return nil, errors.New("request too large")
	}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	defer tr.CloseIdleConnections()
	client := &http.Client{Transport: tr, Timeout: 35 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", "http://localhost/rpc", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBody {
		return nil, errors.New("response too large")
	}
	var out response
	if err = json.Unmarshal(data, &out); err != nil {
		return nil, errors.New("invalid RPC response")
	}
	if out.Error != "" {
		return nil, errors.New(out.Error)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RPC HTTP status %d", resp.StatusCode)
	}
	return out.Result, nil
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}
type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

// MCP implements newline-delimited JSON-RPC for an explicitly credentialed session.
// Registration and administrative retry are intentionally absent from the tool surface.
func MCP(ctx context.Context, in io.Reader, out io.Writer, socket, token string) error {
	scan := bufio.NewScanner(in)
	scan.Buffer(make([]byte, 4096), maxBody)
	enc := json.NewEncoder(out)
	for scan.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		var req rpcRequest
		if err := json.Unmarshal(scan.Bytes(), &req); err != nil {
			if err = enc.Encode(rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32700, "Parse error"}}); err != nil {
				return err
			}
			continue
		}
		var id any
		validID := len(req.ID) == 0 || json.Unmarshal(req.ID, &id) == nil
		if id != nil {
			switch id.(type) {
			case string, float64:
			default:
				validID = false
			}
		}
		if req.JSONRPC != "2.0" || req.Method == "" || !validID {
			if err := enc.Encode(rpcResponse{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &rpcError{-32600, "Invalid Request"}}); err != nil {
				return err
			}
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		res := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if req.JSONRPC != "2.0" {
			res.Error = &rpcError{-32600, "Invalid Request"}
		} else {
			switch req.Method {
			case "initialize":
				res.Result = map[string]any{"protocolVersion": "2024-11-05", "capabilities": map[string]any{"tools": map[string]any{}}, "serverInfo": map[string]any{"name": "agent-commons", "version": "0.1.0"}}
			case "ping":
				res.Result = map[string]any{}
			case "tools/list":
				res.Result = map[string]any{"tools": toolDefinitions()}
			case "tools/call":
				var p struct {
					Name      string          `json:"name"`
					Arguments json.RawMessage `json:"arguments"`
				}
				if err := json.Unmarshal(req.Params, &p); err != nil || !isTool(p.Name) {
					res.Error = &rpcError{-32602, "Unknown tool or invalid parameters"}
					break
				}
				if len(p.Arguments) == 0 {
					p.Arguments = json.RawMessage("{}")
				}
				data, err := Call(ctx, socket, token, p.Name, p.Arguments)
				text := string(data)
				if err != nil {
					text = err.Error()
				}
				res.Result = map[string]any{"content": []any{map[string]any{"type": "text", "text": text}}, "isError": err != nil}
			default:
				res.Error = &rpcError{-32601, "Method not found"}
			}
		}
		if err := enc.Encode(res); err != nil {
			return err
		}
	}
	return scan.Err()
}
