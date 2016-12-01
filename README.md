# dumb-mp3-livestreamer
WIP, experimenting with go's net/http

Reads mp3 data from Stdin, and serves them over http (livestream)

```
Usage: cat *.mp3 | dumb-mp3-streamer [options...]

Access stream from http://localhost:8080

Options:
	-port 	Portnumber for server (max 65535). Default: 8080
	-buffer Number of mp3 frames to buffer at start. Default: 500
	-chanbuf Buffer length for go channels (To avoid lockup). Default: 1024

```

Limitations: Currently there's no way to kill slow clients.
