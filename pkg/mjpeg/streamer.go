package mjpeg

import (
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"sync"
	"time"
)

//Based on https://github.com/mattn/go-mjpeg/blob/master/mjpeg.go

type Streamer struct {
	channels [](chan *[]byte)
	mut      sync.Mutex
}

func NewStreamer() *Streamer {
	return &Streamer{
		channels: make([]chan *[]byte, 0),
	}
}

func (s *Streamer) NewFrame(frame *[]byte) error {
	s.sendFrame(frame)
	return nil
}

func (s *Streamer) sendFrame(data *[]byte) {
	s.mut.Lock()
	defer s.mut.Unlock()
	for _, c := range s.channels {
		select {
		case c <- data:
		default:
		}
	}
}

func (s *Streamer) Close() {
	s.mut.Lock()
	defer s.mut.Unlock()
	for _, c := range s.channels {
		close(c)
	}
}

func (s *Streamer) registerChannel(c chan *[]byte) {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.channels = append(s.channels, c)
}

func (s *Streamer) unregisterChannel(c chan *[]byte) {
	s.mut.Lock()
	defer s.mut.Unlock()
	newChannels := make([]chan *[]byte, 0)
	for _, d := range s.channels {
		if d != c {
			newChannels = append(newChannels, d)
		}
	}
	s.channels = newChannels
}

func tryCloseChannel(c chan *[]byte) {
	select {
	case <-c:
		close(c)
	}
}

func (s *Streamer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c := make(chan *[]byte)
	defer tryCloseChannel(c)
	s.registerChannel(c)
	defer s.unregisterChannel(c)
	m := multipart.NewWriter(w)
	defer m.Close()
	w.Header().Set("Content-Type", "multipart/x-mixed-replace; boundary="+m.Boundary())
	w.Header().Set("Connection", "close")
	h := textproto.MIMEHeader{}
	st := fmt.Sprint(time.Now().Unix())
	for frame := range c {
		h.Set("Content-Type", "image/jpeg")
		h.Set("Content-Length", fmt.Sprint(len(*frame)))
		h.Set("X-StartTime", st)
		h.Set("X-TimeStamp", fmt.Sprint(time.Now().Unix()))
		mw, err := m.CreatePart(h)
		if err != nil {
			break
		}
		_, err = mw.Write(*frame)
		if err != nil {
			break
		}
		if flusher, ok := mw.(http.Flusher); ok {
			flusher.Flush()
		}
	}
}
