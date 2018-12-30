## go version of shark tools

### usage
```
$ ./shark-go client -h
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

### TODO

1. [ ] socks5 support
2. [ ] server implementation
3. [ ] panic handler
4. [ ] package and release tools
