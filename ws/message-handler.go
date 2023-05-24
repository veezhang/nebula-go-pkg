package ws

import (
	"encoding/json"
)

var _ MessageHandlerInterface = (*MessageHandler[any, any])(nil)

type (
	MessageHandlerInterface interface {
		GetHeader() *Header
		Handle(wsCtx WebsocketContext, header *Header, msg []byte) (respData any, err error)
	}

	MessageHandler[ReqDataType any, RespDataType any] struct {
		Header     Header
		HandleFunc func(wsCtx WebsocketContext, header *Header, reqData ReqDataType) (respData RespDataType, err error)
	}
)

func (mh *MessageHandler[ReqDataType, RespDataType]) GetHeader() *Header {
	h := mh.Header
	return &h
}

func (mh *MessageHandler[ReqDataType, RespDataType]) Handle(
	wsCtx WebsocketContext,
	header *Header,
	msg []byte,
) (respData any, err error) {
	var tmpReq struct {
		Data ReqDataType `json:"data"`
	}
	if err = json.Unmarshal(msg, &tmpReq); err != nil {
		return nil, err
	}

	return mh.HandleFunc(wsCtx, header, tmpReq.Data)
}
