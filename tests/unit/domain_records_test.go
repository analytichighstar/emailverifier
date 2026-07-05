package unit

import (
	"emailvalidator/pkg/validator"
	"net"
	"testing"
	"time"
)

func TestValidateDomainRecordsCachesResults(t *testing.T) {
	mockResolver := &MockResolver{
		HostResults: map[string][]string{
			"example.com": {"192.0.2.1"},
		},
		MXResults: map[string][]*net.MX{
			"example.com": {{Host: "mail.example.com", Pref: 10}},
		},
	}

	cacheManager := validator.NewDomainCacheManager(time.Hour)
	domainValidator := validator.NewDomainValidator(mockResolver, cacheManager)

	exists1, hasMX1 := domainValidator.ValidateDomainRecords("example.com")
	exists2, hasMX2 := domainValidator.ValidateDomainRecords("example.com")

	if !exists1 || !hasMX1 {
		t.Fatalf("first lookup = (%v, %v), want (true, true)", exists1, hasMX1)
	}
	if exists1 != exists2 || hasMX1 != hasMX2 {
		t.Fatalf("cached lookup mismatch: (%v,%v) vs (%v,%v)", exists2, hasMX2, exists1, hasMX1)
	}
}

func TestValidateDomainRecordsMXOnlySkipsHostLookup(t *testing.T) {
	hostLookups := 0
	mockResolver := &trackingResolver{
		MockResolver: MockResolver{
			MXResults: map[string][]*net.MX{
				"mx-only.com": {{Host: "mail.mx-only.com", Pref: 10}},
			},
		},
		onHostLookup: func() { hostLookups++ },
	}

	cacheManager := validator.NewDomainCacheManager(time.Hour)
	domainValidator := validator.NewDomainValidator(mockResolver, cacheManager)

	exists, hasMX := domainValidator.ValidateDomainRecords("mx-only.com")
	if !exists || !hasMX {
		t.Fatalf("ValidateDomainRecords = (%v, %v), want (true, true)", exists, hasMX)
	}
	if hostLookups != 0 {
		t.Fatalf("host lookups = %d, want 0 when MX records are valid", hostLookups)
	}
}

func TestValidateDomainRecordsFallsBackToHostLookup(t *testing.T) {
	mockResolver := &MockResolver{
		HostResults: map[string][]string{
			"a-only.com": {"192.0.2.1"},
		},
		MXResults: map[string][]*net.MX{
			"a-only.com": {},
		},
	}

	cacheManager := validator.NewDomainCacheManager(time.Hour)
	domainValidator := validator.NewDomainValidator(mockResolver, cacheManager)

	exists, hasMX := domainValidator.ValidateDomainRecords("a-only.com")
	if !exists || hasMX {
		t.Fatalf("ValidateDomainRecords = (%v, %v), want (true, false)", exists, hasMX)
	}
}

type trackingResolver struct {
	MockResolver
	onHostLookup func()
}

func (r *trackingResolver) LookupHost(domain string) ([]string, error) {
	if r.onHostLookup != nil {
		r.onHostLookup()
	}
	return r.MockResolver.LookupHost(domain)
}

func (r *trackingResolver) LookupMX(domain string) ([]*net.MX, error) {
	return r.MockResolver.LookupMX(domain)
}
