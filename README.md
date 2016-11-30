# dumb-mp3-livestreamer
WIP, experimenting with go's net/http

Reads mp3 data from Stdin, and serves them over http (livestream)

```
Usage: cat *.mp3 | dumb-mp3-streamer [options...]

Access stream from http://localhost:8080

Options:
	-port 	Portnumber for server (max 65535). Default: 8080
```

Limitations: Currently there's no way to kill slow clients so you may run out ram if some clients can't catch up
