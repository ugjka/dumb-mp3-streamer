package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	goupnp "github.com/NebulousLabs/go-upnp"
)

var usage = `
Usage: cat *.wav | lame - - | dumb-mp3-streamer [options...]

Options:
	-port 		Portnumber for server (max 65535). Default: 8080
	-buffer 	Number of seconds of mp3 audio to buffer at start. Default: 10
	-readsize	Number of seconds of mp3 audio to read at once. Default: 1
	-queue		Number of unsent chunks before dropping data. Default: 10
	-writebuff	Write buffer. Default: 32768
	-upnp		Use to forward the port on the router

`

func main() {
	var port *uint
	var upnp *bool
	var buffSize *int
	var readSize *int
	var queueSize *int
	var writeBuff *int
	var c = make(chan os.Signal, 2)
	port = flag.Uint("port", 8080, "Server Port")
	buffSize = flag.Int("buffer", 10, "buffer size in seconds")
	readSize = flag.Int("readsize", 1, "how many seconds read from source at once")
	queueSize = flag.Int("queue", 10, "queue size")
	writeBuff = flag.Int("writebuff", 32768, "write buffer size")
	upnp = flag.Bool("upnp", false, "Enable upnp port forwarding")

	flag.Usage = func() {
		fmt.Fprint(os.Stderr, fmt.Sprintf(usage))
	}
	flag.Parse()
	if *port > 65535 {
		fmt.Fprint(os.Stderr, "error: invalid port number\n")
		return
	}
	if *buffSize < 1 {
		fmt.Fprint(os.Stderr, "error: buffer too small\n")
		return
	}
	if *readSize < 1 {
		fmt.Fprint(os.Stderr, "error: chunksize too small\n")
		return
	}
	if *queueSize < 1 {
		fmt.Fprint(os.Stderr, "error: queue size too small\n")
		return
	}
	if *writeBuff < 1 {
		fmt.Fprint(os.Stderr, "error: writebuff size too small\n")
		return
	}

	str := new(streamer)
	str.input = os.Stdin
	str.readSize = time.Duration(*readSize) * time.Second
	str.buffSize = time.Duration(*buffSize) * time.Second
	str.queueSize = *queueSize
	str.writeBuff = *writeBuff
	//Catch Ctrl+C and Kill
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Shutting Down!")
		if *upnp {
			err := clearUpnp(*port)
			if err != nil {
				log.Println(err)
			}
		}
		os.Exit(0)
	}()
	//Print all possible access points
	printIP(*upnp, *port)
	//Main Reader
	err := str.init()
	if err != nil {
		log.Fatalln(err)
	}
	go str.readLoop()

	srv := &http.Server{
		Addr: fmt.Sprintf(":%d", *port),
	}
	http.Handle("/stream", str)
	log.Fatalln(srv.ListenAndServe())
}

func printIP(upnp bool, port uint) {
	if upnp {
		ip, err := forward(port)
		if err != nil {
			log.Println("Upnp forwarding failed!")
		} else {
			log.Printf("Starting Streaming on http://%s:%d/stream\n", ip, port)
		}
	}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		log.Println(err)
		return
	}
	for _, addr := range addrs {
		net, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		if strings.Contains(net.IP.String(), ":") {
			log.Printf("Starting Streaming on http://[%s]:%d/stream\n", net.IP, port)
		} else {
			log.Printf("Starting Streaming on http://%s:%d/stream\n", net.IP, port)
		}
	}
}

func forward(port uint) (string, error) {
	d, err := goupnp.Discover()
	if err != nil {
		return "", err
	}
	if err := d.Forward(uint16(port), "dumb-mp3-streamer"); err != nil {
		return "", err
	}
	return d.ExternalIP()
}

func clearUpnp(port uint) error {
	d, err := goupnp.Discover()
	if err != nil {
		return err
	}
	return d.Clear(uint16(port))
}
