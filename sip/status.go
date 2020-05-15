package sip

// Originally forked from https://github.com/1lann/go-sip/blob/master/sipnet/status.go
// SIP response status codes.
type StatusCode uint16

const (
	StatusTrying               StatusCode = 100
	StatusRinging              StatusCode = 180
	StatusCallIsBeingForwarded StatusCode = 181
	StatusQueued               StatusCode = 182
	StatusSessionProgress      StatusCode = 183

	StatusOK StatusCode = 200

	StatusMultipleChoices    StatusCode = 300
	StatusMovedPermanently   StatusCode = 301
	StatusMovedTemporarily   StatusCode = 302
	StatusUseProxy           StatusCode = 305
	StatusAlternativeService StatusCode = 380

	StatusBadRequest                  StatusCode = 400
	StatusUnauthorized                StatusCode = 401
	StatusPaymentRequired             StatusCode = 402
	StatusForbidden                   StatusCode = 403
	StatusNotFound                    StatusCode = 404
	StatusMethodNotAllowed            StatusCode = 405
	StatusNotAcceptable               StatusCode = 406
	StatusProxyAuthenticationRequired StatusCode = 407
	StatusRequestTimeout              StatusCode = 408
	StatusGone                        StatusCode = 410
	StatusRequestEntityTooLarge       StatusCode = 413
	StatusRequestURITooLong           StatusCode = 414
	StatusUnsupportedMediaType        StatusCode = 415
	StatusUnsupportedURIScheme        StatusCode = 416
	StatusBadExtension                StatusCode = 420
	StatusExtensionRequired           StatusCode = 421
	StatusIntervalTooBrief            StatusCode = 423
	StatusNoResponse                  StatusCode = 480
	StatusCallTransactionDoesNotExist StatusCode = 481
	StatusLoopDetected                StatusCode = 482
	StatusTooManyHops                 StatusCode = 483
	StatusAddressIncomplete           StatusCode = 484
	StatusAmbiguously                 StatusCode = 485
	StatusBusyHere                    StatusCode = 486
	StatusRequestTerminated           StatusCode = 487
	StatusNotAcceptableHere           StatusCode = 488
	StatusRequestPending              StatusCode = 491
	StatusUndecipherable              StatusCode = 493

	StatusServerInternalError StatusCode = 500
	StatusNotImplemented      StatusCode = 501
	StatusBadGateway          StatusCode = 502
	StatusServiceUnavailable  StatusCode = 503
	StatusServerTimeout       StatusCode = 504
	StatusVersionNotSupported StatusCode = 505
	StatusMessageTooLarge     StatusCode = 513

	StatusBusyEverywhere       StatusCode = 600
	StatusDecline              StatusCode = 603
	StatusDoesNotExistAnywhere StatusCode = 604
	StatusUnacceptable         StatusCode = 606
)

var statusTexts = map[StatusCode]string{
	StatusTrying:                      "Trying",
	StatusRinging:                     "Ringing",
	StatusCallIsBeingForwarded:        "Call Is Being Forwarded",
	StatusQueued:                      "Queued",
	StatusSessionProgress:             "Session Progress",
	StatusOK:                          "OK",
	StatusMultipleChoices:             "Multiple Choices",
	StatusMovedPermanently:            "Moved Permanently",
	StatusMovedTemporarily:            "Moved Temporarily",
	StatusUseProxy:                    "Use Proxy",
	StatusAlternativeService:          "Alternative Service",
	StatusBadRequest:                  "Bad Request",
	StatusUnauthorized:                "Unauthorized",
	StatusPaymentRequired:             "Payment Required",
	StatusForbidden:                   "Forbidden",
	StatusNotFound:                    "Not Found",
	StatusMethodNotAllowed:            "Method Not Allowed",
	StatusNotAcceptable:               "Not Acceptable",
	StatusProxyAuthenticationRequired: "Proxy Authentication Required",
	StatusRequestTimeout:              "Request Timeout",
	StatusGone:                        "Gone",
	StatusRequestEntityTooLarge:       "Request Entity Too Large",
	StatusRequestURITooLong:           "Request-URI Too Long",
	StatusUnsupportedMediaType:        "Unsupported Media Type",
	StatusUnsupportedURIScheme:        "Unsupported URI Scheme",
	StatusBadExtension:                "Bad Extension",
	StatusExtensionRequired:           "Extension Required",
	StatusIntervalTooBrief:            "Interval Too Brief",
	StatusNoResponse:                  "No Response",
	StatusCallTransactionDoesNotExist: "Call/Transaction Does Not Exist",
	StatusLoopDetected:                "Loop Detected",
	StatusTooManyHops:                 "Too Many Hops",
	StatusAddressIncomplete:           "Address Incomplete",
	StatusAmbiguously:                 "Ambiguous",
	StatusBusyHere:                    "Busy Here",
	StatusRequestTerminated:           "Request Terminated",
	StatusNotAcceptableHere:           "Not Acceptable Here",
	StatusRequestPending:              "Request Pending",
	StatusUndecipherable:              "Undecipherable",
	StatusServerInternalError:         "Server Internal Error",
	StatusNotImplemented:              "Not Implemented",
	StatusBadGateway:                  "Bad Gateway",
	StatusServiceUnavailable:          "Service Unavailable",
	StatusServerTimeout:               "Server Timeout",
	StatusVersionNotSupported:         "Version Not Supported",
	StatusMessageTooLarge:             "Message Too Large",
	StatusBusyEverywhere:              "Busy Everywhere",
	StatusDecline:                     "Decline",
	StatusDoesNotExistAnywhere:        "Does Not Exist Anywhere",
	StatusUnacceptable:                "Not Acceptable",
}

// StatusText returns the human readable text representation of a status code.
func StatusText(code StatusCode) string {
	return statusTexts[code]
}
