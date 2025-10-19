# Specification for tcz-proxy

## High level description
tcz-proxy is a go application that behaves similar to a http proxy.
It listens on a tcp port (8080 by default, can be overridden through a command line flag).

When an http request comes, the server will replace the host part of the url with a different one. 
The replacement host can be read from a configuration file in yaml format or overridden using a command line flag.

The server will also have a full suite of unit tests to verify all functionality.

## Features

### Fetching specific files from other locations

The server will also be able to redirect some requests to other hosts based on a mapping in the config file.
The mapping will include a regex pattern to match against the path of the url and that pattern will be mapped to a destination, replacing references to capture groups in the pattern.

For example, the config can include a mapping such as:

from: .*/\(\d+\).x/(aarch64|armhf)/tcz/watchdog.tcz
to: https://github.com/asssaf/picore-watchdog/releases/download/\1/watchdog-\2.zip

The proxy can also handle a destination that is https even though the client is communicating with the proxy server over http.
