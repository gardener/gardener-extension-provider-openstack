// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper

import (
	"regexp"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var (
	unauthenticatedRegexp               = regexp.MustCompile(`(?i)(Authentication failed|invalid character|invalid_client|cannot fetch token|InvalidSecretAccessKey|Could not find Application Credential)`)
	unauthorizedRegexp                  = regexp.MustCompile(`(?i)(Unauthorized|SignatureDoesNotMatch|invalid_grant|Authorization Profile was not found|no active subscriptions|not authorized|AccessDenied|PolicyNotAuthorized)`)
	quotaExceededRegexp                 = regexp.MustCompile(`(?i)((?:^|[^t]|(?:[^s]|^)t|(?:[^e]|^)st|(?:[^u]|^)est|(?:[^q]|^)uest|(?:[^e]|^)quest|(?:[^r]|^)equest)LimitExceeded|Quotas|Quota.*exceeded|exceeded quota|Quota has been met|QUOTA_EXCEEDED|Maximum number of ports exceeded|VolumeSizeExceedsAvailableQuota)`)
	rateLimitsExceededRegexp            = regexp.MustCompile(`(?i)(RequestLimitExceeded|Throttling|Too many requests)`)
	dependenciesRegexp                  = regexp.MustCompile(`(?i)(PendingVerification|Access Not Configured|accessNotConfigured|DependencyViolation|OptInRequired|Conflict|inactive billing state|timeout while waiting for state to become|InvalidCidrBlock|already busy for|A resource with the ID|There are not enough hosts available|No Router found|There are one or more ports still in use on the network)`)
	retryableDependenciesRegexp         = regexp.MustCompile(`(?i)(RetryableError|internal server error)`)
	resourcesDepletedRegexp             = regexp.MustCompile(`(?i)(not available in the current hardware cluster|out of stock|ResourceExhausted)`)
	configurationProblemRegexp          = regexp.MustCompile(`(?i)(missing expected router|Policy doesn't allow .* to be performed|overlaps with cidr|not supported in your requested Availability Zone|notFound|Invalid value|violates constraint|no attached internet gateway found|Your query returned no results|invalid VPC attributes|unrecognized feature gate|runtime-config invalid key|strict decoder error|not allowed to configure an unsupported|error during apply of object .* is invalid:|duplicate zones|overlapping zones)`)
	retryableConfigurationProblemRegexp = regexp.MustCompile(`(?i)(is misconfigured and requires zero voluntary evictions|SDK.CanNotResolveEndpoint|The requested configuration is currently not supported)`)

	// KnownCodes maps Gardener error codes to respective regex.
	KnownCodes = map[gardencorev1beta1.ErrorCode]func(string) bool{
		gardencorev1beta1.ErrorInfraUnauthenticated:          unauthenticatedRegexp.MatchString,
		gardencorev1beta1.ErrorInfraUnauthorized:             unauthorizedRegexp.MatchString,
		gardencorev1beta1.ErrorInfraQuotaExceeded:            quotaExceededRegexp.MatchString,
		gardencorev1beta1.ErrorInfraRateLimitsExceeded:       rateLimitsExceededRegexp.MatchString,
		gardencorev1beta1.ErrorInfraDependencies:             dependenciesRegexp.MatchString,
		gardencorev1beta1.ErrorRetryableInfraDependencies:    retryableDependenciesRegexp.MatchString,
		gardencorev1beta1.ErrorInfraResourcesDepleted:        resourcesDepletedRegexp.MatchString,
		gardencorev1beta1.ErrorConfigurationProblem:          configurationProblemRegexp.MatchString,
		gardencorev1beta1.ErrorRetryableConfigurationProblem: retryableConfigurationProblemRegexp.MatchString,
	}
)
