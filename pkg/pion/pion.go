// +build:ignore
package pion

import (
	// "encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"strconv"
	"strings"

	"time"

	"github.com/at-wat/ebml-go/webm"
	"github.com/gorilla/websocket"

	"github.com/neversi/media-server-pion/pkg/media/mjrwriter"
	"github.com/neversi/media-server-pion/pkg/models"
	webrtcsignal "github.com/neversi/media-server-pion/pkg/signal"

	"github.com/pion/interceptor"
	"github.com/pion/rtcp"
	"github.com/pion/webrtc/v3"
	"github.com/pion/webrtc/v3/pkg/media"
)

func HandleMedia(conn *websocket.Conn, done chan struct{}) {
	var connected bool = false
	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		object := p
		fmt.Println(string(object))
		jsonRequest := models.ClientRequest{}
		if err := json.Unmarshal(object, &jsonRequest); err != nil {
			print(err.Error())
			return
		}
		fmt.Println(jsonRequest)
		
		c := make(chan string)
		rand_num := strconv.Itoa(rand.Intn(1_000_000_000))
		rand_string := "videoroom-1234-" + rand_num + "-" + fmt.Sprintf("%v", time.Now().Unix())
		go StartConn([]byte(jsonRequest.SDP), rand_string, c, done)

		remoteSDP := <-c
		if !connected {
			jsonResponse := models.ConnectResponse{
				Command:     "connect",
				Status:      "ok",
				SDP:         remoteSDP,
				Device:      jsonRequest.Device,
				RandNumber: rand_num,
			}
			jsonResponseBytes, err := json.Marshal(jsonResponse)
			if err != nil {
				return
			}
	
			if err := conn.WriteMessage(messageType, jsonResponseBytes); err != nil {
				log.Println(err)
				return
			}
			connected = !connected
		}
	}
}

func StartConn(sd []byte, filePath string, c chan string, done chan struct{}) {
	saver := newWebmSaver(filePath)
	defer saver.Close()
	defer close(c)
	peerConnection, remoteSD := createWebRTCConn(saver, sd)
	c <- remoteSD
	<-done

	if err := peerConnection.Close(); err != nil {
		panic(err)
	}
}

type webmSaver struct {
	audioWriter, videoWriter       webm.BlockWriteCloser
	filePath                       string
}

func newWebmSaver(file string) *webmSaver {
	return &webmSaver{
		filePath:     file,
	}
}

func (s *webmSaver) Close() {
	fmt.Printf("Finalizing webm...\n")
	if s.audioWriter != nil {
		if err := s.audioWriter.Close(); err != nil {
			panic(err)
		}
	}
	if s.videoWriter != nil {
		if err := s.videoWriter.Close(); err != nil {
			panic(fmt.Errorf("Here"))
		}
	}
}

func saveToDisk(i media.Writer, track *webrtc.TrackRemote) {
	defer func() {
		if err := i.Close(); err != nil {
			panic(err)
		}
	}()

	for {
		rtpPacket, _, err := track.ReadRTP()
		if err != nil {
			if err == io.EOF {
				return
			}
			panic(err)
		}
		// data := make([]byte, 4)
		// binary.BigEndian.PutUint32(data, rtpPacket.Timestamp)
		// fmt.Println(data)
		if err := i.WriteRTP(rtpPacket); err != nil {
			panic(err)
		}
	}
}

func createWebRTCConn(saver *webmSaver, sd []byte) (*webrtc.PeerConnection, string) {
	// Everything below is the pion-WebRTC API! Thanks for using it ❤️.

	// Prepare the configuration
	config := webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	// Create a MediaEngine object to configure the supported codec
	m := &webrtc.MediaEngine{}

	// Setup the codecs you want to use.
	// Only support VP8 and OPUS, this makes our WebM muxer code simpler
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: "video/VP8", ClockRate: 90000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        96,
	}, webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}
	if err := m.RegisterCodec(webrtc.RTPCodecParameters{
		RTPCodecCapability: webrtc.RTPCodecCapability{MimeType: "audio/opus", ClockRate: 48000, Channels: 0, SDPFmtpLine: "", RTCPFeedback: nil},
		PayloadType:        111,
	}, webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	}

	i := &interceptor.Registry{}

	if err := webrtc.RegisterDefaultInterceptors(m, i); err != nil {
		panic(err)
	}

	api := webrtc.NewAPI(webrtc.WithMediaEngine(m), webrtc.WithInterceptorRegistry(i))

	// Create a new RTCPeerConnection
	peerConnection, err := api.NewPeerConnection(config)
	if err != nil {
		panic(err)
	}

	if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeAudio); err != nil {
		panic(err)
	} else if _, err = peerConnection.AddTransceiverFromKind(webrtc.RTPCodecTypeVideo); err != nil {
		panic(err)
	}

	var audioFile *mjrwriter.MJRWriter
	var videoFile *mjrwriter.MJRWriter
	// Set a handler for when a new remote track starts, this handler copies inbound RTP packets,
	// replaces the SSRC and sends them back
	peerConnection.OnTrack(func(track *webrtc.TrackRemote, receiver *webrtc.RTPReceiver) {
		// Send a PLI on an interval so that the publisher is pushing a keyframe every rtcpPLIInterval
		go func() {
			ticker := time.NewTicker(time.Second * 3)
			for range ticker.C {
				errSend := peerConnection.WriteRTCP([]rtcp.Packet{&rtcp.PictureLossIndication{MediaSSRC: uint32(track.SSRC())}})
				if errSend != nil {
					fmt.Println(errSend)
				}
			}
		}()

		codec := track.Codec()
		if strings.EqualFold(codec.MimeType, webrtc.MimeTypeOpus) {
			audioFile, err = mjrwriter.New(saver.filePath + "-audio.mjr", mjrwriter.Audio)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Got Opus track, saving to disk as %s.mjr (48 kHz, 2 channels)\n", saver.filePath)
			saveToDisk(audioFile, track)
		} else if strings.EqualFold(codec.MimeType, webrtc.MimeTypeVP8) {
			videoFile, err = mjrwriter.New(saver.filePath + "-video.mjr", mjrwriter.Video)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Got VP8 track, saving to disk as %s.mjr\n", saver.filePath)
			saveToDisk(videoFile, track)

		}
	})
	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())

		if connectionState == webrtc.ICEConnectionStateConnected {
		} else if connectionState == webrtc.ICEConnectionStateFailed ||
			connectionState == webrtc.ICEConnectionStateDisconnected || 
			connectionState == webrtc.ICEConnectionStateClosed {
			if audioFile != nil {
				closeErr := audioFile.Close()
				if closeErr != nil {
					panic(closeErr)
				}
			}
			if videoFile != nil {
				closeErr := videoFile.Close()
				if closeErr != nil {
					panic(closeErr)
				}
			}
			fmt.Println("Done writing media files")
		}
	})

	// Wait for the offer to be pasted
	offer := webrtc.SessionDescription{}
	webrtcsignal.Decode(string(sd), &offer)

	// Set the remote SessionDescription
	err = peerConnection.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}

	// Create an answer
	answer, err := peerConnection.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(peerConnection)

	// Sets the LocalDescription, and starts our UDP listeners
	err = peerConnection.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete

	// Output the answer in base64 so we can paste it in browser
	fmt.Println(webrtcsignal.Encode(*peerConnection.LocalDescription()))

	return peerConnection, webrtcsignal.Encode(*peerConnection.LocalDescription())
}
