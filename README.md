# AppRTC server in Go

[tomkwok/apprtc-go](https://github.com/tomkwok/apprtc-go) is a fork of [daozhao/apprtc-go](https://github.com/daozhao/apprtc-go), which is a fork of [webrtc/apprtc](https://github.com/webrtc/apprtc) to rewrite the Node.js server for [AppRTC](https://github.com/webrtc/apprtc) in Golang.

This fork of mine includes the following changes:

- Updated JavaScript files from upstream [AppRTC](https://github.com/webrtc/apprtc) to fix compatibility with the latest version of Safari
- Added functionality to dismiss red info box on clicking (which is useful for hiding error messages in Safari)
- Removed fallback to `computeengineondemand.appspot.com` TURN server to ensure only TURN server specified in command line argument is used
- Changed Go server to take address:port string instead of port number integer in command line argument (which is necessary to configure server to listen on an address other than loopback/localhost if the server is not served behind a reverse proxy)
- Removed redundant command line options, comments and code in Go server
- Added more detailed documentation and usage example
- Updated `.gitignore` to ignore binary `apprtc-go` built
- Removed flip and rotate transform on videos in CSS used for animation to reduce unnecessary load on browser
- (Todo) Rewrite Go server `apprtc.go` to re-organize code

Note that the official AppRTC documentation for query parameters to the web app can be found on the hosted site at `/params.html`.

Note that AppRTC supports two people only in a room.

## Building

To build the AppRTC Go server, run the following commands:

```
go get
go build -o apprtc-go apprtc.go
```

Cross-compiling example for ARMv6 Linux:

```
go get
env GOOS=linux GOARCH=arm GOARM=6 go build -o apprtc-go apprtc.go
```

## Usage

The AppRTC Go server `apprtc-go` is intended to be used with a STUN/TURN server such as [coturn](https://github.com/coturn/coturn).

```
Usage of ./apprtc-go:
  -cert string
      https cert pem file (default "./fullchain.pem")
  -http string
      address:port that https server listens on (default ":8080")
  -https string
      address:port that http server listens on (default ":8888")
  -key string
      https cert key file (default "./privkey.pem")
  -stun string
      stun server host:port
  -turn string
      turn server host:port
  -turn-password string
      turn server user password (default "password")
  -turn-static-auth-secret string
      turn server static auth secret
  -turn-username string
      turn server username (default "username")
```

Example usage of `apprtc-go`:

```
./apprtc-go
  --https="192.168.1.111:443"
  --cert="/path/to/combined.pem"
  --key="/path/to/combined.pem"
  --turn-username="user_in_turnserver_conf"
  --turn-password="user_in_turnserver_conf"
  --turn-static-auth-secret="static-auth-secret_in_turnserver_conf"
  --turn="example.com:5349"
  --stun="example.com:5349"
```
