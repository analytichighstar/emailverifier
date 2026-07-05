package service

import (
	"context"
	"sync"
)

// ConcurrentDomainValidationService handles concurrent domain validation operations
type ConcurrentDomainValidationService struct {
	domainValidator DomainValidator
}

// NewConcurrentDomainValidationService creates a new instance of ConcurrentDomainValidationService
func NewConcurrentDomainValidationService(validator DomainValidator) *ConcurrentDomainValidationService {
	return &ConcurrentDomainValidationService{
		domainValidator: validator,
	}
}

// ValidateDomainConcurrently runs domain validation checks concurrently.
// DNS lookups are unified into a single cached call; disposable checks run in parallel.
func (s *ConcurrentDomainValidationService) ValidateDomainConcurrently(ctx context.Context, domain string) (exists, hasMX, isDisposable bool) {
	select {
	case <-ctx.Done():
		return false, false, false
	default:
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		exists, hasMX = s.domainValidator.ValidateDomainRecords(domain)
	}()

	go func() {
		defer wg.Done()
		isDisposable = s.domainValidator.IsDisposable(domain)
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return false, false, false
	case <-done:
		return exists, hasMX, isDisposable
	}
}
