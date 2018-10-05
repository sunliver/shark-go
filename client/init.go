package main

import (
	"flag"
)

var h bool
var lp int
var protocol string
var addr string
var rp int
var loglevel int
var cpuprofile bool
var coreSz int

func init() {

	flag.BoolVar(&h, "help", false, "show usage")
	flag.IntVar(&lp, "lp", 10087, "local proxy port")
	flag.StringVar(&protocol, "protocol", "http", "local proxy protocol")
	flag.StringVar(&addr, "addr", "localhost", "remote server addr")
	flag.IntVar(&rp, "rp", 10086, "remote server port")
	flag.IntVar(&loglevel, "loglevel", 3, "log level; 0->panic, 1->fatal, 2->error, 3->warn, 4->info, 5->debug")
	flag.BoolVar(&cpuprofile, "cpuprofile", false, "begin cpu profile")
	flag.IntVar(&coreSz, "coresz", -1, "max num of connections with remote server, -1 is unlimited")

	flag.Parse()
}
