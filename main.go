package main

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	upnp "github.com/NebulousLabs/go-upnp"
	"github.com/tcolgate/mp3"
)

func main() {
	var port uint16 = 4321
	rout, _ := upnp.Discover()
	ip, _ := rout.ExternalIP()
	log.Println(ip)
	_ = rout.Forward(port, "GoStream")
	type data struct {
		sync.Mutex
		clients map[uint64]chan []byte
		id      uint64
	}
	var d data
	d.clients = make(map[uint64]chan []byte)

	read := func() {
		in := mp3.NewDecoder(os.Stdin)
		var f mp3.Frame
		for {
			start := time.Now()
			if err := in.Decode(&f); err != nil {
				break
			}
			d.Lock()
			for _, k := range d.clients {
				buf, _ := ioutil.ReadAll(f.Reader())
				k <- buf
			}
			d.Unlock()
			time.Sleep(f.Duration() - time.Now().Sub(start))
		}
		d.Lock()
		for _, k := range d.clients {
			k <- nil
		}
		d.Unlock()
	}

	stream := func(w http.ResponseWriter, r *http.Request) {
		d.Lock()
		d.id++
		id := d.id
		d.clients[id] = make(chan []byte)
		d.Unlock()
		w.Header().Set("Content-Type", "audio/mpeg")
		// MP3 stream header
		b := []byte{0x49, 0x44, 0x33, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
		io.Copy(w, bytes.NewReader(b))
		for {
			buf := <-d.clients[id]
			if buf == nil {
				break
			}
			_, err := io.Copy(w, bytes.NewReader(buf))
			if err != nil {
				break
			}
		}
		d.Lock()
		delete(d.clients, id)
		d.Unlock()
	}

	go read()
	srv := &http.Server{
		Addr: ":" + strconv.Itoa(int(port)),
	}
	http.HandleFunc("/", stream)
	log.Fatal(srv.ListenAndServe())
}
