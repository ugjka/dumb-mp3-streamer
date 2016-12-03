# dumb-mp3-streamer

Reads mp3 data from Stdin, and serves them over http (livestream)

```
Usage: cat *.wav | lame - - | dumb-mp3-streamer [options...]

Access stream from http://localhost:8080

Options:
	-port 	Portnumber for server (max 65535). Default: 8080
	-buffer Number of mp3 frames to buffer at start. Default: 500
	-live   Set true if input is live, to prevent overruns

```

Beware: doing something like `cat *.mp3 | dumb-mp3-streamer` can produce frankenstein streams.
Use [mp3cat](https://tomclegg.ca/mp3cat) instead!
