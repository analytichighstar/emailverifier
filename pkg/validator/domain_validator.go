package validator

import (
	"time"

	"emailvalidator/pkg/monitoring"
)

// DomainValidator handles domain existence validation
type DomainValidator struct {
	resolver     DNSResolver
	cacheManager *DomainCacheManager
}

// NewDomainValidator creates a new instance of DomainValidator
func NewDomainValidator(resolver DNSResolver, cacheManager *DomainCacheManager) *DomainValidator {
	return &DomainValidator{
		resolver:     resolver,
		cacheManager: cacheManager,
	}
}

// ValidateDomainRecords checks MX records first, then falls back to host lookup per RFC 5321.
// Results are cached together to avoid duplicate DNS queries during batch validation.
func (v *DomainValidator) ValidateDomainRecords(domain string) (exists bool, hasMX bool) {
	if exists, hasMX, found := v.cacheManager.GetValidation(domain); found {
		monitoring.RecordCacheOperation("domain_validation", "hit")
		return exists, hasMX
	}
	monitoring.RecordCacheOperation("domain_validation", "miss")

	hasMX = v.lookupMX(domain)
	if hasMX {
		v.cacheManager.SetValidation(domain, true, true)
		return true, true
	}

	start := time.Now()
	_, err := v.resolver.LookupHost(domain)
	monitoring.RecordDNSLookup("host", time.Since(start))
	exists = err == nil

	v.cacheManager.SetValidation(domain, exists, false)
	return exists, false
}

// Validate checks if the domain exists
func (v *DomainValidator) Validate(domain string) bool {
	exists, _ := v.ValidateDomainRecords(domain)
	return exists
}

// ValidateMX checks if the domain has valid MX records
func (v *DomainValidator) ValidateMX(domain string) bool {
	_, hasMX := v.ValidateDomainRecords(domain)
	return hasMX
}

func (v *DomainValidator) lookupMX(domain string) bool {
	start := time.Now()
	mxRecords, err := v.resolver.LookupMX(domain)
	monitoring.RecordDNSLookup("mx", time.Since(start))

	if err != nil || len(mxRecords) == 0 {
		return false
	}

	// Null MX record (RFC 7505)
	if len(mxRecords) == 1 && mxRecords[0].Host == "." {
		return false
	}

	return true
}
