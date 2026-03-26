package ipc

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

const listenAddr = "127.0.0.1:19265"

// Mode indicates whether to just add files or add and start encoding.
type Mode int

const (
	ModeAdd   Mode = iota // Just add to queue
	ModeStart             // Add and start encoding
)

// FileReceiver is called when files are received from another instance.
type FileReceiver func(mode Mode, files []string)

// TryConnect attempts to send files to an already-running instance.
// Returns true if another instance was found and files were sent.
func TryConnect(mode Mode, files []string) bool {
	conn, err := net.Dial("tcp", listenAddr)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Send mode as first line
	switch mode {
	case ModeStart:
		fmt.Fprintf(conn, "START\n")
	default:
		fmt.Fprintf(conn, "ADD\n")
	}

	for _, f := range files {
		fmt.Fprintf(conn, "%s\n", f)
	}
	fmt.Fprintf(conn, "END\n")
	return true
}

// Server listens for incoming file paths from new instances.
type Server struct {
	listener net.Listener
	onFiles  FileReceiver
}

// StartServer begins listening for IPC connections.
func StartServer(onFiles FileReceiver) (*Server, error) {
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return nil, fmt.Errorf("IPC listen failed: %w", err)
	}

	s := &Server{
		listener: ln,
		onFiles:  onFiles,
	}

	go s.acceptLoop()
	return s, nil
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

	mode := ModeAdd
	var files []string
	firstLine := true

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "END" {
			break
		}
		// First line is mode
		if firstLine {
			firstLine = false
			switch line {
			case "START":
				mode = ModeStart
				continue
			case "ADD":
				mode = ModeAdd
				continue
			}
			// If first line isn't a mode keyword, treat as filename (backward compat)
			if line != "" {
				files = append(files, line)
			}
			continue
		}
		if line != "" {
			files = append(files, line)
		}
	}

	if len(files) > 0 && s.onFiles != nil {
		s.onFiles(mode, files)
	}
}

func (s *Server) Close() {
	if s.listener != nil {
		s.listener.Close()
	}
}
