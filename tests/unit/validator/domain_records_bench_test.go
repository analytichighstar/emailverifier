package validatortest

import (
	"emailvalidator/pkg/validator"
	"net"
	"testing"
)

type benchResolver struct {
	hostResults map[string][]string
	mxResults   map[string][]*net.MX
}

func (r *benchResolver) LookupHost(domain string) ([]string, error) {
	return r.hostResults[domain], nil
}

func (r *benchResolver) LookupMX(domain string) ([]*net.MX, error) {
	return r.mxResults[domain], nil
}

func BenchmarkValidateDomainRecordsCold(b *testing.B) {
	resolver := &benchResolver{
		hostResults: map[string][]string{"example.com": {"192.0.2.1"}},
		mxResults: map[string][]*net.MX{
			"example.com": {{Host: "mail.example.com", Pref: 10}},
		},
	}
	cacheManager := validator.NewDomainCacheManager(0)
	domainValidator := validator.NewDomainValidator(resolver, cacheManager)

	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		cacheManager.ClearExpired()
		domainValidator.ValidateDomainRecords("example.com")
	}
}

func BenchmarkValidateDomainRecordsCached(b *testing.B) {
	resolver := &benchResolver{
		hostResults: map[string][]string{"example.com": {"192.0.2.1"}},
		mxResults: map[string][]*net.MX{
			"example.com": {{Host: "mail.example.com", Pref: 10}},
		},
	}
	cacheManager := validator.NewDomainCacheManager(0)
	domainValidator := validator.NewDomainValidator(resolver, cacheManager)
	domainValidator.ValidateDomainRecords("example.com")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		domainValidator.ValidateDomainRecords("example.com")
	}
}
