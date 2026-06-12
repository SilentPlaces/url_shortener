package safeurl

import (
	"errors"
	"net"
	"testing"
)

type fakeResolver map[string][]net.IP

func (f fakeResolver) LookupIP(host string) ([]net.IP, error) {
	if ips, ok := f[host]; ok {
		return ips, nil
	}
	return nil, errors.New("no such host")
}

func TestValidate_AcceptsPublicURL(t *testing.T) {
	v := New(WithResolver(fakeResolver{
		"example.com": {net.IPv4(93, 184, 216, 34)},
	}))
	if err := v.Validate("https://example.com/path"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_RejectsEmpty(t *testing.T) {
	if err := New().Validate("   "); !errors.Is(err, ErrEmptyURL) {
		t.Fatalf("expected ErrEmptyURL, got %v", err)
	}
}

func TestValidate_RejectsNonHTTP(t *testing.T) {
	if err := New().Validate("ftp://example.com"); !errors.Is(err, ErrInvalidScheme) {
		t.Fatalf("expected ErrInvalidScheme, got %v", err)
	}
}

func TestValidate_RejectsUserInfo(t *testing.T) {
	v := New(WithSkipDNS(true))
	if err := v.Validate("https://user:pass@example.com"); !errors.Is(err, ErrUserInfoPresent) {
		t.Fatalf("expected ErrUserInfoPresent, got %v", err)
	}
}

func TestValidate_RejectsDisallowedPort(t *testing.T) {
	v := New(WithSkipDNS(true))
	if err := v.Validate("https://example.com:5432"); !errors.Is(err, ErrDisallowedPort) {
		t.Fatalf("expected ErrDisallowedPort, got %v", err)
	}
}

func TestValidate_RejectsLoopbackLiteral(t *testing.T) {
	v := New()
	for _, raw := range []string{
		"http://127.0.0.1",
		"http://[::1]/",
	} {
		if err := v.Validate(raw); !errors.Is(err, ErrPrivateAddress) {
			t.Fatalf("expected ErrPrivateAddress for %s, got %v", raw, err)
		}
	}
}

func TestValidate_RejectsPrivateRanges(t *testing.T) {
	v := New()
	for _, raw := range []string{
		"http://10.0.0.1",
		"http://192.168.1.1",
		"http://172.16.0.1",
		"http://169.254.169.254/latest/meta-data/",
		"http://100.64.0.1",
	} {
		if err := v.Validate(raw); !errors.Is(err, ErrPrivateAddress) {
			t.Fatalf("expected ErrPrivateAddress for %s, got %v", raw, err)
		}
	}
}

func TestValidate_RejectsResolvedPrivateHost(t *testing.T) {
	v := New(WithResolver(fakeResolver{
		"intranet.example.com": {net.IPv4(10, 0, 0, 1)},
	}))
	err := v.Validate("https://intranet.example.com")
	if !errors.Is(err, ErrPrivateAddress) {
		t.Fatalf("expected ErrPrivateAddress, got %v", err)
	}
}

func TestValidate_AllowPrivateBypassesIPChecks(t *testing.T) {
	v := New(WithAllowPrivate(true))
	if err := v.Validate("http://127.0.0.1"); err != nil {
		t.Fatalf("unexpected error with allowPrivate=true: %v", err)
	}
}

func TestValidate_DNSFailureIsReported(t *testing.T) {
	v := New(WithResolver(fakeResolver{}))
	err := v.Validate("https://nope.example")
	if !errors.Is(err, ErrAddressResolution) {
		t.Fatalf("expected ErrAddressResolution, got %v", err)
	}
}
