package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"time"

	goupnp "github.com/NebulousLabs/go-upnp"
	"github.com/tcolgate/mp3"
)

var usage = `
Usage: cat *.wav | lame - - | dumb-mp3-streamer [options...]

Access stream from http://localhost:8080

Options:
	-port 	Portnumber for server (max 65535). Default: 8080
	-buffer Number of mp3 frames to buffer at start. Default: 500
	-upnp		Use to forward the port on the router

`

type data struct {
	sync.Mutex
	clients map[uint64]chan []byte
	id      uint64
	buffer  [][]byte
}

var d data
var buffer *uint
var port *uint
var upnp *bool
var c = make(chan os.Signal, 2)

func main() {
	port = flag.Uint("port", 8080, "Server Port")
	buffer = flag.Uint("buffer", 500, "Number of frames to buffer")
	upnp = flag.Bool("upnp", false, "Enable upnp port forwarding")
	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(usage))
	}
	flag.Parse()
	if *port > 65535 {
		fmt.Fprint(os.Stderr, "ERROR: Invalid port number\n")
		return
	}
	if *buffer == 0 {
		fmt.Fprint(os.Stderr, "ERROR: Buffer cannot be 0\n")
		return
	}
	//Catch Ctrl+C and Kill
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)
	go func() {
		<-c
		log.Println("Shutting Down!")
		if *upnp {
			_ = clearUpnp()
		}
		os.Exit(0)
	}()

	d.clients = make(map[uint64]chan []byte)
	d.buffer = make([][]byte, *buffer)
	//Print all possible access points
	printIP()
	//Main Reader
	go read()

	srv := &http.Server{
		Addr: ":" + strconv.Itoa(int(*port)),
	}
	http.HandleFunc("/", stream)
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
		time.Sleep(time.Second * 10)
		c <- os.Kill
	}
	defer finish()

	in := mp3.NewDecoder(os.Stdin)
	var f mp3.Frame

	//Generate buffer
	d.Lock()
	for i, _ := range d.buffer {
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
	log.Println("Buffer created!")
	// Timings
	var wait time.Duration
	var delta time.Duration
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
		//Do not send data when channel is full
		for _, k := range d.clients {
			if len(k) < int(*buffer) {
				k <- buf
			}
		}
		// Update the buffer
		d.buffer = d.buffer[1:]
		d.buffer = append(d.buffer, buf)
		d.Unlock()

		//Frame Delayer
		wait += delta
		delta = f.Duration() - time.Now().Sub(start)
		if wait > time.Second {
			time.Sleep(wait)
			wait = 0
		}

	}
}

// Streamer
func stream(w http.ResponseWriter, r *http.Request) {
	// Register new client
	d.Lock()
	d.id++
	id := d.id
	d.clients[id] = make(chan []byte, *buffer)
	d.Unlock()
	// Remove client
	finish := func() {
		d.Lock()
		delete(d.clients, id)
		d.Unlock()
	}
	defer finish()
	// Set some headers
	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Server", "dumb-mp3-streamer")
	//Send MP3 stream header
	b := []byte{0x49, 0x44, 0x33, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	if _, err := w.Write(b); err != nil {
		return
	}
	// Send initial buffer
	d.Lock()
	// copy the buffer to reduce the lock time
	buf := d.buffer
	d.Unlock()
	for _, k := range buf {
		if _, err := w.Write(k); err != nil {
			return
		}
	}
	// Release the copied buffer
	buf = nil
	//Listen for new frames and send them
	for {
		buf := <-d.clients[id]
		//End if data is nil
		if buf == nil {
			return
		}
		if _, err := w.Write(buf); err != nil {
			return
		}
	}
}

func printIP() {
	if *upnp {
		ip, err := forward()
		if err != nil {
			log.Println("Upnp forwarding failed!")
		} else {
			log.Println("Starting Streaming on http://" + ip + ":" + strconv.Itoa(int(*port)) + "/")
		}
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if strings.Contains(ip.String(), ":") {
				log.Println("Starting Streaming on http://[" + ip.String() + "]:" + strconv.Itoa(int(*port)) + "/")
			} else {
				log.Println("Starting Streaming on http://" + ip.String() + ":" + strconv.Itoa(int(*port)) + "/")
			}
		}
	}
}

func forward() (string, error) {
	d, err := goupnp.Discover()
	if err != nil {
		return "", err
	}
	if err := d.Forward(uint16(*port), "dumb-mp3-streamer"); err != nil {
		return "", err
	}
	if ip, err := d.ExternalIP(); err != nil {
		return "", err
	} else {
		return ip, nil
	}
}

func clearUpnp() error {
	d, err := goupnp.Discover()
	if err != nil {
		return err
	}
	if err := d.Clear(uint16(*port)); err != nil {
		return err
	}
	return nil
}
