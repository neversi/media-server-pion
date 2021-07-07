package docs

import "github.com/neversi/media-server-pion/pkg/models"

// swagger:route POST /ws ws-tag ws
// connect-rtc sends an SDP to establish new connection with media-server.
// responses:
// 200: connectResponse

//
// swagger:response connectResponse
type connectResponseWrapper struct {
	// in:body
	Body models.ConnectResponse
}

// swagger:parameters ws
type wsRequestWrapper struct {
	// in:body
	Body models.ClientRequest
}

/* со стороны студента добавить вход по комнате по вебсокет, проктор */