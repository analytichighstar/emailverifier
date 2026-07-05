package validator

import (
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

// DisposableValidator handles disposable email validation
type DisposableValidator struct {
	disposableDomains  map[string]struct{}
	registrableDomains map[string]struct{}
}

// NewDisposableValidator creates a new instance of DisposableValidator using the config file
func NewDisposableValidator() (*DisposableValidator, error) {
	// Get the project root directory
	projectRoot, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Keep going up until we find the config directory or hit the root
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "config")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			return nil, err
		}
		projectRoot = parent
	}

	reader := NewFileDomainReader(filepath.Join(projectRoot, "config", "disposable_domains.txt"))
	return NewDisposableValidatorWithReader(reader)
}

// NewDisposableValidatorWithDomains creates a new instance of DisposableValidator with a custom list of domains
func NewDisposableValidatorWithDomains(domains []string) *DisposableValidator {
	disposableDomains := make(map[string]struct{}, len(domains))
	registrableDomains := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		normalized := normalizeDomain(domain)
		if normalized == "" {
			continue
		}
		disposableDomains[normalized] = struct{}{}
		// Always extract and store the registrable domain for subdomain matching
		if registrable, err := publicsuffix.EffectiveTLDPlusOne(normalized); err == nil {
			registrableDomains[registrable] = struct{}{}
		}
	}
	return &DisposableValidator{
		disposableDomains:  disposableDomains,
		registrableDomains: registrableDomains,
	}
}

// NewDisposableValidatorWithReader creates a new instance of DisposableValidator using a DomainReader
func NewDisposableValidatorWithReader(reader DomainReader) (*DisposableValidator, error) {
	domains, err := reader.ReadDomains()
	if err != nil {
		return nil, err
	}
	return NewDisposableValidatorWithDomains(domains), nil
}

// Validate checks if the email domain is from a disposable email provider
func (v *DisposableValidator) Validate(domain string) bool {
	normalized := normalizeDomain(domain)
	if normalized == "" {
		return false
	}

	// Check exact match in disposable domains
	if _, exists := v.disposableDomains[normalized]; exists {
		return true
	}

	// Get the registrable domain (e.g., foo.bar.tempmail.com -> tempmail.com)
	registrable, err := publicsuffix.EffectiveTLDPlusOne(normalized)
	if err != nil {
		return false
	}

	// Check if the registrable domain is blocked
	_, exists := v.registrableDomains[registrable]
	return exists
}

// normalizeDomain lowercases and trims the domain.
// IDN conversion only happens for non-ASCII domains.
func normalizeDomain(domain string) string {
	if len(domain) == 0 {
		return ""
	}

	trimmed := strings.TrimSpace(domain)
	trimmed = strings.TrimRight(trimmed, ".")
	if trimmed == "" {
		return ""
	}

	// Fast path: ASCII-only domains (99% of cases)
	if isASCII(trimmed) {
		return toLowerASCII(trimmed)
	}

	// IDN conversion for international domains
	ascii, err := idna.Lookup.ToASCII(trimmed)
	if err != nil {
		// Fallback: use strings.ToLower which handles Unicode
		return strings.ToLower(trimmed)
	}
	// ToASCII output is always ASCII, so use efficient lowercase
	return toLowerASCII(ascii)
}

// isASCII checks if a string contains only ASCII characters
func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] > 127 {
			return false
		}
	}
	return true
}

// toLowerASCII lowercases ASCII strings efficiently
func toLowerASCII(s string) string {
	// Check if already lowercase
	hasUpper := false
	for i := 0; i < len(s); i++ {
		if s[i] >= 'A' && s[i] <= 'Z' {
			hasUpper = true
			break
		}
	}
	if !hasUpper {
		return s
	}

	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
