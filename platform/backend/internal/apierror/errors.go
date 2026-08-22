package apierror

import "net/http"

var (
	ErrBadRequest       = &APIError{Code: "BAD_REQUEST", Message: "Invalid request", Status: http.StatusBadRequest}
	ErrValidationFailed = &APIError{Code: "VALIDATION_FAILED", Message: "Validation failed", Status: http.StatusBadRequest}

	ErrCompanyEmailConflict = &APIError{Code: "COMPANY_EMAIL_CONFLICT", Message: "Company email already registered", Status: http.StatusConflict}
	ErrUserEmailConflict    = &APIError{Code: "USER_EMAIL_CONFLICT", Message: "User email already registered", Status: http.StatusConflict}

	ErrInvalidCredentials = &APIError{Code: "INVALID_CREDENTIALS", Message: "Invalid email or password", Status: http.StatusUnauthorized}
	ErrLoginLocked        = &APIError{Code: "LOGIN_LOCKED", Message: "Too many failed login attempts. Try again later.", Status: http.StatusTooManyRequests}

	ErrUnauthorized = &APIError{Code: "UNAUTHORIZED", Message: "Unauthorized", Status: http.StatusUnauthorized}
	ErrForbidden    = &APIError{Code: "FORBIDDEN", Message: "Forbidden", Status: http.StatusForbidden}

	ErrUserDeactivated = &APIError{Code: "USER_DEACTIVATED", Message: "Your account has been deactivated by your company administrator.", Status: http.StatusForbidden}

	ErrInternal = &APIError{Code: "INTERNAL", Message: "Internal server error", Status: http.StatusInternalServerError}
	ErrAuthTemporarilyUnavailable = &APIError{Code: "AUTH_TEMPORARILY_UNAVAILABLE", Message: "Authentication temporarily unavailable", Status: http.StatusServiceUnavailable}

	ErrInvalidResetToken = &APIError{Code: "INVALID_RESET_TOKEN", Message: "This password reset link is invalid or has expired.", Status: http.StatusBadRequest}

	ErrAccountSuspended     = &APIError{Code: "ACCOUNT_SUSPENDED", Message: "Your account has been suspended.", Status: http.StatusForbidden}
	ErrSubscriptionExpired  = &APIError{Code: "SUBSCRIPTION_EXPIRED", Message: "Your subscription has expired.", Status: http.StatusForbidden}
)
