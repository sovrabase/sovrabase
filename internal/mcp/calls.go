package mcp

import "time"

// callRingSize is how many recent tool calls are kept per project.
const callRingSize = 200

// Call records one tools/call request.
type Call struct {
	At         time.Time `json:"at"`
	TokenID    string    `json:"token_id"`
	TokenName  string    `json:"token_name"`
	Tool       string    `json:"tool"`
	OK         bool      `json:"ok"`
	DurationMs int64     `json:"duration_ms"`
	Error      string    `json:"error"`
}

type callRing struct {
	buf  [callRingSize]Call
	next int
	size int
}

func (r *callRing) add(c Call) {
	r.buf[r.next] = c
	r.next = (r.next + 1) % callRingSize
	if r.size < callRingSize {
		r.size++
	}
}

// newest returns up to limit calls, newest first.
func (r *callRing) newest(limit int) []Call {
	if limit <= 0 || limit > r.size {
		limit = r.size
	}
	out := make([]Call, 0, limit)
	for i := 0; i < limit; i++ {
		idx := (r.next - 1 - i + callRingSize) % callRingSize
		out = append(out, r.buf[idx])
	}
	return out
}

func (s *Server) recordCall(projectID string, c Call) {
	s.callsMu.Lock()
	defer s.callsMu.Unlock()
	r, ok := s.calls[projectID]
	if !ok {
		r = &callRing{}
		s.calls[projectID] = r
	}
	r.add(c)
}

// Calls returns a project's recent tool calls, newest first (in memory,
// last 200, reset on restart).
func (s *Server) Calls(projectID string, limit int) []Call {
	s.callsMu.Lock()
	defer s.callsMu.Unlock()
	r, ok := s.calls[projectID]
	if !ok {
		return []Call{}
	}
	return r.newest(limit)
}
