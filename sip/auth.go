package sip

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

func CreateAuthorization() *Authorization {
	auth := &Authorization{
		Algorithm: "MD5",
		Other:     NewParams(),
	}

	return auth
}

// currently only Digest and MD5
type Authorization struct {
	name      string
	mode      string
	Realm     string
	Nonce     string
	Algorithm string
	Username  string
	Password  string
	Uri       string
	Response  string
	Method    string
	Other     Params
}

func (auth *Authorization) Name() string {
	if auth.name == "proxy-authorization" {
		return "Proxy-Authorization"
	}
	return "Authorization"
}

func (auth *Authorization) Copy() Header {
	newAuth := &Authorization{
		name:      auth.Name(),
		mode:      auth.Mode(),
		Realm:     auth.Realm,
		Nonce:     auth.Nonce,
		Algorithm: auth.Algorithm,
		Username:  auth.Username,
		Password:  auth.Password,
		Uri:       auth.Uri,
		Response:  auth.Response,
		Method:    auth.Method,
	}

	if auth.Other != nil {
		newAuth.Other = auth.Other.Copy()
	}

	return newAuth
}

func (auth *Authorization) Equals(Other interface{}) bool {
	if h, ok := Other.(*Authorization); ok {
		return strings.Compare(auth.name, h.name) == 0 &&
			strings.Compare(auth.Realm, h.Realm) == 0 &&
			strings.Compare(auth.Nonce, h.Nonce) == 0 &&
			strings.Compare(auth.Algorithm, h.Algorithm) == 0 &&
			strings.Compare(auth.Username, h.Username) == 0 &&
			strings.Compare(auth.Password, h.Password) == 0 &&
			strings.Compare(auth.Uri, h.Uri) == 0 &&
			strings.Compare(auth.Response, h.Response) == 0 &&
			strings.Compare(auth.Method, h.Method) == 0 &&
			auth.Other.Equals(h.Other)
	}

	return false
}

func (auth *Authorization) SetName(name string) {
	auth.name = name
}

func (auth *Authorization) SetUsername(Username string) {
	auth.Username = Username
}

func (auth *Authorization) SetUri(Uri string) {
	auth.Uri = Uri
}

func (auth *Authorization) SetMethod(Method string) {
	auth.Method = Method
}

func (auth *Authorization) Mode() string {
	if auth.mode == "" {
		return "Digest"
	}
	return auth.mode
}
func (auth *Authorization) SetMode(mode string) {
	auth.mode = mode
}

func (auth *Authorization) SetPassword(Password string) {
	auth.Password = Password
}

func (auth *Authorization) CalcResponse() {
	auth.Response = calcResponse(
		auth.Username,
		auth.Realm,
		auth.Password,
		auth.Method,
		auth.Uri,
		auth.Nonce,
	)
}

func (auth *Authorization) String() string {
	if strings.ToLower(auth.Mode()) != "digest" {
		return fmt.Sprintf(
			`%s: %s algorithm=%s`,
			auth.Name(),
			auth.Mode(),
			auth.Algorithm,
		)
	}

	return fmt.Sprintf(
		`%s: %s username="%s",realm="%s",nonce="%s",uri="%s",response="%s",algorithm=%s`,
		auth.Name(),
		auth.Mode(),
		auth.Username,
		auth.Realm,
		auth.Nonce,
		auth.Uri,
		auth.Response,
		auth.Algorithm,
	)
}

// 解析
func (auth *Authorization) ParseAuthorization(value string) {

	if str := strings.ToLower(value); strings.Contains(str, "capability") {
		auth.SetMode("Capability")
	}

	re := regexp.MustCompile(`([\w]+)="([^"]+)"`)
	matches := re.FindAllStringSubmatch(value, -1)
	for _, match := range matches {
		switch match[1] {
		case "realm":
			auth.Realm = match[2]
		case "algorithm":
			auth.Algorithm = match[2]
		case "nonce":
			auth.Nonce = match[2]
		case "username":
			auth.Username = match[2]
		case "uri":
			auth.Uri = match[2]
		case "response":
			auth.Response = match[2]
		case "method":
			auth.Method = match[2]
		default:
			auth.Other.Add(match[1], String{Str: match[2]})
		}
	}
}

func parseAuthorization(headerName string, headerData string) (headers []Header, err error) {
	auth := CreateAuthorization()
	auth.SetName(headerName)
	auth.ParseAuthorization(headerData)

	headers = []Header{auth}
	return
}

// calculates Authorization Response https://www.ietf.org/rfc/rfc2617.txt
func calcResponse(Username string, Realm string, Password string, Method string, Uri string, Nonce string) string {
	calcA1 := func() string {
		encoder := md5.New()
		encoder.Write([]byte(Username + ":" + Realm + ":" + Password))

		return hex.EncodeToString(encoder.Sum(nil))
	}
	calcA2 := func() string {
		encoder := md5.New()
		encoder.Write([]byte(strings.ToUpper(Method) + ":" + Uri))

		return hex.EncodeToString(encoder.Sum(nil))
	}

	encoder := md5.New()
	encoder.Write([]byte(calcA1() + ":" + Nonce + ":" + calcA2()))

	return hex.EncodeToString(encoder.Sum(nil))
}

func AuthorizeRequest(request Request, Response Response, user, Password MaybeString) error {
	if user == nil {
		return fmt.Errorf("authorize request: user is nil")
	}

	var authenticateHeaderName, authorizeHeaderName string
	if Response.StatusCode() == 401 {
		// on 401 Unauthorized increase request seq num, add Authorization header and send once again
		authenticateHeaderName = "WWW-Authenticate"
		authorizeHeaderName = "Authorization"
	} else {
		// 407 Proxy authentication
		authenticateHeaderName = "Proxy-Authenticate"
		authorizeHeaderName = "Proxy-Authorization"
	}

	if hdrs := Response.GetHeaders(authenticateHeaderName); len(hdrs) > 0 {
		authenticateHeader := hdrs[0].(*GenericHeader)

		auth := CreateAuthorization()
		auth.ParseAuthorization(authenticateHeader.Contents)
		auth.SetMethod(string(request.Method()))
		auth.SetUri(request.Recipient().String())
		auth.SetUsername(user.String())

		if Password != nil {
			auth.SetPassword(Password.String())
		}

		auth.CalcResponse()

		var authorizationHeader *Authorization
		hdrs = request.GetHeaders(authorizeHeaderName)
		if len(hdrs) > 0 {
			authorizationHeader = hdrs[0].(*Authorization)
			authorizationHeader = auth
		} else {
			authorizationHeader = auth
			request.AddHeader(authorizationHeader)
		}
	} else {
		return fmt.Errorf("authorize request: header '%s' not found in Response", authenticateHeaderName)
	}

	if viaHop, ok := request.ViaHop(); ok {
		viaHop.Params.Add("branch", String{Str: GenerateBranch()})
	}

	if cseq, ok := request.CSeq(); ok {
		cseq.SeqNo++
	}

	return nil
}

type Authorized interface {
	AddAuthInfo(request Request, Response Response) error
}

type DefaultAuthorized struct {
	User     MaybeString
	Password MaybeString
}

func (auth *DefaultAuthorized) AddAuthInfo(request Request, Response Response) error {
	return AuthorizeRequest(request, Response, auth.User, auth.Password)
}
