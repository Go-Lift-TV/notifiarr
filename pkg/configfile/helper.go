package configfile

import (
	"fmt"
	"net"
	"strings"

	"github.com/Notifiarr/notifiarr/pkg/mnd"
)

/* This is a helper method to check if an IP is in a list/cidr. */

// AllowedIPs determines who can set x-forwarded-for.
type AllowedIPs []*net.IPNet

var _ = fmt.Stringer(AllowedIPs(nil))

// String turns a list of allowedIPs into a printable masterpiece.
func (n AllowedIPs) String() (s string) {
	if len(n) < 1 {
		return "(none)"
	}

	for i := range n {
		if s != "" {
			s += ", "
		}

		s += n[i].String()
	}

	return s
}

// Contains returns true if an IP is allowed.
func (n AllowedIPs) Contains(ip string) bool {
	for i := range n {
		if n[i].Contains(net.ParseIP(ip)) {
			return true
		}
	}

	return false
}

// MakeIPs turns a list of CIDR strings (or plain IPs) into a list of net.IPNet.
// This "allowed" list is later used to check incoming IPs from web requests.
func MakeIPs(upstreams []string) (a AllowedIPs) {
	for _, ip := range upstreams {
		if !strings.Contains(ip, "/") {
			if strings.Contains(ip, ":") {
				ip += "/128"
			} else {
				ip += "/32"
			}
		}

		if _, i, err := net.ParseCIDR(ip); err == nil {
			a = append(a, i)
		}
	}

	return a
}

// CheckPort attempts to bind to a port to check if it's in use or not.
// We use this to check the port before starting the webserver.
func CheckPort(addr string) (string, error) {
	// Cleanup user input.
	addr = strings.TrimPrefix(strings.TrimPrefix(strings.TrimRight(addr, "/"), "http://"), "https://")
	if addr == "" {
		addr = mnd.DefaultBindAddr
	} else if !strings.Contains(addr, ":") {
		addr = "0.0.0.0:" + addr
	}

	a, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return addr, fmt.Errorf("provided ip:port combo is invalid: %w", err)
	}

	l, err := net.ListenTCP("tcp", a)
	if err != nil {
		return addr, fmt.Errorf("unable to listen on provided ip:port: %w", err)
	}
	defer l.Close()

	return addr, nil
}
