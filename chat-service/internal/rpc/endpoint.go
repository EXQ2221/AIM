package rpc

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

// NormalizeHostPort validates endpoint as TCP host:port and normalizes it.
// It prevents Kitex WithHostPorts from falling back to unix socket resolution.
func NormalizeHostPort(raw string) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", fmt.Errorf("endpoint is empty")
	}

	if _, err := net.ResolveTCPAddr("tcp", value); err == nil {
		return value, nil
	}

	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return "", fmt.Errorf("invalid endpoint %q: %w", value, err)
	}

	host = strings.TrimSpace(host)
	port = strings.TrimSpace(port)
	if host == "" || port == "" {
		return "", fmt.Errorf("invalid endpoint %q: host or port is empty", value)
	}
	portNum, convErr := strconv.Atoi(port)
	if convErr != nil || portNum <= 0 || portNum > 65535 {
		return "", fmt.Errorf("invalid endpoint %q: invalid port", value)
	}

	normalized := net.JoinHostPort(host, strconv.Itoa(portNum))
	if _, err := net.ResolveTCPAddr("tcp", normalized); err != nil {
		return "", fmt.Errorf("invalid endpoint %q: %w", value, err)
	}
	return normalized, nil
}
