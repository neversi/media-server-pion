package models

type ClientRequest struct {
	Command string `json:"cmd"`
	Room	int    `json:"room,omitempty"`
	SDP     string `json:"sdp,omitempty"`
	Device  string `json:"device,omitempty"`
}

type ConnectResponse struct {
	Command     string `json:"cmd"`
	Status      string `json:"status"`
	SDP         string `json:"sdp,omitempty"`
	Device      string `json:"device,omitempty"`
	RandNumber string `json:"rand_num,omitempty"`
}
