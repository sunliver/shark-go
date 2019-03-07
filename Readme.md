## go version of shark tools

A self defined protocol proxy client and server.

### usage

server

```
shark server

Usage:
  shark server [flags]

Flags:
  -a, --addr string    bind address default=127.0.0.1 (default "127.0.0.1")
  -h, --help           help for server
  -g, --loglevel int   log level; 0->panic, 1->fatal, 2->error, 3->warn, 4->info, 5->debug (default 2)
  -p, --port int       bind port default=12306 (default 12306)
```

client
```
shark client

Usage:
  shark client [flags]

Flags:
      --coresz int        max num of connections with remote server (default 4)
  -h, --help              help for client
  -l, --localport int     local proxy port (default 10087)
  -g, --loglevel int      log level; 0->panic, 1->fatal, 2->error, 3->warn, 4->info, 5->debug (default 2)
  -p, --protocol string   local proxy protocol, currently only support http (default "http")
  -r, --remoteport int    remote server port (default 12306)
  -s, --server string     remote server addr (default "127.0.0.1")
```
