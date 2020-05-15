package sip

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/zenghr0820/gsip/logger"
	"github.com/zenghr0820/gsip/utils"
)

// Whitespace recognised by SIP protocol.
const cAbaftWs = " \t"

// ==================================
// 		INTERFACE 头部接口定义
// ==================================

// SIP 信息头部定义
type Header interface {
	// 头部标题
	Name() string
	// 复制
	Copy() Header
	String() string
	Equals(other interface{}) bool
}

// Params 定义
type Params interface {
	Get(key string) (MaybeString, bool)
	Add(key string, val MaybeString) Params
	Copy() Params
	Equals(params interface{}) bool
	ToString(sep uint8) string
	String() string
	Length() int
	Items() map[string]MaybeString
	Keys() []string
	Has(key string) bool
}

// Uri 头部定义
type Uri interface {
	// Determine if the two URIs are equal according to the rules in RFC 3261 s. 19.1.4.
	Equals(other interface{}) bool
	String() string
	Copy() Uri

	IsEncrypted() bool
	SetEncrypted(flag bool)
	User() MaybeString
	SetUser(user MaybeString)
	Password() MaybeString
	SetPassword(pass MaybeString)
	Domain() Addr
	SetDomain(domain Addr)
	UriParams() Params
	SetUriParams(params Params)
	Headers() Params
	SetHeaders(params Params)
	// Return true if and only if the URI is the special wildcard URI '*'; that is, if it is
	// a WildcardUri struct.
	IsWildcard() bool
}

// A URI from a schema suitable for inclusion in a Contact: header.
// The only such URIs are sip/sips URIs and the special wildcard URI '*'.
// hold this interface to not break other code
type ContactUri interface {
	Uri
}

// ================================================================================
// 									IMPLEMENTATION 接口实现
// ================================================================================

// ==========================
// 		headers 头部实现
// ==========================
// headers is a struct with methods to work with SIP headers.
type headers struct {
	// The logical SIP headers attached to this message.
	headers map[string][]Header
	// The order the headers should be displayed in.
	headerOrder []string
}

func newHeaders(hers []Header) *headers {
	hs := new(headers)
	hs.headers = make(map[string][]Header)
	hs.headerOrder = make([]string, 0)
	for _, header := range hers {
		hs.AddHeader(header)
	}
	return hs
}

// 生成请求默认头部
func DefaultHeader() *headers {
	hs := new(headers)
	hs.headers = make(map[string][]Header)
	hs.headerOrder = make([]string, 0)

	// 生成 via from to Call-ID Cseq Max-Forwards

	hs.AddHeader(DefaultViaHeader())  // via
	hs.AddHeader(DefaultFromHeader()) // from
	hs.AddHeader(DefaultToHeader())   // to
	//hs.AddHeader(DefaultFromHeader())  // from
	//hs.AddHeader(DefaultToHeader())    // to
	hs.AddHeader(DefaultCallID())      // Call-Id
	hs.AddHeader(DefaultCSeq())        // Cseq
	hs.AddHeader(DefaultMaxForwards()) // Max-Forwards

	return hs
}

func (hs headers) String() string {
	buffer := bytes.Buffer{}
	// Construct each header in turn and add it to the message.
	for typeIdx, name := range hs.headerOrder {
		headers := hs.headers[name]
		for idx, header := range headers {
			buffer.WriteString(header.String())
			if typeIdx < len(hs.headerOrder) || idx < len(headers) {
				buffer.WriteString("\r\n")
			}
		}
	}
	return buffer.String()
}

func (hs *headers) AddHeaderString(headName string, value string) error {
	headers, err := ParseHeader(headName+":"+value, nil)
	if err != nil {
		return err
	}
	// 将头存储在消息对象中
	for _, header := range headers {
		hs.AddHeader(header)
	}

	return nil
}

// Add the given header.
func (hs *headers) AddHeader(header Header) {
	name := strings.ToLower(header.Name())
	if headerList, ok := hs.headers[name]; ok {
		a := headerList[0]
		logger.Error(headerList[0])
		logger.Error(a)
		logger.Error(headerList[0].Equals(Header(nil)))
		logger.Error(headerList[0].String())
		if len(headerList) > 0 && !headerList[0].Equals(nil) {
			hs.headers[name] = append(headerList, header)
		} else {
			hs.headers[name] = []Header{header}
		}
	} else {
		hs.headers[name] = []Header{header}
		hs.headerOrder = append(hs.headerOrder, name)
	}
}

// replace header
func (hs *headers) ReplaceHeader(header Header) {
	name := strings.ToLower(header.Name())
	hs.headers[name] = []Header{header}
	if _, ok := hs.headers[name]; !ok {
		hs.headerOrder = append(hs.headerOrder, name)
	}
}

// AddFrontHeader adds header to the front of header list
// if there is no header has h's name, add h to the font of all headers
// if there are some headers have h's name, add h to front of the sublist
func (hs *headers) PrependHeader(header Header) {
	name := strings.ToLower(header.Name())
	if hers, ok := hs.headers[name]; ok {
		hs.headers[name] = append([]Header{header}, hers...)
	} else {
		hs.headers[name] = []Header{header}
		newOrder := make([]string, 1, len(hs.headerOrder)+1)
		newOrder[0] = name
		hs.headerOrder = append(newOrder, hs.headerOrder...)
	}
}

func (hs *headers) PrependHeaderAfter(header Header, afterName string) {
	headerName := strings.ToLower(header.Name())
	afterName = strings.ToLower(afterName)
	if _, ok := hs.headers[afterName]; ok {
		afterIdx := -1
		headerIdx := -1
		for i, name := range hs.headerOrder {
			if name == afterName {
				afterIdx = i
			}
			if name == headerName {
				headerIdx = i
			}
		}

		if headerIdx == -1 {
			hs.headers[headerName] = []Header{header}
			newOrder := make([]string, 0)
			newOrder = append(newOrder, hs.headerOrder[:afterIdx+1]...)
			newOrder = append(newOrder, headerName)
			newOrder = append(newOrder, hs.headerOrder[afterIdx+1:]...)
			hs.headerOrder = newOrder
		} else {
			hs.headers[headerName] = append([]Header{header}, hs.headers[headerName]...)
			newOrder := make([]string, 0)
			if afterIdx < headerIdx {
				newOrder = append(newOrder, hs.headerOrder[:afterIdx+1]...)
				newOrder = append(newOrder, headerName)
				newOrder = append(newOrder, hs.headerOrder[afterIdx+1:headerIdx]...)
				newOrder = append(newOrder, hs.headerOrder[headerIdx+1:]...)
			} else {
				newOrder = append(newOrder, hs.headerOrder[:headerIdx]...)
				newOrder = append(newOrder, hs.headerOrder[headerIdx+1:afterIdx+1]...)
				newOrder = append(newOrder, headerName)
				newOrder = append(newOrder, hs.headerOrder[afterIdx+1:]...)
			}
			hs.headerOrder = newOrder
		}
	} else {
		hs.PrependHeader(header)
	}
}

// Gets some headers.
func (hs *headers) Headers() []Header {
	hers := make([]Header, 0)
	for _, key := range hs.headerOrder {
		hers = append(hers, hs.headers[key]...)
	}

	return hers
}

func (hs *headers) GetHeaderString(name string) []string {
	name = strings.ToLower(name)
	var values []string
	if hs.headers == nil {
		hs.headers = map[string][]Header{}
		hs.headerOrder = []string{}
	}
	if headers, ok := hs.headers[name]; ok {
		for _, key := range headers {
			s := strings.Split(key.String(), fmt.Sprintf("%s:", key.Name()))
			fmt.Println(s)
			values = append(values, strings.TrimSpace(s[1]))
		}
		return values
	}

	return []string{}
}
func (hs *headers) GetHeaders(name string) []Header {
	name = strings.ToLower(name)
	if hs.headers == nil {
		hs.headers = map[string][]Header{}
		hs.headerOrder = []string{}
	}
	if headers, ok := hs.headers[name]; ok {
		return headers
	}

	return []Header{}
}

func (hs *headers) DelHeader(names ...string) {
	if len(names) > 0 {
		for _, name := range names {
			name = strings.ToLower(name)
			delete(hs.headers, name)

			// update order slice
			for idx, entry := range hs.headerOrder {
				if entry == name {
					hs.headerOrder = append(hs.headerOrder[:idx], hs.headerOrder[idx+1:]...)
					break
				}
			}
		}
	} else {
		hs.headers = make(map[string][]Header)
		hs.headerOrder = make([]string, 0)
	}

}

// CloneHeaders returns all cloned headers in slice.
func (hs *headers) CloneHeaders() []Header {
	hers := make([]Header, 0)
	for _, header := range hs.Headers() {
		hers = append(hers, header.Copy())
	}

	return hers
}

func (hs *headers) CallID() (*CallID, bool) {
	hers := hs.GetHeaders("Call-ID")
	if len(hers) == 0 {
		return nil, false
	}
	callId, ok := hers[0].(*CallID)
	if !ok {
		return nil, false
	}
	return callId, true
}

func (hs *headers) Expires() (*Expires, bool) {
	hers := hs.GetHeaders("Expires")
	if len(hers) == 0 {
		return nil, false
	}
	expire, ok := hers[0].(*Expires)
	if !ok {
		return nil, false
	}
	return expire, true
}

func (hs *headers) Authorization() *Authorization {
	hers := hs.GetHeaders("Authorization")
	if len(hers) == 0 {
		return nil
	}
	auth, ok := hers[0].(*Authorization)
	if !ok {
		return nil
	}

	return auth
}

func (hs *headers) Via() (ViaHeader, bool) {
	hers := hs.GetHeaders("Via")
	if len(hers) == 0 {
		return nil, false
	}
	via, ok := (hers[0]).(ViaHeader)
	if !ok {
		return nil, false
	}

	return via, true
}

func (hs *headers) ViaHop() (*ViaHop, bool) {
	via, ok := hs.Via()
	if !ok {
		return nil, false
	}
	hops := []*ViaHop(via)
	if len(hops) == 0 {
		return nil, false
	}

	return hops[0], true
}

func (hs *headers) From() (*FromHeader, bool) {
	hers := hs.GetHeaders("From")
	if len(hers) == 0 {
		return nil, false
	}
	from, ok := hers[0].(*FromHeader)
	if !ok {
		return nil, false
	}
	return from, true
}

func (hs *headers) To() (*ToHeader, bool) {
	hers := hs.GetHeaders("To")
	if len(hers) == 0 {
		return nil, false
	}
	to, ok := hers[0].(*ToHeader)
	if !ok {
		return nil, false
	}
	return to, true
}

func (hs *headers) CSeq() (*CSeq, bool) {
	hers := hs.GetHeaders("CSeq")
	if len(hers) == 0 {
		return nil, false
	}
	cseq, ok := hers[0].(*CSeq)
	if !ok {
		return nil, false
	}
	return cseq, true
}

func (hs *headers) ContentLength() (*ContentLength, bool) {
	hers := hs.GetHeaders("Content-Length")
	if len(hers) == 0 {
		return nil, false
	}
	contentLength, ok := hers[0].(*ContentLength)
	if !ok {
		return nil, false
	}
	return contentLength, true
}

func (hs *headers) ContentType() (*ContentType, bool) {
	hers := hs.GetHeaders("Content-Type")
	if len(hers) == 0 {
		return nil, false
	}
	contentType, ok := hers[0].(*ContentType)
	if !ok {
		return nil, false
	}
	return contentType, true
}

func (hs *headers) Contact() (*ContactHeader, bool) {
	hers := hs.GetHeaders("Contact")
	if len(hers) == 0 {
		return nil, false
	}
	contactHeader, ok := hers[0].(*ContactHeader)
	if !ok {
		return nil, false
	}
	return contactHeader, true
}

// ==========================
// 		headerParams 实现
// ==========================
type headerParams struct {
	params     map[string]MaybeString
	paramOrder []string
}

// Create an empty set of parameters.
func NewParams() Params {
	return &headerParams{
		params:     make(map[string]MaybeString),
		paramOrder: []string{},
	}
}

// Returns the entire parameter map.
func (params *headerParams) Items() map[string]MaybeString {
	return params.params
}

// Returns a slice of keys, in order.
func (params *headerParams) Keys() []string {
	return params.paramOrder
}

// Returns the requested parameter value.
func (params *headerParams) Get(key string) (MaybeString, bool) {
	v, ok := params.params[key]
	return v, ok
}

// Put a new parameter.
func (params *headerParams) Add(key string, val MaybeString) Params {
	// Add param to order list if new.
	if _, ok := params.params[key]; !ok {
		params.paramOrder = append(params.paramOrder, key)
	}

	// Set param value.
	params.params[key] = val

	// Return the params so calls can be chained.
	return params
}

func (params *headerParams) Has(key string) bool {
	_, ok := params.params[key]

	return ok
}

// Copy a list of params.
func (params *headerParams) Copy() Params {
	dup := NewParams()
	for _, key := range params.Keys() {
		if val, ok := params.Get(key); ok {
			dup.Add(key, val)
		}
	}

	return dup
}

// Render params to a string.
// Note that this does not escape special characters, this should already have been done before calling this method.
func (params *headerParams) ToString(sep uint8) string {
	var buffer bytes.Buffer
	first := true

	for _, key := range params.Keys() {
		val, ok := params.Get(key)

		if !ok {
			continue
		}

		if !first {
			buffer.WriteString(fmt.Sprintf("%c", sep))
		}
		first = false

		buffer.WriteString(fmt.Sprintf("%s", key))

		if val != nil {
			if strings.ContainsAny(val.String(), cAbaftWs) {
				buffer.WriteString(fmt.Sprintf("=\"%s\"", val.String()))
			} else {
				buffer.WriteString(fmt.Sprintf("=%s", val.String()))
			}
		}
	}

	return buffer.String()
}

// String returns params joined with '&' char.
func (params *headerParams) String() string {
	return params.ToString('&')
}

// Returns number of params.
func (params *headerParams) Length() int {
	return len(params.params)
}

// Check if two maps of parameters are equal in the sense of having the same keys with the same values.
// This does not rely on any ordering of the keys of the map in memory.
func (params *headerParams) Equals(other interface{}) bool {
	q, ok := other.(*headerParams)
	if !ok {
		return false
	}

	if params.Length() == 0 && q.Length() == 0 {
		return true
	}

	if params.Length() != q.Length() {
		return false
	}

	for key, pVal := range params.Items() {
		qVal, ok := q.Get(key)
		if !ok {
			return false
		}
		if pVal != qVal {
			return false
		}
	}

	return true
}

// ======================
// 		Accept 头部实现
// ======================
type Accept string

func (ct Accept) String() string { return "Accept: " + string(ct) }

func (ct *Accept) Name() string { return "Accept" }

func (ct *Accept) Copy() Header { return ct }

func (ct *Accept) Equals(other interface{}) bool {
	if h, ok := other.(Accept); ok {
		return *ct == h
	}
	if h, ok := other.(*Accept); ok {
		return *ct == *h
	}

	return false
}

// =============================
// 		AllowHeader 头部实现
// =============================
type AllowHeader []RequestMethod

func (allow AllowHeader) String() string {
	parts := make([]string, 0)
	for _, method := range allow {
		parts = append(parts, string(method))
	}

	return fmt.Sprintf("Allow: %s", strings.Join(parts, ", "))
}

func (allow AllowHeader) Name() string { return "Allow" }

func (allow AllowHeader) Copy() Header {
	newAllow := make(AllowHeader, len(allow))
	copy(newAllow, allow)

	return newAllow
}

func (allow AllowHeader) Equals(other interface{}) bool {
	if h, ok := other.(AllowHeader); ok {
		if len(allow) != len(h) {
			return false
		}

		for i, v := range allow {
			if v != h[i] {
				return false
			}
		}

		return true
	}

	return false
}

//=======================
// 		CSeq 头部实现
// ======================
type CSeq struct {
	SeqNo      uint32
	MethodName RequestMethod
}

func DefaultCSeq() *CSeq {
	return &CSeq{
		SeqNo:      1,
		MethodName: INVITE,
	}
}

func (seq *CSeq) String() string {
	return fmt.Sprintf("CSeq: %d %s", seq.SeqNo, seq.MethodName)
}

func (seq *CSeq) Name() string { return "CSeq" }

func (seq *CSeq) Copy() Header {
	return &CSeq{
		SeqNo:      seq.SeqNo,
		MethodName: seq.MethodName,
	}
}

func (seq *CSeq) Equals(other interface{}) bool {
	if h, ok := other.(*CSeq); ok {
		return seq.SeqNo == h.SeqNo &&
			seq.MethodName == h.MethodName
	}

	return false
}

//=======================
// 		CallID 头部实现
// ======================
type CallID string

func DefaultCallID() *CallID {
	callId := CallID(utils.RandString(11, true))
	return &callId
}

func (callId *CallID) String() string {
	return "Call-ID: " + string(*callId)
}

func (callId *CallID) Name() string { return "Call-ID" }

func (callId *CallID) Copy() Header {
	return callId
}

func (callId *CallID) Equals(other interface{}) bool {
	if h, ok := other.(CallID); ok {
		return *callId == h
	}
	if h, ok := other.(*CallID); ok {
		return *callId == *h
	}

	return false
}

//=======================
// 		Contact 头部实现
// ======================
type ContactHeader struct {
	// The display name from the header, may be omitted.
	DisplayName MaybeString
	Address     ContactUri
	// Any parameters present in the header.
	Params Params
}

func (contact *ContactHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("Contact: ")

	if displayName, ok := contact.DisplayName.(String); ok && displayName.String() != "" {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", displayName))
	}

	switch contact.Address.(type) {
	case *WildcardUri:
		// Treat the Wildcard URI separately as it must not be contained in < > angle brackets.
		buffer.WriteString("*")
	default:
		buffer.WriteString(fmt.Sprintf("<%s>", contact.Address.String()))
	}

	if (contact.Params != nil) && (contact.Params.Length() > 0) {
		buffer.WriteString(";")
		buffer.WriteString(contact.Params.ToString(';'))
	}

	return buffer.String()
}

func (contact *ContactHeader) Name() string { return "Contact" }

// Copy the header.
func (contact *ContactHeader) Copy() Header {
	return &ContactHeader{
		DisplayName: contact.DisplayName,
		Address:     contact.Address.Copy(),
		Params:      contact.Params.Copy(),
	}
}

func (contact *ContactHeader) Equals(other interface{}) bool {
	if h, ok := other.(*ContactHeader); ok {
		return contact.DisplayName == h.DisplayName &&
			contact.Address.Equals(h.Address) &&
			contact.Params.Equals(h.Params)
	}

	return false
}

//=============================
// 		ContentLength 实现
// ============================
type ContentLength uint32

func (contentLength ContentLength) String() string {
	return fmt.Sprintf("Content-Length: %d", int(contentLength))
}

func (contentLength *ContentLength) Name() string { return "Content-Length" }

func (contentLength *ContentLength) Copy() Header { return contentLength }

func (contentLength *ContentLength) Equals(other interface{}) bool {
	if h, ok := other.(ContentLength); ok {
		return *contentLength == h
	}
	if h, ok := other.(*ContentLength); ok {
		return *contentLength == *h
	}

	return false
}

//=============================
// 		ContentType 实现
// ============================
type ContentType string

func (ct ContentType) String() string { return "Content-Type: " + string(ct) }

func (ct *ContentType) Name() string { return "Content-Type" }

func (ct *ContentType) Copy() Header { return ct }

func (ct *ContentType) Equals(other interface{}) bool {
	if h, ok := other.(ContentType); ok {
		return *ct == h
	}
	if h, ok := other.(*ContentType); ok {
		return *ct == *h
	}

	return false
}

//=============================
// 		Expires 实现
// ============================
type Expires uint32

func (expires Expires) String() string {
	return fmt.Sprintf("Expires: %d", int(expires))
}

func (expires *Expires) Name() string { return "Expires" }

func (expires *Expires) Copy() Header { return expires }

func (expires *Expires) Equals(other interface{}) bool {
	if h, ok := other.(Expires); ok {
		return *expires == h
	}
	if h, ok := other.(*Expires); ok {
		return *expires == *h
	}

	return false
}

//=============================
// 		FromHeader 实现
// ============================
type FromHeader struct {
	// The display name from the header, may be omitted.
	DisplayName MaybeString

	Address Uri

	// Any parameters present in the header.
	Params Params
}

func DefaultFromHeader() *FromHeader {
	return nil
}

func (from *FromHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("From: ")

	if displayName, ok := from.DisplayName.(String); ok && displayName.String() != "" {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", displayName))
	}

	buffer.WriteString(fmt.Sprintf("<%s>", from.Address))
	if from.Params.Length() > 0 {
		buffer.WriteString(";")
		buffer.WriteString(from.Params.ToString(';'))
	}

	return buffer.String()
}

func (from *FromHeader) Name() string { return "From" }

// Copy the header.
func (from *FromHeader) Copy() Header {
	return &FromHeader{
		DisplayName: from.DisplayName,
		Address:     from.Address.Copy(),
		Params:      from.Params.Copy(),
	}
}

func (from *FromHeader) Equals(other interface{}) bool {
	if h, ok := other.(*FromHeader); ok {
		return from.DisplayName == h.DisplayName &&
			from.Address.Equals(h.Address) &&
			from.Params.Equals(h.Params)
	}

	return false
}

//=============================
// 		Generic 实现
// ============================
type GenericHeader struct {
	// The name of the header.
	HeaderName string
	// The contents of the header, including any parameters.
	// This is transparent data that is not natively understood by gossip.
	Contents string
}

// Convert the header to a flat string representation.
func (header *GenericHeader) String() string {
	return header.HeaderName + ": " + header.Contents
}

// Pull out the header name.
func (header *GenericHeader) Name() string {
	return header.HeaderName
}

// Copy the header.
func (header *GenericHeader) Copy() Header {
	return &GenericHeader{
		HeaderName: header.HeaderName,
		Contents:   header.Contents,
	}
}

func (header *GenericHeader) Equals(other interface{}) bool {
	if h, ok := other.(*GenericHeader); ok {
		return header.HeaderName == h.HeaderName &&
			header.Contents == h.Contents
	}

	return false
}

//=============================
// 		MaxForwards 实现
// ============================
type MaxForwards uint32

func DefaultMaxForwards() *MaxForwards {
	maxForwards := MaxForwards(70)
	return &maxForwards
}

func (maxForwards *MaxForwards) String() string {
	return fmt.Sprintf("Max-Forwards: %d", int(*maxForwards))
}

func (maxForwards *MaxForwards) Name() string { return "Max-Forwards" }

func (maxForwards *MaxForwards) Copy() Header { return maxForwards }

func (maxForwards *MaxForwards) Equals(other interface{}) bool {
	if h, ok := other.(MaxForwards); ok {
		return *maxForwards == h
	}
	if h, ok := other.(*MaxForwards); ok {
		return *maxForwards == *h
	}

	return false
}

//================================
// 		Proxy-Require 头部实现
// ===============================
type ProxyRequireHeader struct {
	Options []string
}

func (proxyRequire *ProxyRequireHeader) String() string {
	return fmt.Sprintf("Proxy-Require: %s",
		strings.Join(proxyRequire.Options, ", "))
}

func (proxyRequire *ProxyRequireHeader) Name() string { return "Proxy-Require" }

func (proxyRequire *ProxyRequireHeader) Copy() Header {
	dup := make([]string, len(proxyRequire.Options))
	copy(dup, proxyRequire.Options)
	return &ProxyRequireHeader{dup}
}

func (proxyRequire *ProxyRequireHeader) Equals(other interface{}) bool {
	if h, ok := other.(*ProxyRequireHeader); ok {
		if len(proxyRequire.Options) != len(h.Options) {
			return false
		}

		for i, opt := range proxyRequire.Options {
			if opt != h.Options[i] {
				return false
			}
		}

		return true
	}

	return false
}

//=============================
// 		RecordRoute 实现
// ============================
type RecordRouteHeader struct {
	Addresses []Uri
}

func (route *RecordRouteHeader) Name() string { return "Record-Route" }

func (route *RecordRouteHeader) String() string {
	var addrs []string

	for _, uri := range route.Addresses {
		addrs = append(addrs, "<"+uri.String()+">")
	}

	return fmt.Sprintf("Record-Route: %s", strings.Join(addrs, ", "))
}

func (route *RecordRouteHeader) Copy() Header {
	newRoute := &RecordRouteHeader{
		Addresses: make([]Uri, len(route.Addresses)),
	}

	for i, uri := range route.Addresses {
		newRoute.Addresses[i] = uri.Copy()
	}

	return newRoute
}

func (route *RecordRouteHeader) Equals(other interface{}) bool {
	if h, ok := other.(*RecordRouteHeader); ok {
		for i, uri := range route.Addresses {
			if !uri.Equals(h.Addresses[i]) {
				return false
			}
		}

		return true
	}

	return false
}

//=============================
// 		Require 实现
// ============================
type RequireHeader struct {
	Options []string
}

func (require *RequireHeader) String() string {
	return fmt.Sprintf("Require: %s",
		strings.Join(require.Options, ", "))
}

func (require *RequireHeader) Name() string { return "Require" }

func (require *RequireHeader) Copy() Header {
	dup := make([]string, len(require.Options))
	copy(dup, require.Options)
	return &RequireHeader{dup}
}

func (require *RequireHeader) Equals(other interface{}) bool {
	if h, ok := other.(*RequireHeader); ok {
		if len(require.Options) != len(h.Options) {
			return false
		}

		for i, opt := range require.Options {
			if opt != h.Options[i] {
				return false
			}
		}

		return true
	}

	return false
}

//======================
// 		Route 头部实现
// ======================
type RouteHeader struct {
	Addresses []Uri
}

func (route *RouteHeader) Name() string { return "Route" }

func (route *RouteHeader) String() string {
	var addrs []string

	for _, uri := range route.Addresses {
		addrs = append(addrs, "<"+uri.String()+">")
	}

	return fmt.Sprintf("Route: %s", strings.Join(addrs, ", "))
}

func (route *RouteHeader) Copy() Header {
	newRoute := &RouteHeader{
		Addresses: make([]Uri, len(route.Addresses)),
	}

	for i, uri := range route.Addresses {
		newRoute.Addresses[i] = uri.Copy()
	}

	return newRoute
}

func (route *RouteHeader) Equals(other interface{}) bool {
	if h, ok := other.(*RouteHeader); ok {
		for i, uri := range route.Addresses {
			if !uri.Equals(h.Addresses[i]) {
				return false
			}
		}

		return true
	}

	return false
}

//=============================
// 		ExpSupported 实现
// ============================
type SupportedHeader struct {
	Options []string
}

func (support *SupportedHeader) String() string {
	return fmt.Sprintf("Supported: %s",
		strings.Join(support.Options, ", "))
}

func (support *SupportedHeader) Name() string { return "Supported" }

func (support *SupportedHeader) Copy() Header {
	dup := make([]string, len(support.Options))
	copy(dup, support.Options)
	return &SupportedHeader{dup}
}

func (support *SupportedHeader) Equals(other interface{}) bool {
	if h, ok := other.(*SupportedHeader); ok {
		if len(support.Options) != len(h.Options) {
			return false
		}

		for i, opt := range support.Options {
			if opt != h.Options[i] {
				return false
			}
		}

		return true
	}

	return false
}

//=============================
// 		ToHeader 实现
// ============================
type ToHeader struct {
	// The display name from the header, may be omitted.
	DisplayName MaybeString
	Address     Uri
	// Any parameters present in the header.
	Params Params
}

func DefaultToHeader() *ToHeader {
	return nil
}

func (to *ToHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("To: ")

	if displayName, ok := to.DisplayName.(String); ok && displayName.String() != "" {
		buffer.WriteString(fmt.Sprintf("\"%s\" ", displayName))
	}

	buffer.WriteString(fmt.Sprintf("<%s>", to.Address))

	if to.Params != nil && to.Params.Length() > 0 {
		buffer.WriteString(";")
		buffer.WriteString(to.Params.ToString(';'))
	}

	return buffer.String()
}

func (to *ToHeader) Name() string { return "To" }

// Copy the header.
func (to *ToHeader) Copy() Header {
	newTo := &ToHeader{
		DisplayName: to.DisplayName,
	}
	if to.Address != nil {
		newTo.Address = to.Address.Copy()
	}
	if to.Params != nil {
		newTo.Params = to.Params.Copy()
	}
	return newTo
}

func (to *ToHeader) Equals(other interface{}) bool {
	if h, ok := other.(*ToHeader); ok {
		return to.DisplayName.Equals(h.DisplayName) &&
			to.Address.Equals(h.Address) &&
			to.Params.Equals(h.Params)
	}

	return false
}

//=============================
// 		Unsupported 实现
// ============================
type UnsupportedHeader struct {
	Options []string
}

func (unsupported *UnsupportedHeader) String() string {
	return fmt.Sprintf("Unsupported: %s",
		strings.Join(unsupported.Options, ", "))
}

func (unsupported *UnsupportedHeader) Name() string { return "Unsupported" }

func (unsupported *UnsupportedHeader) Copy() Header {
	dup := make([]string, len(unsupported.Options))
	copy(dup, unsupported.Options)
	return &UnsupportedHeader{dup}
}

func (unsupported *UnsupportedHeader) Equals(other interface{}) bool {
	if h, ok := other.(*UnsupportedHeader); ok {
		if len(unsupported.Options) != len(h.Options) {
			return false
		}

		for i, opt := range unsupported.Options {
			if opt != h.Options[i] {
				return false
			}
		}

		return true
	}

	return false
}

//=============================
// 		UserAgent 实现
// ============================
type UserAgentHeader string

func (ua UserAgentHeader) String() string {
	return "User-Agent: " + string(ua)
}

func (ua *UserAgentHeader) Name() string { return "User-Agent" }

func (ua *UserAgentHeader) Copy() Header { return ua }

func (ua *UserAgentHeader) Equals(other interface{}) bool {
	if h, ok := other.(UserAgentHeader); ok {
		return *ua == h
	}
	if h, ok := other.(*UserAgentHeader); ok {
		return *ua == *h
	}

	return false
}

//=======================
// 		Via 头部实现
// ======================
type ViaHop struct {
	// E.g. 'SIP'.
	ProtocolName string
	// E.g. '2.0'.
	ProtocolVersion string
	Transport       string
	Host            string
	// The port for this via hop. This is stored as a pointer type, since it is an optional field.
	Port   *Port
	Params Params
}

func (hop *ViaHop) SentBy() string {
	var buf bytes.Buffer
	buf.WriteString(hop.Host)
	if hop.Port != nil {
		buf.WriteString(fmt.Sprintf(":%d", *hop.Port))
	}

	return buf.String()
}

func (hop *ViaHop) String() string {
	var buffer bytes.Buffer
	buffer.WriteString(
		fmt.Sprintf(
			"%s/%s/%s %s",
			hop.ProtocolName,
			hop.ProtocolVersion,
			hop.Transport,
			hop.Host,
		),
	)
	if hop.Port != nil {
		buffer.WriteString(fmt.Sprintf(":%d", *hop.Port))
	}

	if hop.Params.Length() > 0 {
		buffer.WriteString(";")
		buffer.WriteString(hop.Params.ToString(';'))
	}

	return buffer.String()
}

// Return an exact copy of this ViaHop.
func (hop *ViaHop) Copy() *ViaHop {
	return &ViaHop{
		ProtocolName:    hop.ProtocolName,
		ProtocolVersion: hop.ProtocolVersion,
		Transport:       hop.Transport,
		Host:            hop.Host,
		Port:            hop.Port.Copy(),
		Params:          hop.Params.Copy(),
	}
}

func (hop *ViaHop) Equals(other interface{}) bool {
	if h, ok := other.(*ViaHop); ok {
		return hop.ProtocolName == h.ProtocolName &&
			hop.ProtocolVersion == h.ProtocolVersion &&
			hop.Transport == h.Transport &&
			hop.Host == h.Host &&
			utils.Uint16PtrEq((*uint16)(hop.Port), (*uint16)(h.Port)) &&
			hop.Params.Equals(h.Params)
	}

	return false
}

type ViaHeader []*ViaHop

func DefaultViaHeader() ViaHeader {
	port := Port(5060)
	via := &ViaHop{
		ProtocolName:    "SIP",
		ProtocolVersion: "2.0",
		Transport:       DefaultProtocol,
		Host:            "127.0.0.1",
		Port:            &port,
		Params:          NewParams(),
	}

	via.Params.Add("rport", nil)
	via.Params.Add("branch", String{Str: GenerateBranch()})
	return ViaHeader{via}
}

func (via ViaHeader) String() string {
	var buffer bytes.Buffer
	buffer.WriteString("Via: ")
	for idx, hop := range via {
		buffer.WriteString(hop.String())
		if idx != len(via)-1 {
			buffer.WriteString(", ")
		}
	}

	return buffer.String()
}

func (via ViaHeader) Name() string { return "Via" }

func (via ViaHeader) Copy() Header {
	dup := make([]*ViaHop, 0, len(via))
	for _, hop := range via {
		dup = append(dup, hop.Copy())
	}
	return ViaHeader(dup)
}

func (via ViaHeader) Equals(other interface{}) bool {
	if h, ok := other.(ViaHeader); ok {
		if len(via) != len(h) {
			return false
		}

		for i, hop := range via {
			if !hop.Equals(h[i]) {
				return false
			}
		}

		return true
	}

	return false
}

func CopyWithNil(params Params) Params {
	if params == nil {
		return NewParams()
	}
	return params.Copy()
}

//=============================
// 		SipUri 实现
// ============================
type SipUri struct {
	// True if and only if the URI is a SIPS URI.
	FIsEncrypted bool

	// The user part of the URI: the 'joe' in sip:joe@bloggs.com
	// This is a pointer, so that URIs without a user part can have 'nil'.
	FUser MaybeString

	// The password field of the URI. This is represented in the URI as joe:hunter2@bloggs.com.
	// Note that if a URI has a password field, it *must* have a user field as well.
	// This is a pointer, so that URIs without a password field can have 'nil'.
	// Note that RFC 3261 strongly recommends against the use of password fields in SIP URIs,
	// as they are fundamentally insecure.
	FPassword MaybeString

	// The host part of the URI. This can be a domain, or a string representation of an IP address.
	// The port part of the URI. This is optional, and so is represented here as a pointer type.
	FDomain Addr

	// Any parameters associated with the URI.
	// These are used to provide information about requests that may be constructed from the URI.
	// (For more details, see RFC 3261 section 19.1.1).
	// These appear as a semicolon-separated list of key=value pairs following the host[:port] part.
	FUriParams Params

	// Any headers to be included on requests constructed from this URI.
	// These appear as a '&'-separated list at the end of the URI, introduced by '?'.
	// Although the values of the map are strings, they will never be NoString in practice as the parser
	// guarantees to not return blank values for header elements in SIP URIs.
	// You should not set the values of headers to NoString.
	FHeaders Params
}

func (uri *SipUri) IsEncrypted() bool {
	return uri.FIsEncrypted
}

func (uri *SipUri) SetEncrypted(flag bool) {
	uri.FIsEncrypted = flag
}

func (uri *SipUri) User() MaybeString {
	return uri.FUser
}

func (uri *SipUri) SetUser(user MaybeString) {
	uri.FUser = user
}

func (uri *SipUri) Password() MaybeString {
	return uri.FPassword
}

func (uri *SipUri) SetPassword(pass MaybeString) {
	uri.FPassword = pass
}

func (uri *SipUri) Domain() Addr {
	return uri.FDomain
}

func (uri *SipUri) SetDomain(domain Addr) {
	uri.FDomain = domain
}

func (uri *SipUri) UriParams() Params {
	return uri.FUriParams
}

func (uri *SipUri) SetUriParams(params Params) {
	uri.FUriParams = params
}

func (uri *SipUri) Headers() Params {
	return uri.FHeaders
}

func (uri *SipUri) SetHeaders(params Params) {
	uri.FHeaders = params
}

func (uri *SipUri) IsWildcard() bool {
	return false
}

// Determine if the SIP URI is equal to the specified URI according to the rules laid down in RFC 3261 s. 19.1.4.
// TODO: The Equals method is not currently RFC-compliant; fix this!
func (uri *SipUri) Equals(val interface{}) bool {
	otherPtr, ok := val.(*SipUri)
	if !ok {
		return false
	}

	other := *otherPtr
	result := uri.FIsEncrypted == other.FIsEncrypted &&
		uri.FUser == other.FUser &&
		uri.FPassword == other.FPassword &&
		uri.FDomain.Host == other.FDomain.Host &&
		utils.Uint16PtrEq((*uint16)(uri.FDomain.Port), (*uint16)(other.FDomain.Port))

	if !result {
		return false
	}

	if !uri.FUriParams.Equals(other.FUriParams) {
		return false
	}

	if !uri.FHeaders.Equals(other.FHeaders) {
		return false
	}

	return true
}

// Generates the string representation of a SipUri struct.
func (uri *SipUri) String() string {
	var buffer bytes.Buffer

	// Compulsory protocol identifier.
	if uri.FIsEncrypted {
		buffer.WriteString("sips")
		buffer.WriteString(":")
	} else {
		buffer.WriteString("sip")
		buffer.WriteString(":")
	}

	// Optional userinfo part.
	if user, ok := uri.FUser.(String); ok && user.String() != "" {
		buffer.WriteString(uri.FUser.String())
		if pass, ok := uri.FPassword.(String); ok && pass.String() != "" {
			buffer.WriteString(":")
			buffer.WriteString(pass.String())
		}
		buffer.WriteString("@")
	}

	// Compulsory hostname.
	buffer.WriteString(uri.FDomain.String())

	// Optional port number.
	//if uri.FPort != nil {
	//	buffer.WriteString(fmt.Sprintf(":%d", *uri.FPort))
	//}

	if (uri.FUriParams != nil) && uri.FUriParams.Length() > 0 {
		buffer.WriteString(";")
		buffer.WriteString(uri.FUriParams.ToString(';'))
	}

	if (uri.FHeaders != nil) && uri.FHeaders.Length() > 0 {
		buffer.WriteString("?")
		buffer.WriteString(uri.FHeaders.ToString('&'))
	}

	return buffer.String()
}

// Copy the Sip URI.
func (uri *SipUri) Copy() Uri {
	newUri := &SipUri{
		FIsEncrypted: uri.FIsEncrypted,
		FUser:        uri.FUser,
		FPassword:    uri.FPassword,
		FDomain: Addr{
			Host: uri.FDomain.Host,
		},
		FUriParams: CopyWithNil(uri.FUriParams),
		FHeaders:   CopyWithNil(uri.FHeaders),
	}

	if uri.FDomain.Port != nil {
		newUri.FDomain.Port = uri.FDomain.Port.Copy()
	}

	return newUri
}

//=============================
// 		WildcardUri 实现
// ============================
// The special wildcard URI used in Contact: headers in REGISTER requests when expiring all registrations.
type WildcardUri struct{}

func (uri WildcardUri) IsEncrypted() bool { return false }

func (uri WildcardUri) SetEncrypted(flag bool) {}

func (uri WildcardUri) User() MaybeString { return nil }

func (uri WildcardUri) SetUser(user MaybeString) {}

func (uri WildcardUri) Password() MaybeString { return nil }

func (uri WildcardUri) SetPassword(pass MaybeString) {}

func (uri *WildcardUri) Domain() Addr { return Addr{} }

func (uri *WildcardUri) SetDomain(domain Addr) {}

func (uri WildcardUri) UriParams() Params { return nil }

func (uri WildcardUri) SetUriParams(params Params) {}

func (uri WildcardUri) Headers() Params { return nil }

func (uri WildcardUri) SetHeaders(params Params) {}

// Copy the wildcard URI. Not hard!
func (uri WildcardUri) Copy() Uri { return &WildcardUri{} }

// Always returns 'true'.
func (uri WildcardUri) IsWildcard() bool {
	return true
}

// Always returns '*' - the representation of a wildcard URI in a SIP message.
func (uri WildcardUri) String() string {
	return "*"
}

// Determines if this wildcard URI equals the specified other URI.
// This is true if and only if the other URI is also a wildcard URI.
func (uri WildcardUri) Equals(other interface{}) bool {
	switch other.(type) {
	case WildcardUri:
		return true
	default:
		return false
	}
}
