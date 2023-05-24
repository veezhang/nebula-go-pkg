package ws

import (
	"net"

	"github.com/olahol/melody"
	"golang.org/x/net/context"
)

type (
	WebsocketContext interface {
		Context() context.Context
		WithContext(ctx context.Context)
		Value(key any) any
		WithValue(key any, val any) WebsocketContext
		LocalAddr() net.Addr
		RemoteAddr() net.Addr
	}

	websocketContext struct {
		m *melody.Melody
		s *melody.Session
	}
)

func newWebsocketContext(m *melody.Melody, s *melody.Session) WebsocketContext {
	return &websocketContext{
		m: m,
		s: s,
	}
}

func (c *websocketContext) Context() context.Context {
	return c.s.Request.Context()
}

func (c *websocketContext) WithContext(ctx context.Context) {
	c.s.Request = c.s.Request.WithContext(ctx)
}

func (c *websocketContext) Value(key any) any {
	return c.Context().Value(key)
}

func (c *websocketContext) WithValue(key any, val any) WebsocketContext {
	c.WithContext(context.WithValue(c.Context(), key, val))
	return c
}

func (c *websocketContext) LocalAddr() net.Addr {
	return c.s.LocalAddr()
}

func (c *websocketContext) RemoteAddr() net.Addr {
	return c.s.RemoteAddr()
}
