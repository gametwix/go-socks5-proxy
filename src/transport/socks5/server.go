package socks5

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

const (
	socksVersion5 = 0x05
	authUserPass  = 0x02
	authNoMethods = 0xFF

	authVersion1   = 0x01
	authStatusFail = 0x01
	authStatusOK   = 0x00

	cmdConnect = 0x01

	atypIPv4   = 0x01
	atypDomain = 0x03
	atypIPv6   = 0x04

	repSuccess            = 0x00
	repGeneralFailure     = 0x01
	repNetworkUnreachable = 0x03
	repHostUnreachable    = 0x04
	repConnectionRefused  = 0x05
	repCommandNotSupport  = 0x07
	repAddrTypeNotSupport = 0x08
)

type AuthService interface {
	ValidateCredentials(ctx context.Context, username, password string) (bool, error)
	MarkAuthenticated(ctx context.Context, username string) error
}

type StatsService interface {
	AddTraffic(ctx context.Context, username string, upload, download int64) error
}

type Server struct {
	auth        AuthService
	stats       StatsService
	dialTimeout time.Duration
}

func New(auth AuthService, stats StatsService, dialTimeout time.Duration) *Server {
	return &Server{auth: auth, stats: stats, dialTimeout: dialTimeout}
}

func (s *Server) Serve(ctx context.Context, addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	defer ln.Close()

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	log.Printf("SOCKS5 listening on %s", addr)

	for {
		conn, acceptErr := ln.Accept()
		if acceptErr != nil {
			if ctx.Err() != nil {
				return nil
			}
			if ne, ok := acceptErr.(net.Error); ok && ne.Temporary() {
				continue
			}
			return acceptErr
		}
		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(client net.Conn) {
	defer client.Close()
	ctx := context.Background()

	username, ok := s.handshakeAndAuth(ctx, client)
	if !ok {
		return
	}

	targetAddr, rep, ok := s.readConnectRequest(client)
	if !ok {
		sendReply(client, rep, nil)
		return
	}

	upstream, err := net.DialTimeout("tcp", targetAddr, s.dialTimeout)
	if err != nil {
		sendReply(client, classifyDialErr(err), nil)
		return
	}
	defer upstream.Close()

	localAddr, _ := upstream.LocalAddr().(*net.TCPAddr)
	sendReply(client, repSuccess, localAddr)

	var upload int64
	var download int64
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		_, _ = io.Copy(countingWriter{w: upstream, n: &upload}, client)
		closeWrite(upstream)
	}()

	_, _ = io.Copy(countingWriter{w: client, n: &download}, upstream)
	closeWrite(client)
	wg.Wait()

	if err = s.stats.AddTraffic(ctx, username, upload, download); err != nil {
		log.Printf("failed to persist traffic for %s: %v", username, err)
	}
}

func (s *Server) handshakeAndAuth(ctx context.Context, conn net.Conn) (string, bool) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(conn, header); err != nil {
		return "", false
	}
	if header[0] != socksVersion5 {
		return "", false
	}

	nMethods := int(header[1])
	methods := make([]byte, nMethods)
	if _, err := io.ReadFull(conn, methods); err != nil {
		return "", false
	}

	supportsUserPass := false
	for _, method := range methods {
		if method == authUserPass {
			supportsUserPass = true
			break
		}
	}

	if !supportsUserPass {
		_, _ = conn.Write([]byte{socksVersion5, authNoMethods})
		return "", false
	}

	if _, err := conn.Write([]byte{socksVersion5, authUserPass}); err != nil {
		return "", false
	}

	authHeader := make([]byte, 2)
	if _, err := io.ReadFull(conn, authHeader); err != nil {
		return "", false
	}
	if authHeader[0] != authVersion1 {
		_, _ = conn.Write([]byte{authVersion1, authStatusFail})
		return "", false
	}

	usernameLen := int(authHeader[1])
	if usernameLen <= 0 {
		_, _ = conn.Write([]byte{authVersion1, authStatusFail})
		return "", false
	}

	usernameBuf := make([]byte, usernameLen)
	if _, err := io.ReadFull(conn, usernameBuf); err != nil {
		return "", false
	}

	passwordLenBuf := make([]byte, 1)
	if _, err := io.ReadFull(conn, passwordLenBuf); err != nil {
		return "", false
	}
	passwordLen := int(passwordLenBuf[0])
	passwordBuf := make([]byte, passwordLen)
	if _, err := io.ReadFull(conn, passwordBuf); err != nil {
		return "", false
	}

	username := string(usernameBuf)
	password := string(passwordBuf)

	valid, err := s.auth.ValidateCredentials(ctx, username, password)
	if err != nil {
		log.Printf("redis auth error: %v", err)
		_, _ = conn.Write([]byte{authVersion1, authStatusFail})
		return "", false
	}
	if !valid {
		_, _ = conn.Write([]byte{authVersion1, authStatusFail})
		return "", false
	}
	if err = s.auth.MarkAuthenticated(ctx, username); err != nil {
		log.Printf("failed to save auth date for %s: %v", username, err)
	}

	if _, err = conn.Write([]byte{authVersion1, authStatusOK}); err != nil {
		return "", false
	}
	return username, true
}

func (s *Server) readConnectRequest(conn net.Conn) (string, byte, bool) {
	req := make([]byte, 4)
	if _, err := io.ReadFull(conn, req); err != nil {
		return "", repGeneralFailure, false
	}

	if req[0] != socksVersion5 {
		return "", repGeneralFailure, false
	}
	if req[1] != cmdConnect {
		return "", repCommandNotSupport, false
	}

	atyp := req[3]
	address, err := readAddress(conn, atyp)
	if err != nil {
		if errors.Is(err, errUnsupportedATYP) {
			return "", repAddrTypeNotSupport, false
		}
		return "", repGeneralFailure, false
	}

	portBuf := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBuf); err != nil {
		return "", repGeneralFailure, false
	}
	port := binary.BigEndian.Uint16(portBuf)
	return net.JoinHostPort(address, strconv.Itoa(int(port))), repSuccess, true
}

var errUnsupportedATYP = errors.New("unsupported atyp")

func readAddress(conn net.Conn, atyp byte) (string, error) {
	switch atyp {
	case atypIPv4:
		buf := make([]byte, net.IPv4len)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		return net.IP(buf).String(), nil
	case atypIPv6:
		buf := make([]byte, net.IPv6len)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		return net.IP(buf).String(), nil
	case atypDomain:
		lenBuf := make([]byte, 1)
		if _, err := io.ReadFull(conn, lenBuf); err != nil {
			return "", err
		}
		domainLen := int(lenBuf[0])
		if domainLen <= 0 {
			return "", errors.New("empty domain")
		}
		buf := make([]byte, domainLen)
		if _, err := io.ReadFull(conn, buf); err != nil {
			return "", err
		}
		return string(buf), nil
	default:
		return "", errUnsupportedATYP
	}
}

func classifyDialErr(err error) byte {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return repHostUnreachable
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return repConnectionRefused
		}
		if errors.Is(opErr.Err, syscall.ENETUNREACH) {
			return repNetworkUnreachable
		}
		if errors.Is(opErr.Err, syscall.EHOSTUNREACH) {
			return repHostUnreachable
		}
	}
	return repHostUnreachable
}

func sendReply(conn net.Conn, rep byte, addr *net.TCPAddr) {
	if addr == nil {
		_, _ = conn.Write([]byte{socksVersion5, rep, 0x00, atypIPv4, 0, 0, 0, 0, 0, 0})
		return
	}

	if ip4 := addr.IP.To4(); ip4 != nil {
		resp := make([]byte, 10)
		resp[0] = socksVersion5
		resp[1] = rep
		resp[2] = 0x00
		resp[3] = atypIPv4
		copy(resp[4:8], ip4)
		binary.BigEndian.PutUint16(resp[8:10], uint16(addr.Port))
		_, _ = conn.Write(resp)
		return
	}

	ip16 := addr.IP.To16()
	if ip16 == nil {
		ip16 = net.IPv6unspecified
	}
	resp := make([]byte, 22)
	resp[0] = socksVersion5
	resp[1] = rep
	resp[2] = 0x00
	resp[3] = atypIPv6
	copy(resp[4:20], ip16)
	binary.BigEndian.PutUint16(resp[20:22], uint16(addr.Port))
	_, _ = conn.Write(resp)
}

func closeWrite(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		_ = conn.Close()
		return
	}
	_ = tcpConn.CloseWrite()
}

type countingWriter struct {
	w io.Writer
	n *int64
}

func (cw countingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	atomic.AddInt64(cw.n, int64(n))
	return n, err
}
