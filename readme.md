# Rotator

Rotator is a Go package that provides a proxy rotation service. It reads a list of proxy URLs from a file, tests their availability, and manages a pool of working proxies. The package also includes a simple HTTP proxy server that rotates through the available proxies for each request.

## Installation

To install the package, use the following command:
```
go install github.com/rodrikv/rotator/cmd/rotator
```

This will install the `rotator` command in your `$GOPATH/bin` directory.

## Usage

To run the proxy rotation service, use the following command:

```
./rotator -l path/to/proxies.txt -p PORT -h HOST
```

- `-l` or `--links`: Path to the file containing the proxy URLs (one per line).
- `-p` or `--port`: Port number to listen on (default: 9000).
- `-h` or `--host`: Host address to listen on (default: 127.0.0.1).

Once the service is running, you can configure your applications to use the proxy server at `HOST:PORT`.



## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
