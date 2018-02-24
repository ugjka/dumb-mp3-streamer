package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	goupnp "github.com/NebulousLabs/go-upnp"
)

var usage = `
Usage: cat *.wav | lame - - | dumb-mp3-streamer [options...]

Access stream from http://localhost:8080/stream

Options:
	-port 		Portnumber for server (max 65535). Default: 8080
	-buffer 	Number of seconds of data to buffer at start. Default: 10
	-chunksize	how many seconds of data to send at once. Default: 1
	-queue		How many unsent chunks before dropping data. Default: 10
	-upnp		Use to forward the port on the router

`

func main() {
	var buffSize *uint
	var port *uint
	var upnp *bool
	var chunkSize *int
	var queueSize *int
	var c = make(chan os.Signal, 2)
	chunkSize = flag.Int("chunk", 1, "chunk size in seconds")
	port = flag.Uint("port", 8080, "Server Port")
	buffSize = flag.Uint("buffer", 10, "buffer size in seconds")
	queueSize = flag.Int("queue", 10, "queue size")
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
	if *chunkSize < 1 {
		fmt.Fprint(os.Stderr, "error: chunksize too small\n")
		return
	}
	if *queueSize < 1 {
		fmt.Fprint(os.Stderr, "error: queue size too small\n")
		return
	}

	str := new(streamer)
	str.input = os.Stdin
	str.chunkSize = time.Duration(*chunkSize) * time.Second
	str.buffSize = time.Duration(*buffSize) * time.Second
	str.queueSize = *queueSize
	//Catch Ctrl+C and Kill
	signal.Notify(c, os.Interrupt)
	signal.Notify(c, os.Kill)
	go func() {
		<-c
		log.Println("Shutting Down!")
		close(str.stopper)
		if *upnp {
			_ = clearUpnp(*port)
		}
		str.Wait()
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
		Addr: ":" + strconv.Itoa(int(*port)),
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
			log.Println("Starting Streaming on http://" + ip + ":" + strconv.Itoa(int(port)) + "/")
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
				log.Println("Starting Streaming on http://[" + ip.String() + "]:" + strconv.Itoa(int(port)) + "/stream")
			} else {
				log.Println("Starting Streaming on http://" + ip.String() + ":" + strconv.Itoa(int(port)) + "/stream")
			}
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
