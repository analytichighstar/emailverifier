package service

import (
	"context"
	"runtime"
	"strings"
	"sync"

	"emailvalidator/internal/model"
	"emailvalidator/internal/utils"
)

const maxConcurrentDomainLookups = 25

// BatchValidationService handles batch email validation operations
type BatchValidationService struct {
	emailRuleValidator   EmailRuleValidator
	domainValidationSvc  DomainValidationService
	metricsCollector     MetricsCollector
	maxConcurrentWorkers int
}

// NewBatchValidationService creates a new instance of BatchValidationService
func NewBatchValidationService(
	ruleValidator EmailRuleValidator,
	domainValidationSvc DomainValidationService,
	metricsCollector MetricsCollector,
) *BatchValidationService {
	return &BatchValidationService{
		emailRuleValidator:   ruleValidator,
		domainValidationSvc:  domainValidationSvc,
		metricsCollector:     metricsCollector,
		maxConcurrentWorkers: runtime.NumCPU() * 4,
	}
}

// ValidateEmails performs validation on multiple email addresses concurrently
func (s *BatchValidationService) ValidateEmails(emails []string) model.BatchValidationResponse {
	if len(emails) == 0 {
		return model.BatchValidationResponse{Results: []model.EmailValidationResponse{}}
	}

	emailsByDomain := s.groupEmailsByDomain(emails)
	domainResults := s.processDomainValidations(emailsByDomain)
	return s.processEmails(emails, domainResults)
}

func (s *BatchValidationService) groupEmailsByDomain(emails []string) map[string][]string {
	emailsByDomain := make(map[string][]string)
	for _, email := range emails {
		if email == "" {
			continue
		}

		parts := strings.Split(email, "@")
		if len(parts) != 2 {
			continue
		}

		domain := parts[1]
		emailsByDomain[domain] = append(emailsByDomain[domain], email)
	}
	return emailsByDomain
}

func (s *BatchValidationService) processDomainValidations(emailsByDomain map[string][]string) map[string]struct {
	DomainExists bool
	MXRecords    bool
	IsDisposable bool
} {
	ctx := context.Background()
	domainResults := make(map[string]struct {
		DomainExists bool
		MXRecords    bool
		IsDisposable bool
	}, len(emailsByDomain))

	var wg sync.WaitGroup
	sem := make(chan struct{}, maxConcurrentDomainLookups)
	var mu sync.Mutex

	for domain := range emailsByDomain {
		wg.Add(1)
		go func(d string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			exists, hasMX, isDisposable := s.domainValidationSvc.ValidateDomainConcurrently(ctx, d)

			mu.Lock()
			domainResults[d] = struct {
				DomainExists bool
				MXRecords    bool
				IsDisposable bool
			}{exists, hasMX, isDisposable}
			mu.Unlock()
		}(domain)
	}

	wg.Wait()
	return domainResults
}

func (s *BatchValidationService) processEmails(
	emails []string,
	domainResults map[string]struct {
		DomainExists bool
		MXRecords    bool
		IsDisposable bool
	},
) model.BatchValidationResponse {
	results := make([]model.EmailValidationResponse, len(emails))
	jobs := make(chan int, len(emails))

	workerCount := utils.MinInt(len(emails), s.maxConcurrentWorkers)
	var wg sync.WaitGroup
	wg.Add(workerCount)

	for i := 0; i < workerCount; i++ {
		go func() {
			defer wg.Done()
			for index := range jobs {
				results[index] = s.validateSingleEmail(emails[index], domainResults)
			}
		}()
	}

	for i := range emails {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	return model.BatchValidationResponse{Results: results}
}

func (s *BatchValidationService) validateSingleEmail(
	email string,
	domainResults map[string]struct {
		DomainExists bool
		MXRecords    bool
		IsDisposable bool
	},
) model.EmailValidationResponse {
	response := model.EmailValidationResponse{
		Email:       email,
		Validations: model.ValidationResults{},
	}

	if email == "" {
		response.Status = model.ValidationStatusMissingEmail
		return response
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		response.Status = model.ValidationStatusInvalidFormat
		return response
	}

	domain := parts[1]
	response.Validations.Syntax = s.emailRuleValidator.ValidateSyntax(email)
	if !response.Validations.Syntax {
		response.Status = model.ValidationStatusInvalidFormat
		return response
	}

	domainValidation := domainResults[domain]
	response.Validations.DomainExists = domainValidation.DomainExists
	response.Validations.MXRecords = domainValidation.MXRecords
	response.Validations.IsDisposable = domainValidation.IsDisposable
	response.Validations.IsRoleBased = s.emailRuleValidator.IsRoleBased(email)
	response.Validations.MailboxExists = response.Validations.MXRecords

	if canonicalEmail := s.emailRuleValidator.DetectAlias(email); canonicalEmail != "" && canonicalEmail != email {
		response.AliasOf = canonicalEmail
	}

	validationMap := map[string]bool{
		"syntax":         response.Validations.Syntax,
		"domain_exists":  response.Validations.DomainExists,
		"mx_records":     response.Validations.MXRecords,
		"mailbox_exists": response.Validations.MailboxExists,
		"is_disposable":  response.Validations.IsDisposable,
		"is_role_based":  response.Validations.IsRoleBased,
	}
	response.Score = s.emailRuleValidator.CalculateScore(validationMap)
	response.Status = s.determineValidationStatus(&response)

	return response
}

func (s *BatchValidationService) determineValidationStatus(response *model.EmailValidationResponse) model.ValidationStatus {
	switch {
	case !response.Validations.DomainExists:
		return model.ValidationStatusInvalidDomain
	case !response.Validations.MXRecords:
		response.Score = 40
		return model.ValidationStatusNoMXRecords
	case response.Validations.IsDisposable:
		return model.ValidationStatusDisposable
	case response.Score >= 90:
		return model.ValidationStatusValid
	case response.Score >= 70:
		return model.ValidationStatusProbablyValid
	default:
		return model.ValidationStatusInvalid
	}
}
