package ws_test

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/vesoft-inc/go-pkg/ws"
)

func ExampleWebsocket() {
	type (
		MyReqData struct {
			MsgReq string `json:"msgReq"`
		}

		MyRespData struct {
			MsgResp    string `json:"msgResp"`
			ReqTime    string `json:"reqTime"`
			LocalAddr  string `json:"localAddr"`
			RemoteAddr string `json:"remoteAddr"`
		}
	)

	s := ws.NewServer(
		ws.WithDetailsType(ws.HandlerDetailsFull),
		ws.WithContextErrorf(func(_ context.Context, format string, a ...interface{}) {
			fmt.Printf(format+"\n", a...)
		}),
	)

	var a int
	var mu sync.Mutex
	s.RegisterMessageHandler(
		&ws.MessageHandler[*MyReqData, *MyRespData]{
			Header: ws.Header{
				ws.HeaderFieldVersion: "v1",
				ws.HeaderFieldAction:  "echo",
			},
			HandleFunc: func(wsCtx ws.WebsocketContext, header *ws.Header, reqData *MyReqData) (respData *MyRespData, err error) {
				mu.Lock()
				a++
				a1 := a
				mu.Unlock()
				time.Sleep(time.Duration(rand.Int63n(int64(time.Second))))

				header.Set("A", "aa")
				header.Set("NSID", wsCtx.Value("NSID"))

				return &MyRespData{
					MsgResp:    fmt.Sprintf(reqData.MsgReq+"%d", a1),
					ReqTime:    fmt.Sprint(wsCtx.Value("ReqTime")),
					LocalAddr:  wsCtx.LocalAddr().String(),
					RemoteAddr: wsCtx.RemoteAddr().String(),
				}, nil
			},
		},
	)

	http.Handle("/ws", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = r.WithContext(context.WithValue(r.Context(), "ReqTime", time.Now()))
		r = r.WithContext(context.WithValue(r.Context(), "NSID", "a"))
		s.ServeHTTP(w, r)
	}))

	if err := http.ListenAndServe(":1234", nil); err != nil {
		log.Fatal("ListenAndServe:", err)
	}
}
