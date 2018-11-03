## go version of shark tools

### usage
```
Usage of ./client:
  -addr string
        remote server addr (default "localhost")
  -coresz int
        max num of connections with remote server, -1 is unlimited (default -1)
  -cpuprofile
        begin cpu profile
  -help
        show usage
  -loglevel int
        log level; 0->panic, 1->fatal, 2->error, 3->warn, 4->info, 5->debug (default 3)
  -lp int
        local proxy port (default 10087)
  -protocol string
        local proxy protocol (default "http")
  -rp int
        remote server port (default 10086)
```

### TODO

1. [ ] socks5 support
2. [ ] server implementation
3. [ ] panic handler
4. [ ] package and release tools
