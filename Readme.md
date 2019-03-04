## go version of shark tools

### usage

server

```
$ ./shark server -h
shark server

Usage:
  shark server [flags]

Flags:
  -a, --addr string    bind address default=127.0.0.1 (default "127.0.0.1")
  -h, --help           help for server
  -l, --loglevel int   log level; 0->panic, 1->fatal, 2->error, 3->warn, 4->info, 5->debug (default 3)
  -p, --port int       bind port default=12306 (default 12306)

```

client
```
$ ./shark client -h
shark client

Usage:
  shark client [flags]

Flags:
      --coresz int        max num of connections with remote server, -1 is unlimited (default -1)
      --cpuprofile        begin cpu profile
  -h, --help              help for client
  -l, --localport int     local proxy port (default 10087)
  -g, --loglevel int      log level; 0->panic, 1->fatal, 2->error, 3->warn, 4->info, 5->debug (default 3)
  -p, --protocol string   local proxy protocol (default "http")
  -r, --remoteport int    remote server port (default 10086)
  -s, --server string     remote server addr (default "localhost")
```


### logs

|level|msg|
|--|--|
|Debug|package seq, etc.|
|Info|routine info, etc.|
|Warn|http time out, etc.|
|Error|protocol error, or broken package. etc.|
|Fatal||
|Panic||

### TODO

1. [ ] socks support
2. [ ] more test
3. [ ] panic handler
4. [ ] package and release tools
