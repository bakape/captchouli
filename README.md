[![GoDoc](https://godoc.org/github.com/bakape/captchouli?status.svg)](https://godoc.org/github.com/bakape/captchouli)
[![Build Status](https://travis-ci.org/bakape/captchouli.svg?branch=master)](https://travis-ci.org/bakape/captchouli)

# captchouli
booru-backed procedurally-generated anime image captcha library and server

![sample](https://github.com/bakape/captchouli/raw/master/assets/sample.png)

Captchouli scrapes boorus for admin-defined tags and generates and verifies captchas for user anti-bot authentication.

## Installation

1. Install OpenCV >= 2.4 development library (`libopencv-dev` on Debian-based systems)
2. Install Go >= 1.10
3. Run `go get github.com/bakape/captchouli/v2/cmd/captchouli`
4. The captchouli server binary will be located under `$HOME/go/bin/captchouli`, if the default `$GOPATH` is used.

## Usage

Captchouli can be used as either a library or standalone server.

### Server

Run `captchouli --help` for a list CLI flags.

After the server has been started and the inital tag pool populated captchouli can be accessed using a HTTP API:

| Method | Address | Receives                                                                                                                               | Returns                                                                                                                                    |
|--------|---------|----------------------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------|
| GET    | /       | Optional query parameters "captchouli-color" and "captchouli-background" for overriding the default captcha text colour and background | New captcha form HTML                                                                                                                      |
| POST   | /       | Form data from the user                                                                                                                | Either the ID of the solved captcha on success or a redirect to a fresh captcha, if incorrectly solved                                     |
| POST   | /status | "captchouli-id" parameter - the ID of the captcha you wish to check the status of                                                      | "true", if captcha exists and has been solved or "false" otherwise. Note that this unregisters the captcha to prevent reply-again attacks. |


### Advanced use cases

For more advanced use cases please refer to the Go API documented here [![GoDoc](https://godoc.org/github.com/bakape/captchouli?status.svg)](https://godoc.org/github.com/bakape/captchouli).

