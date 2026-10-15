// Copyright (c) 2026 OpenDefender Contributors
// SPDX-License-Identifier: AGPL-3.0-only
// This program is free software: you can redistribute it and/or modify it under
// the terms of the GNU Affero General Public License v3.0 (see LICENSE).

// Package smtpmock is an in-process SMTP server for tests. It speaks just
// enough of RFC 5321 for net/smtp (EHLO, MAIL, RCPT, DATA, RSET, NOOP, QUIT),
// advertises no STARTTLS and no AUTH, and records every message it accepts.
//
// It exists so a test can prove that a notice actually left through the real
// transport (email.Transport), not through a fake that only records a call.
package smtpmock

import (
	"bufio"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// Message is one accepted mail.
type Message struct {
	From string
	To   []string
	Data string // headers + body, as received
}

// Server is a running mock.
type Server struct {
	ln   net.Listener
	mu   sync.Mutex
	msgs []Message
	wg   sync.WaitGroup
}

// Start listens on a free loopback port and stops when the test ends.
func Start(t *testing.T) *Server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("smtpmock: listen: %v", err)
	}
	s := &Server{ln: ln}
	s.wg.Add(1)
	go s.serve()
	t.Cleanup(func() {
		_ = ln.Close()
		s.wg.Wait()
	})
	return s
}

// Host is the listening address's host.
func (s *Server) Host() string { return s.ln.Addr().(*net.TCPAddr).IP.String() }

// Port is the listening port.
func (s *Server) Port() int { return s.ln.Addr().(*net.TCPAddr).Port }

// Messages returns a copy of everything accepted so far.
func (s *Server) Messages() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Message, len(s.msgs))
	copy(out, s.msgs)
	return out
}

func (s *Server) serve() {
	defer s.wg.Done()
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handle(conn)
		}()
	}
}

func (s *Server) handle(conn net.Conn) {
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)
	reply := func(code int, text string) {
		_, _ = w.WriteString(strconv.Itoa(code) + " " + text + "\r\n")
		_ = w.Flush()
	}

	reply(220, "smtpmock ready")
	var cur Message
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		verb := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(verb, "EHLO"), strings.HasPrefix(verb, "HELO"):
			reply(250, "smtpmock")
		case strings.HasPrefix(verb, "MAIL FROM:"):
			cur = Message{From: addr(line[len("MAIL FROM:"):])}
			reply(250, "ok")
		case strings.HasPrefix(verb, "RCPT TO:"):
			cur.To = append(cur.To, addr(line[len("RCPT TO:"):]))
			reply(250, "ok")
		case verb == "DATA":
			reply(354, "end with <CRLF>.<CRLF>")
			var b strings.Builder
			for {
				l, err := r.ReadString('\n')
				if err != nil {
					return
				}
				if l == ".\r\n" || l == ".\n" {
					break
				}
				b.WriteString(strings.TrimPrefix(l, "."))
			}
			cur.Data = b.String()
			s.mu.Lock()
			s.msgs = append(s.msgs, cur)
			s.mu.Unlock()
			cur = Message{}
			reply(250, "queued")
		case verb == "RSET":
			cur = Message{}
			reply(250, "ok")
		case verb == "NOOP":
			reply(250, "ok")
		case verb == "QUIT":
			reply(221, "bye")
			return
		default:
			reply(502, "not implemented")
		}
	}
}

func addr(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, ' '); i >= 0 { // drop ESMTP params (BODY=8BITMIME)
		s = s[:i]
	}
	return strings.Trim(s, "<>")
}
