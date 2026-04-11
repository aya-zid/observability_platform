package apierror

import "errors"

// APIError is a structured, typed error that carries an HTTP status and a stable code.
// Status is intentionally omitted from JSON responses; handlers use it to set HTTP status.
type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *APIError) Error() string {
	return e.Message
}

func As(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr != nil {
		return apiErr, true
	}
	return nil, false
}

