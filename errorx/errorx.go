package errorx

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	_ "unsafe" // for go:linkname

	"github.com/pkg/errors"
)

const (
	// CodeCategory
	CCBadRequest     = http.StatusBadRequest          // 400
	CCUnauthorized   = http.StatusUnauthorized        // 401
	CCForbidden      = http.StatusForbidden           // 403
	CCNotFound       = http.StatusNotFound            // 404
	CCInternalServer = http.StatusInternalServerError // 500
	CCNotImplemented = http.StatusNotImplemented      // 501
	CCUnknown        = 900                            // 900
)

var (
	_              CodeError = (*codeError)(nil)
	codeCombinerMu sync.Mutex
	codeCombiner   CodeCombiner = codeCombiner323{}
)

type (
	// ErrCode is the error code for app
	// 0 indicates success, others indicate failure.
	// It is combined of error category code, platform code, and specific code via CodeCombiner.
	// The default CodeCombiner's rules are as follows:
	// - The first three digits represent the category code, analogous to the http status code.
	// - The next two digits indicate the platform code.
	// - The last three digits indicate the specific code.
	//   For example:
	//     4041001:
	// 	     404 is the error category code
	// 	      10 is the error platform code
	// 	     001 is the error specific code
	ErrCode struct {
		code    int
		message string
	}

	CodeError interface {
		error
		GetErrCode() *ErrCode
		GetCode() int
		GetCategoryCode() int
		GetPlatformCode() int
		GetSpecificCode() int
		GetMessage() string
		GetHTTPStatus() int
		IsErrCode(c *ErrCode) bool
	}

	codeError struct {
		error
		*ErrCode
	}

	CodeCombiner interface {
		Combine(categoryCode, platformCode, specificCode int) int
		Separate(int) (categoryCode, platformCode, specificCode int)
	}

	codeCombiner323 struct{}
)

// SetCodeCombiner changes the defalut CodeCombiner.
func SetCodeCombiner(combiner CodeCombiner) {
	codeCombinerMu.Lock()
	codeCombiner = combiner
	codeCombinerMu.Unlock()
}

// WithCode return error warps with codeError.
// c is the code. err is the real err. formatWithArgs is format string with args.
// For example:
//  WithCode(ErrBadRequest, nil)
//  WithCode(ErrBadRequest, err)
//  WithCode(ErrBadRequest, err, "message")
//  WithCode(ErrBadRequest, err, "message %s", "id")
func WithCode(c *ErrCode, err error, formatWithArgs ...interface{}) error {
	ce := newCodeErrorInternal(c, err)
	if len(formatWithArgs) == 0 {
		return ce
	}

	format, ok := formatWithArgs[0].(string)
	if !ok {
		return ce
	}

	if len(formatWithArgs) == 1 {
		return errors.WithMessage(ce, format)
	}

	return errors.WithMessagef(ce, format, formatWithArgs[1:]...)
}

func AsCodeError(err error) (CodeError, bool) {
	if e := new(codeError); errors.As(err, &e) {
		return e, true
	}
	return nil, false
}

func IsCodeError(err error, c ...*ErrCode) bool {
	ce, ok := AsCodeError(err)
	if !ok {
		return false
	}
	switch len(c) {
	case 0:
		return true
	case 1:
		return ce.IsErrCode(c[0])
	default:
		return false
	}
}

// SeparateCode splits code with category code, platform code and specific code.
func SeparateCode(code int) (categoryCode, platformCode, specificCode int) {
	return codeCombiner.Separate(code)
}

// NewErrCode is create an new *ErrCode, it's only used for global initialization.
func NewErrCode(categoryCode, platformCode, specificCode int, message string) *ErrCode {
	return &ErrCode{
		code:    codeCombiner.Combine(categoryCode, platformCode, specificCode),
		message: message,
	}
}

func TakeCodePriority(fns ...func() *ErrCode) *ErrCode {
	for _, fn := range fns {
		if e := fn(); e != nil {
			return e
		}
	}
	return nil
}

func newCodeErrorInternal(c *ErrCode, err ...error) error {
	var e error
	if len(err) > 0 {
		e = err[0]
	}
	ce := &codeError{
		error:   e,
		ErrCode: c,
	}

	if hasStack(e) {
		return ce
	}
	return errors.WithStack(ce)
}

func (c *ErrCode) GetErrCode() *ErrCode {
	return c
}

func (c *ErrCode) GetCode() int {
	return c.code
}

func (c *ErrCode) GetCategoryCode() int {
	v, _, _ := codeCombiner.Separate(c.GetCode())
	return v
}

func (c *ErrCode) GetPlatformCode() int {
	_, v, _ := codeCombiner.Separate(c.GetCode())
	return v
}

func (c *ErrCode) GetSpecificCode() int {
	_, _, v := codeCombiner.Separate(c.GetCode())
	return v
}

func (c *ErrCode) GetMessage() string {
	return c.message
}

func (c *ErrCode) GetHTTPStatus() int {
	return getErrCodeHTTPStatus(c)
}

func (c *ErrCode) IsErrCode(ec *ErrCode) bool {
	return c == ec
}

func (e *codeError) Error() string {
	return fmt.Sprintf("%d: %s", e.GetCode(), e.GetMessage())
}

func (e *codeError) Cause() error { return e.error }

// Unwrap provides compatibility for Go 1.13 error chains.
func (e *codeError) Unwrap() error { return e.error }

func (e *codeError) Format(s fmt.State, verb rune) {
	switch verb {
	case 'v':
		if s.Flag('+') && e.Cause() != nil {
			_, _ = fmt.Fprintf(s, "%+v\n", e.Cause())
			_, _ = io.WriteString(s, e.Error())
			return
		}
		fallthrough
	case 's':
		_, _ = io.WriteString(s, e.Error())
	case 'q':
		_, _ = fmt.Fprintf(s, "%q", e.Error())
	}
}

func (codeCombiner323) Combine(categoryCode, platformCode, specificCode int) int {
	return categoryCode*100000 + platformCode*1000 + specificCode
}

func (codeCombiner323) Separate(code int) (categoryCode, platformCode, specificCode int) {
	return code / 100000, code / 1000 % 100, code % 1000
}

func getErrCodeHTTPStatus(c *ErrCode) int {
	categoryCode := c.GetCategoryCode()
	if categoryCode == CCUnknown {
		return http.StatusInternalServerError
	}
	return categoryCode
}

func hasStack(err error) bool {
	type stackTracer interface {
		StackTrace() errors.StackTrace
	}

	for err != nil {
		if _, ok := err.(stackTracer); ok {
			return true
		}
		err = errors.Unwrap(err)
	}

	return false
}
