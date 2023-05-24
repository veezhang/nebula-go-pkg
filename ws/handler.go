package ws

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/olahol/melody"
	pkgerrors "github.com/pkg/errors"
	"github.com/vesoft-inc/go-pkg/errorx"
	"golang.org/x/net/websocket"
)

const (
	HandlerDetailsNone      HandlerDetailsType = 0
	HandlerDetailsNormal    HandlerDetailsType = 1
	HandlerDetailsWithError HandlerDetailsType = 2
	HandlerDetailsFull      HandlerDetailsType = 3

	handlerRespFieldCode    = "code"
	handlerRespFieldMessage = "message"
	handlerRespFieldHeader  = "header"
	handlerRespFieldData    = "data"
	handlerRespFieldDetails = "details"

	DefaultWriteWait      = 10 * time.Second
	DefaultPongWait       = 60 * time.Second
	DefaultPingPeriod     = (DefaultPongWait * 9) / 10
	DefaultMaxMessageSize = 32 << 20 // 32MB
)

type (
	Handler interface {
		http.Handler
		RegisterMessageHandler(mh MessageHandlerInterface)
	}

	Option func(*defaultHandler)

	HandlerDetailsType int

	defaultHandler struct {
		melody            *melody.Melody
		writeWait         time.Duration
		pongWait          time.Duration
		pingPeriod        time.Duration
		maxMessageSize    int64
		fnHandshake       func(*websocket.Config, *http.Request) error
		fnGetErrCode      func(error) *errorx.ErrCode
		fnContextErrorf   func(ctx context.Context, format string, a ...interface{})
		detailsType       HandlerDetailsType
		muMessageHandlers sync.RWMutex
		messageHandlers   map[string]MessageHandlerInterface
	}
)

func NewServer(opts ...Option) Handler {
	h := &defaultHandler{
		melody:          melody.New(),
		writeWait:       DefaultWriteWait,
		pongWait:        DefaultPongWait,
		pingPeriod:      DefaultPingPeriod,
		maxMessageSize:  DefaultMaxMessageSize,
		messageHandlers: map[string]MessageHandlerInterface{},
	}
	for _, opt := range opts {
		opt(h)
	}

	h.melody.Config.WriteWait = h.writeWait
	h.melody.Config.PongWait = h.pongWait
	h.melody.Config.PingPeriod = h.pingPeriod
	h.melody.Config.MaxMessageSize = h.maxMessageSize
	h.melody.HandleMessage(func(s *melody.Session, msg []byte) {
		go h.handleMessage(s, msg)
	})

	return h
}

func WithMaxMessageSize(maxMessageSize int64) Option {
	return func(h *defaultHandler) {
		h.maxMessageSize = maxMessageSize
	}
}

func WithHandshake(fn func(*websocket.Config, *http.Request) error) Option {
	return func(h *defaultHandler) {
		h.fnHandshake = fn
	}
}

func WithGetErrCode(fn func(error) *errorx.ErrCode) Option {
	return func(h *defaultHandler) {
		h.fnGetErrCode = fn
	}
}

func WithContextErrorf(fn func(ctx context.Context, format string, a ...interface{})) Option {
	return func(h *defaultHandler) {
		h.fnContextErrorf = fn
	}
}

func WithDetailsType(detailsType HandlerDetailsType) Option {
	return func(h *defaultHandler) {
		h.detailsType = detailsType
	}
}

func (h *defaultHandler) RegisterMessageHandler(mh MessageHandlerInterface) {
	header := mh.GetHeader()

	h.muMessageHandlers.Lock()
	h.messageHandlers[header.Key()] = mh
	h.muMessageHandlers.Unlock()
}

func (h *defaultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.melody.HandleRequest(w, r)
}

func (h *defaultHandler) handleMessage(s *melody.Session, msg []byte) {
	wsCtx := newWebsocketContext(h.melody, s)

	var tmpReq struct {
		Header Header `json:"header"`
	}

	fn := func() (respData any, err error) {
		defer func() {
			if r := recover(); r != nil {
				err = pkgerrors.New(fmt.Sprintf("panic: %+v", r))
			}
		}()
		if err = json.Unmarshal(msg, &tmpReq); err != nil {
			return nil, pkgerrors.WithMessagef(ErrParam, "unmarshal failed, %s", err)
		}

		h.muMessageHandlers.RLock()
		mh, ok := h.messageHandlers[tmpReq.Header.Key()]
		h.muMessageHandlers.RUnlock()
		if !ok {
			return nil, pkgerrors.WithMessagef(ErrParam, "unknown header msg type %s", tmpReq.Header.Key())
		}

		return mh.Handle(wsCtx, &tmpReq.Header, msg)
	}

	respData, err := fn()

	resp := h.getResp(&tmpReq.Header, respData, err)

	respBytes, err := json.Marshal(resp)
	if err != nil {
		resp = h.getResp(&tmpReq.Header, nil, err)
		respBytes, err = json.Marshal(resp)
		if err != nil {
			h.errorf(wsCtx.Context(), "[ws] %s marshal response failed, %s", tmpReq.Header.Key(), err)
		}
	}

	if err = s.Write(respBytes); err != nil {
		h.errorf(wsCtx.Context(), "[ws] %s send message failed, %s", tmpReq.Header.Key(), err)
	}
}

func (h *defaultHandler) getResp(header *Header, respData any, err error) any {
	resp := map[string]interface{}{
		handlerRespFieldHeader:  header,
		handlerRespFieldCode:    0,
		handlerRespFieldMessage: "Success",
		handlerRespFieldData:    respData,
	}

	if err != nil {
		e, ok := errorx.AsCodeError(err)
		if !ok {
			err = errorx.WithCode(errorx.TakeCodePriority(func() *errorx.ErrCode {
				if h.fnGetErrCode == nil {
					return nil
				}
				return h.fnGetErrCode(err)
			}, func() *errorx.ErrCode {
				return errorx.NewErrCode(errorx.CCInternalServer, 0, 0, "ErrInternalServer")
			}), err)
			e, _ = errorx.AsCodeError(err)
		}

		resp[handlerRespFieldCode] = e.GetCode()
		resp[handlerRespFieldMessage] = e.GetMessage()
		if details := h.getDetails(e); details != "" {
			resp[handlerRespFieldDetails] = details
		}
	}

	return resp
}

func (h *defaultHandler) getDetails(e errorx.CodeError) string {
	switch h.detailsType {
	case HandlerDetailsNone:
	case HandlerDetailsNormal:
		return e.Error()
	case HandlerDetailsWithError:
		if internalError := stderrors.Unwrap(e); internalError != nil {
			return fmt.Sprintf("%s:%s", e.Error(), internalError.Error())
		}
		return e.Error()
	case HandlerDetailsFull:
		return fmt.Sprintf("%+v", e)
	}
	return ""
}

func (h *defaultHandler) errorf(ctx context.Context, format string, a ...interface{}) {
	if h.fnContextErrorf != nil {
		h.fnContextErrorf(ctx, format, a...)
	}
}
