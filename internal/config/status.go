package config

import (
	"net/http"
	"reflect"

	"github.com/go-playground/validator"
)

var (
	AllHttpStatuses = []int{
		// 1xx
		http.StatusContinue,
		http.StatusSwitchingProtocols,
		http.StatusProcessing,
		http.StatusEarlyHints,

		// 2xx
		http.StatusOK,
		http.StatusCreated,
		http.StatusAccepted,
		http.StatusNonAuthoritativeInfo,
		http.StatusNoContent,
		http.StatusResetContent,
		http.StatusPartialContent,
		http.StatusMultiStatus,
		http.StatusAlreadyReported,
		http.StatusIMUsed,

		// 3xx
		http.StatusMultipleChoices,
		http.StatusMovedPermanently,
		http.StatusFound,
		http.StatusSeeOther,
		http.StatusNotModified,
		http.StatusUseProxy,
		http.StatusTemporaryRedirect,
		http.StatusPermanentRedirect,

		// 4xx
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusPaymentRequired,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusNotAcceptable,
		http.StatusProxyAuthRequired,
		http.StatusRequestTimeout,
		http.StatusConflict,
		http.StatusGone,
		http.StatusLengthRequired,
		http.StatusPreconditionFailed,
		http.StatusRequestEntityTooLarge,
		http.StatusRequestURITooLong,
		http.StatusUnsupportedMediaType,
		http.StatusRequestedRangeNotSatisfiable,
		http.StatusExpectationFailed,
		http.StatusTeapot,
		http.StatusMisdirectedRequest,
		http.StatusUnprocessableEntity,
		http.StatusLocked,
		http.StatusFailedDependency,
		http.StatusTooEarly,
		http.StatusUpgradeRequired,
		http.StatusPreconditionRequired,
		http.StatusTooManyRequests,
		http.StatusRequestHeaderFieldsTooLarge,
		http.StatusUnavailableForLegalReasons,

		// 5xx
		http.StatusInternalServerError,
		http.StatusNotImplemented,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
		http.StatusHTTPVersionNotSupported,
		http.StatusVariantAlsoNegotiates,
		http.StatusInsufficientStorage,
		http.StatusLoopDetected,
		http.StatusNotExtended,
		http.StatusNetworkAuthenticationRequired,
	}
)

type HTTPStatusCodeList struct {
	StatusCodes []int
}

func ValidateStatusList(fl validator.FieldLevel) bool {
	field := fl.Field()

	switch field.Kind() {
	case reflect.Int, reflect.Int64, reflect.Float64:
		confStatusCode := int(field.Int())
		statusInvalid := true
		for _, httpStatus := range AllHttpStatuses {
			if confStatusCode == httpStatus {
				statusInvalid = false
				break
			}
		}
		if statusInvalid {
			return false
		}
	case reflect.Slice:
		for i := 0; i < field.Len(); i++ {
			confStatusCode := int(field.Index(i).Int())
			statusInvalid := true
			for _, httpStatus := range AllHttpStatuses {
				if confStatusCode == httpStatus {
					statusInvalid = false
					break
				}
			}
			if statusInvalid {
				return false
			}
		}
	default:
		return false
	}
	return true
}
