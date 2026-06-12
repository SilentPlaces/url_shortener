package safeurl

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

var (
	ErrEmptyURL          = errors.New("url is empty")
	ErrInvalidScheme     = errors.New("url scheme must be http or https")
	ErrMissingHost       = errors.New("url host is required")
	ErrUserInfoPresent   = errors.New("url must not contain userinfo")
	ErrDisallowedPort    = errors.New("url port is not in the allow list")
	ErrPrivateAddress    = errors.New("url resolves to a private or loopback address")
	ErrAddressResolution = errors.New("failed to resolve url host")
)

type Resolver interface {
	LookupIP(host string) ([]net.IP, error)
}

type netResolver struct{}

func (netResolver) LookupIP(host string) ([]net.IP, error) { return net.LookupIP(host) }

var DefaultResolver Resolver = netResolver{}

type Validator struct {
	allowedPorts map[string]struct{}
	allowPrivate bool
	skipDNS      bool
	resolver     Resolver
}

type Option func(*Validator)

func WithAllowPrivate(allow bool) Option {
	return func(v *Validator) { v.allowPrivate = allow }
}

func WithSkipDNS(skip bool) Option {
	return func(v *Validator) { v.skipDNS = skip }
}

func WithAllowedPorts(ports ...string) Option {
	return func(v *Validator) {
		if len(ports) == 0 {
			v.allowedPorts = nil
			return
		}
		v.allowedPorts = make(map[string]struct{}, len(ports))
		for _, p := range ports {
			v.allowedPorts[p] = struct{}{}
		}
	}
}

func WithResolver(r Resolver) Option {
	return func(v *Validator) {
		if r != nil {
			v.resolver = r
		}
	}
}

func New(opts ...Option) *Validator {
	v := &Validator{
		allowedPorts: map[string]struct{}{
			"":    {},
			"80":  {},
			"443": {},
		},
		resolver: DefaultResolver,
	}
	for _, opt := range opts {
		opt(v)
	}
	return v
}

func (v *Validator) Validate(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ErrEmptyURL
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse url: %w", err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("%w: %q", ErrInvalidScheme, parsed.Scheme)
	}

	if parsed.User != nil {
		return ErrUserInfoPresent
	}

	host := parsed.Hostname()
	if host == "" {
		return ErrMissingHost
	}

	if v.allowedPorts != nil {
		port := parsed.Port()
		if _, ok := v.allowedPorts[port]; !ok {
			return fmt.Errorf("%w: %q", ErrDisallowedPort, port)
		}
	}

	if v.allowPrivate {
		return nil
	}

	ips, err := v.collectIPs(host)
	if err != nil {
		return err
	}
	for _, ip := range ips {
		if isDisallowedIP(ip) {
			return fmt.Errorf("%w: %s", ErrPrivateAddress, ip)
		}
	}
	return nil
}

func (v *Validator) collectIPs(host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, nil
	}
	if v.skipDNS {
		return nil, nil
	}
	ips, err := v.resolver.LookupIP(host)
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %v", ErrAddressResolution, host, err)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrAddressResolution, host)
	}
	return ips, nil
}

func isDisallowedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() ||
		ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsInterfaceLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsPrivate() {
		return true
	}
	if v4 := ip.To4(); v4 != nil && v4.Equal(net.IPv4(169, 254, 169, 254)) {
		return true
	}
	if v4 := ip.To4(); v4 != nil && v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127 {
		return true
	}
	return false
}
