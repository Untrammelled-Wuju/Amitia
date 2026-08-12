//go:build linux && !android

package ssh

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type Service struct {
	policy   Policy
	store    *HostKeyStore
	mu       sync.Mutex
	sessions map[string]*sshSession
}

type sshSession struct {
	id        string
	host      string
	port      int
	user      string
	client    *ssh.Client
	lastUsed  time.Time
	createdAt time.Time
}

func NewService(policy Policy, store *HostKeyStore) *Service {
	return &Service{
		policy:   policy,
		store:    store,
		sessions: make(map[string]*sshSession),
	}
}

func (s *Service) Status(ctx context.Context) SSHStatus {
	s.mu.Lock()
	activeCount := len(s.sessions)
	s.mu.Unlock()

	defaultUser := os.Getenv("USER")
	if defaultUser == "" {
		defaultUser = "root"
	}

	knownHostsCount := 0
	if s.store != nil {
		knownHostsCount = s.store.Count()
	}

	return SSHStatus{
		Enabled:         s.policy.Enabled,
		DefaultUser:     defaultUser,
		KnownHostsCount: knownHostsCount,
		MaxSessions:     s.policy.MaxSessions,
		ActiveSessions:  activeCount,
	}
}

func (s *Service) Exec(ctx context.Context, req SSHExecRequest) (*SSHExecResult, error) {
	if !s.policy.Enabled {
		return nil, ErrConnectionFailed("ssh is disabled")
	}

	if req.Host == "" {
		return nil, ErrInvalidRequest("host is required")
	}
	if req.Command == "" {
		return nil, ErrInvalidRequest("command is required")
	}

	port := req.Port
	if port == 0 {
		port = 22
	}

	if isDeniedTarget(req.Host) {
		return nil, ErrHostDenied(req.Host)
	}

	endpointClass := classifyHost(req.Host)
	if !s.policy.IsPortAllowed(port, endpointClass) {
		return nil, ErrPortDenied(port)
	}

	addr := net.JoinHostPort(req.Host, fmt.Sprintf("%d", port))

	hostKeyPolicy := s.policy.DefaultHostKeyPolicy
	if req.HostKeyPolicy != "" {
		hostKeyPolicy = HostKeyPolicy(req.HostKeyPolicy)
	}

	sshConfig := &ssh.ClientConfig{
		User:            req.User,
		HostKeyCallback: s.hostKeyCallback(hostKeyPolicy, req.HostKey, req.Host, port),
		Timeout:         time.Duration(s.policy.DefaultTimeoutSecond) * time.Second,
	}

	if req.TimeoutMs > 0 {
		timeoutSec := req.TimeoutMs / 1000
		if timeoutSec > int64(s.policy.MaxTimeoutSecond) {
			timeoutSec = int64(s.policy.MaxTimeoutSecond)
		}
		sshConfig.Timeout = time.Duration(timeoutSec) * time.Second
	}

	if err := s.applyAuth(sshConfig, req); err != nil {
		return nil, err
	}

	maxOutput := s.policy.MaxOutputBytes
	if req.MaxOutputBytes > 0 && req.MaxOutputBytes < maxOutput {
		maxOutput = req.MaxOutputBytes
	}

	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		if os.IsTimeout(err) {
			return nil, ErrTimeout(fmt.Sprintf("connection to %s timed out", addr))
		}
		return nil, ErrConnectionFailed(fmt.Sprintf("dial %s: %v", addr, err))
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return nil, ErrConnectionFailed(fmt.Sprintf("create session: %v", err))
	}
	defer session.Close()

	if req.Environment != nil {
		for k, v := range req.Environment {
			session.Setenv(k, v)
		}
	}

	stdout, stderr, exitCode, durationMs, err := s.runCommand(session, req.Command, req.Stdin, req.WorkingDir, maxOutput)
	if err != nil {
		return nil, err
	}

	return &SSHExecResult{
		ExitCode:        exitCode,
		Stdout:          stdout.out,
		Stderr:          stderr.out,
		StdoutTruncated: stdout.truncated,
		StderrTruncated: stderr.truncated,
		StdoutBytes:     stdout.bytes,
		StderrBytes:     stderr.bytes,
		DurationMs:      durationMs,
	}, nil
}

type streamResult struct {
	out       string
	bytes     int64
	truncated bool
}

func (s *Service) runCommand(session *ssh.Session, command, stdin, workingDir string, maxOutput int64) (streamResult, streamResult, int, int64, error) {
	if workingDir != "" {
		command = fmt.Sprintf("cd %s && %s", workingDir, command)
	}

	if stdin != "" {
		session.Stdin = newStringReader(stdin)
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return streamResult{}, streamResult{}, 0, 0, ErrCommandFailed(fmt.Sprintf("stdout pipe: %v", err))
	}

	stderrPipe, err := session.StderrPipe()
	if err != nil {
		return streamResult{}, streamResult{}, 0, 0, ErrCommandFailed(fmt.Sprintf("stderr pipe: %v", err))
	}

	startTime := time.Now()
	if err := session.Start(command); err != nil {
		return streamResult{}, streamResult{}, 0, 0, ErrCommandFailed(fmt.Sprintf("start command: %v", err))
	}

	stdoutResult := readLimited(stdoutPipe, maxOutput/2)
	stderrResult := readLimited(stderrPipe, maxOutput/2)

	if err := session.Wait(); err != nil {
		exitCode := 0
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		}
		durationMs := time.Since(startTime).Milliseconds()
		return stdoutResult, stderrResult, exitCode, durationMs, nil
	}

	durationMs := time.Since(startTime).Milliseconds()
	return stdoutResult, stderrResult, 0, durationMs, nil
}

func readLimited(r interface{ Read([]byte) (int, error) }, maxBytes int64) streamResult {
	if maxBytes <= 0 {
		maxBytes = 1024 * 1024
	}

	var buf []byte
	tmp := make([]byte, 4096)
	var totalRead int64

	for totalRead < maxBytes {
		n, err := r.Read(tmp)
		if n > 0 {
			remaining := maxBytes - totalRead
			if int64(n) > remaining {
				n = int(remaining)
			}
			buf = append(buf, tmp[:n]...)
			totalRead += int64(n)
		}
		if err != nil {
			break
		}
	}

	return streamResult{
		out:       string(buf),
		bytes:     totalRead,
		truncated: totalRead >= maxBytes,
	}
}

func (s *Service) applyAuth(config *ssh.ClientConfig, req SSHExecRequest) error {
	var authMethods []ssh.AuthMethod

	if req.PrivateKey != "" {
		signer, err := s.parsePrivateKey(req.PrivateKey, req.Password)
		if err != nil {
			return err
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if req.Password != "" {
		authMethods = append(authMethods, ssh.Password(req.Password))
	}

	if s.policy.EnableAgentAuth && req.AgentAuth {
		return ErrUnsupportedAuth("agent auth not actually implemented in this version")
	}

	if len(authMethods) == 0 {
		return ErrAuthFailed("no authentication method provided (requires privateKey or password)")
	}

	config.Auth = authMethods
	return nil
}

func (s *Service) parsePrivateKey(keyPEM, passphrase string) (ssh.Signer, error) {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, ErrInvalidHostKey("failed to parse PEM block")
	}

	var signer ssh.Signer
	var err error

	if passphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase([]byte(keyPEM), []byte(passphrase))
	} else {
		signer, err = ssh.ParsePrivateKey([]byte(keyPEM))
	}

	if err != nil {
		if _, ok := err.(*x509.UnknownAuthorityError); ok {
			return nil, ErrInvalidHostKey("invalid private key")
		}
		return nil, ErrAuthFailed(fmt.Sprintf("parse private key: %v", err))
	}

	return signer, nil
}

func (s *Service) hostKeyCallback(policy HostKeyPolicy, expectedKey, host string, port int) ssh.HostKeyCallback {
	if policy == HostKeyPolicyAcceptNew {
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			if s.store != nil {
				fingerprint := ComputeFingerprint(key.Marshal())
				s.store.Put(KnownHost{
					Host:        host,
					Port:        port,
					Algorithm:   key.Type(),
					Fingerprint: fingerprint,
					FirstSeen:   time.Now(),
					LastSeen:    time.Now(),
				})
			}
			return nil
		}
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if s.store == nil {
			return ErrHostKeyDenied(host)
		}

		fingerprint := ComputeFingerprint(key.Marshal())

		if expectedKey != "" && expectedKey != fingerprint {
			return ErrHostKeyMismatch(host)
		}

		knownHost, found := s.store.Get(host, port)
		if !found {
			return ErrHostKeyUnknown(host)
		}

		if knownHost.Fingerprint != fingerprint {
			return ErrHostKeyMismatch(host)
		}

		return nil
	}
}

func (s *Service) ScanHostKey(ctx context.Context, req HostKeyScanRequest) (*HostKeyScanResult, error) {
	if req.Host == "" {
		return nil, ErrInvalidRequest("host is required")
	}

	port := req.Port
	if port == 0 {
		port = 22
	}

	if isDeniedTarget(req.Host) {
		return nil, ErrHostDenied(req.Host)
	}

	addr := net.JoinHostPort(req.Host, fmt.Sprintf("%d", port))

	timeout := 10 * time.Second
	if req.TimeoutMs > 0 {
		timeout = time.Duration(req.TimeoutMs) * time.Millisecond
	}

	result := &HostKeyScanResult{
		Host:         req.Host,
		Port:         port,
		Algorithms:   make([]string, 0),
		RawKeys:      make([]string, 0),
		Fingerprints: make([]string, 0),
	}

	config := &ssh.ClientConfig{
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			result.Algorithms = append(result.Algorithms, key.Type())
			result.RawKeys = append(result.RawKeys, base64Encode(key.Marshal()))
			result.Fingerprints = append(result.Fingerprints, ComputeFingerprint(key.Marshal()))
			return fmt.Errorf("skip")
		},
		Timeout: timeout,
	}

	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, ErrConnectionFailed(fmt.Sprintf("dial %s: %v", addr, err))
	}
	conn.Close()

	_, _, _, _ = ssh.NewClientConn(conn, addr, config)

	return result, nil
}

func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, sess := range s.sessions {
		if sess.client != nil {
			sess.client.Close()
		}
		delete(s.sessions, id)
	}
}

type stringReader struct {
	s string
	i int
}

func newStringReader(s string) *stringReader {
	return &stringReader{s: s}
}

func (r *stringReader) Read(p []byte) (int, error) {
	if r.i >= len(r.s) {
		return 0, fmt.Errorf("EOF")
	}
	n := copy(p, r.s[r.i:])
	r.i += n
	return n, nil
}

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

func isDeniedTarget(host string) bool {
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	ip := net.ParseIP(host)
	if ip != nil {
		return ip.IsLoopback() || ip.IsPrivate()
	}
	return false
}

func classifyHost(host string) string {
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() {
			return "loopback"
		}
		if ip.IsPrivate() {
			return "private"
		}
		return "public"
	}
	if host == "localhost" {
		return "loopback"
	}
	return "public"
}
