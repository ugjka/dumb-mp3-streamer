package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/tcolgate/mp3"
)

var usage = `Usage: cat *.mp3 | dumb-mp3-streamer [options...]

Access stream from http://localhost:8080

Options:
	-port 	Portnumber for server (max 65535). Default: 8080
	-buffer Number of mp3 frames to buffer to the client. Default: 500
`

type data struct {
	sync.Mutex
	clients map[uint64]chan [][]byte
	id      uint64
	buffer  [][]byte
}

var d data

func main() {
	port := flag.Uint("port", 8080, "Server Port")
	buffer := flag.Uint("buffer", 500, "Number of frames to buffer")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(usage))
	}
	flag.Parse()
	if *port > 65535 {
		fmt.Fprint(os.Stderr, "ERROR: Invalid port number\n")
		return
	}
	if *buffer == 0 {
		fmt.Fprintf(os.Stderr, "ERROR: Buffer cannot be 0\n")
		return
	}

	d.clients = make(map[uint64]chan [][]byte)
	d.buffer = make([][]byte, *buffer)

	go read()

	srv := &http.Server{
		Addr: ":" + strconv.Itoa(int(*port)),
	}
	http.HandleFunc("/", stream)
	log.Println("Starting Streaming on http://localhost:" + strconv.Itoa(int(*port)) + "/")
	log.Fatal(srv.ListenAndServe())
}

func read() {
	// Send nil to clients to indicate the end
	finish := func() {
		d.Lock()
		for _, k := range d.clients {
			k <- nil
		}
		d.Unlock()
	}
	defer finish()

	in := mp3.NewDecoder(os.Stdin)
	var f mp3.Frame

	// Generate initial buffer
	d.Lock()
	for i := 0; i < cap(d.buffer); i++ {
		if err := in.Decode(&f); err != nil {
			log.Println(err)
			return
		}
		buf, err := ioutil.ReadAll(f.Reader())
		if err != nil {
			log.Println(err)
			return
		}
		d.buffer[i] = buf
	}
	d.Unlock()

	// Loop for sending individual mp3 frames
	for {
		start := time.Now()
		if err := in.Decode(&f); err != nil {
			log.Println(err)
			return
		}
		buf, err := ioutil.ReadAll(f.Reader())
		if err != nil {
			log.Println(err)
			return
		}
		d.Lock()
		for _, k := range d.clients {
			k <- d.buffer
		}
		d.buffer = d.buffer[1:]
		d.buffer = append(d.buffer, buf)
		d.Unlock()
		time.Sleep(f.Duration() - time.Now().Sub(start))
	}
}

// Streamer
func stream(w http.ResponseWriter, r *http.Request) {
	// Register new client
	d.Lock()
	d.id++
	id := d.id
	d.clients[id] = make(chan [][]byte)
	d.Unlock()
	// Set some headers
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Server", "dumb-mp3-livestreamer")
	//Send MP3 stream header
	b := []byte{0x49, 0x44, 0x33, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	io.Copy(w, bytes.NewReader(b))
	//Send buffer
	for _, v := range <-d.clients[id] {
		io.Copy(w, bytes.NewReader(v))
	}
	//Listen for new frames and send them
	for {
		buf := <-d.clients[id]
		if buf == nil {
			break
		}
		_, err := io.Copy(w, bytes.NewReader(buf[len(buf)-1]))
		if err != nil {
			break
		}
	}
	d.Lock()
	delete(d.clients, id)
	d.Unlock()
}
