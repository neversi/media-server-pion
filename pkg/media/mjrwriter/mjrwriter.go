package mjrwriter

// +build:ignore
import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"time"

	"github.com/pion/rtp"
)

var (
	errFileNotOpened = errors.New("file not opened")
)

var (
	types = map[Media][2]string{
		Audio: {"a", "opus"},
		Video: {"v", "vp8"},
		Data:  {"d", "binary"},
	}
)

const (
	mjrHeader = "MJR00002"
	meetHeader = "MEET"
)

type Media uint

const (
	Audio = iota
	Video
	Data
)

type MJRWriter struct {
	ioWriter    io.Writer
	mediaType   Media
	header      bool
	createdTime int64
}

func New(file string, m Media) (*MJRWriter, error) {
	f, err := os.Create(file)
	if err != nil {
		return nil, err
	}
	fs, err := f.Stat()
	if err != nil {
		return nil, err
	}
	createdTime := fs.ModTime().UnixNano() / 1000

	w, err := NewWith(f)
	if err != nil {
		return nil, err
	}
	w.mediaType = m
	w.createdTime = createdTime
	return w, nil
}

func NewWith(out io.Writer) (*MJRWriter, error) {
	if out == nil {
		return nil, errFileNotOpened
	}
	w := &MJRWriter{
		ioWriter: out,
	}
	return w, nil
}

type mjrJsonHeader struct {
	TypeMedia       string `json:"t"`
	Codec           string `json:"c"`
	CreatedTime     int64  `json:"s"`
	FirstPacketTime int64  `json:"u"`
}

func (mjr *MJRWriter) writeHeader(packet *rtp.Packet) error {
	header := make([]byte, 10)
	copy(header[0:], []byte(mjrHeader))

	jsonHeader := mjrJsonHeader{
		TypeMedia:       types[mjr.mediaType][0],
		Codec:           types[mjr.mediaType][1],
		CreatedTime:     mjr.createdTime,
		FirstPacketTime: time.Now().UnixNano() / 1000,
	}
	jsonBytes, err := json.Marshal(jsonHeader)
	if err != nil {
		return err
	}

	binary.BigEndian.PutUint16(header[8:], uint16(len(jsonBytes)))
	header = append(header[:10], jsonBytes...)
	if _, err := mjr.ioWriter.Write(header); err != nil {
		return err
	}
	mjr.header = true
	return nil
}

func (mjr *MJRWriter) WriteRTP(packet *rtp.Packet) error {
	currentTime := time.Now()
	if mjr.ioWriter == nil {
		return errFileNotOpened
	}
	if !mjr.header {
		if err := mjr.writeHeader(packet); err != nil {
			return err
		}
	}

	frameHeader := make([]byte, 10)
	copy(frameHeader, []byte(meetHeader))
	elapsedTime := (currentTime.UnixNano() - mjr.createdTime) / 1000
	packetHeader, err := packet.Header.Marshal()

	if err != nil {
		return err
	}

	binary.BigEndian.PutUint32(frameHeader[4:], uint32(elapsedTime))
	binary.BigEndian.PutUint16(frameHeader[8:], uint16(len(packetHeader)+len(packet.Payload)))

	if _, err := mjr.ioWriter.Write(frameHeader); err != nil {
		return err
	}

	if _, err := mjr.ioWriter.Write(packetHeader); err != nil {
		return err
	}

	if _, err := mjr.ioWriter.Write(packet.Payload); err != nil {
		return err
	}

	return nil
}

func (mjr *MJRWriter) Close() error {
	if mjr.ioWriter == nil {
		return nil
	}

	defer func() {
		mjr.ioWriter = nil
	}()

	if closer, ok := mjr.ioWriter.(io.Closer); ok {
		return closer.Close()
	}

	return nil
}
