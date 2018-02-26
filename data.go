package main

import (
	"bufio"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/tcolgate/mp3"
)

type streamer struct {
	sync.RWMutex
	clients   map[uint64]chan []byte
	id        uint64
	buffer    []byte
	BuffSize  time.Duration
	ReadSize  time.Duration
	QueueSize int
	WriteBuff int
	Input     io.Reader
	dec       *mp3.Decoder
	frame     *mp3.Frame
	skipped   *int
	Stop      chan bool
}

func (s *streamer) init() (err error) {
	s.Lock()
	defer s.Unlock()
	s.frame = new(mp3.Frame)
	s.skipped = new(int)
	s.clients = make(map[uint64]chan []byte)
	s.dec = mp3.NewDecoder(s.Input)
	s.buffer, _, err = s.readChunk(s.BuffSize)
	s.Stop = make(chan bool)
	if err != nil {
		return
	}
	log.Println("Buffer created...")
	return
}

func (s *streamer) addClient() (uint64, chan []byte) {
	s.Lock()
	defer s.Unlock()
	s.id++
	s.clients[s.id] = make(chan []byte, s.QueueSize)
	return s.id, s.clients[s.id]
}

func (s *streamer) delClient(id uint64) {
	s.Lock()
	defer s.Unlock()
	close(s.clients[id])
	delete(s.clients, id)
}

func (s *streamer) send(b []byte) {
	s.RLock()
	defer s.RUnlock()
	for _, v := range s.clients {
		select {
		case v <- b:
		default:
		}
	}
}

func (s *streamer) readChunk(expd time.Duration) (buf []byte, reald time.Duration, err error) {
	for {
		err = s.dec.Decode(s.frame, s.skipped)
		if err != nil {
			return
		}
		var tmp []byte
		tmp, err = ioutil.ReadAll(s.frame.Reader())
		if err != nil {
			return
		}
		buf = append(buf, tmp...)
		reald += s.frame.Duration()
		if expd < reald {
			return
		}
	}
}

func (s *streamer) readLoop() {
	defer close(s.Stop)
	var wait time.Duration
	var start time.Time
	for {
		start = time.Now()
		buf, dur, err := s.readChunk(s.ReadSize)
		if err != nil {
			log.Println(err)
			return
		}
		s.send(buf)
		s.Lock()
		if len(s.buffer) < len(buf) {
			s.buffer = buf
		} else {
			s.buffer = append(s.buffer[len(buf):], buf...)
		}
		s.Unlock()
		wait += dur - time.Now().Sub(start)
		if wait > dur {
			time.Sleep(wait)
			wait = 0
		}
	}
}

func (s *streamer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id, recieve := s.addClient()
	defer s.delClient(id)

	// Set some headers
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Server", "dumb-mp3-streamer")
	//Send MP3 stream header
	head := []byte{0x49, 0x44, 0x33, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	//Send data in chunks
	buffw := bufio.NewWriterSize(w, s.WriteBuff)
	if _, err := buffw.Write(head); err != nil {
		return
	}
	//Copy buffer
	s.RLock()
	buf := make([]byte, len(s.buffer))
	copy(buf, s.buffer)
	s.RUnlock()
	if _, err := buffw.Write(buf); err != nil {
		return
	}
	buf = nil

	for {
		chunk := <-recieve
		if _, err := buffw.Write(chunk); err != nil {
			return
		}
	}
}
