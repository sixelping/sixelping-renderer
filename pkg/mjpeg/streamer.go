package mjpeg

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"time"
)

//Based on https://github.com/mattn/go-mjpeg/blob/master/mjpeg.go

type Streamer struct {
	channel chan []byte
}

func NewStreamer() *Streamer {
	return &Streamer{
		channel: make(chan []byte),
	}
}

func (s *Streamer) NewFrame(frame image.Image) error {
	buf := new(bytes.Buffer)
	err := jpeg.Encode(buf, frame, nil)
	if err != nil {
		return err
	}
	b := buf.Bytes()
	s.channel <- b
	return nil
}

func (s *Streamer) Close() {
	close(s.channel)
}

func (s *Streamer) HandleHTTP(w http.ResponseWriter, r *http.Request) {
	m := multipart.NewWriter(w)
	defer m.Close()
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+m.Boundary())
	w.Header().Set("Connection", "close")
	h := textproto.MIMEHeader{}
	st := fmt.Sprint(time.Now().Unix())
	for frame := range s.channel {
		h.Set("Content-Type", "image/jpeg")
		h.Set("Content-Length", fmt.Sprint(len(frame)))
		h.Set("X-StartTime", st)
		h.Set("X-TimeStamp", fmt.Sprint(time.Now().Unix()))
		mw, err := m.CreatePart(h)
		if err != nil {
			break
		}
		_, err = mw.Write(frame)
		if err != nil {
			break
		}
		if flusher, ok := mw.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}
