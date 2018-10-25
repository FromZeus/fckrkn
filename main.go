package main

import (
	"github.com/FromZeus/fckrkn/fckrkn"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	capacity = kingpin.Flag(
		"capacity",
		"Capacity of local storage in number of proxies.",
	).Default("128").Short('c').Uint()
	timeout = kingpin.Flag(
		"timeout",
		"Timeout of checking out proxies in minutes.",
	).Default("1").Short('t').Uint()
	checkCap = kingpin.Flag(
		"checkCap",
		"Limit of failed checks before deleting from db.",
	).Default("3").Uint8()
	strikeCap = kingpin.Flag(
		"strikeCap",
		"Limit of strikes before deleting from db.",
	).Default("10").Uint()
	strikeTimeout = kingpin.Flag(
		"strikeTimeout",
		"Timeout of reseting strikes in minutes.",
	).Default("10").Uint()
	opTimeout = kingpin.Flag(
		"opTimeout",
		"Timeout of operations per user in sec.",
	).Default("5").Uint()
	subTimeout = kingpin.Flag(
		"subTimeout",
		"Timeout of sending notifications to subscribers in minutes.",
	).Default("15").Uint()
	dbpath = kingpin.Flag(
		"dbpath",
		"Path to level db.",
	).Default("./db").Short('d').String()
	host = kingpin.Flag(
		"host",
		"Host for proxy verification.",
	).Required().Short('h').String()
	port = kingpin.Flag(
		"port",
		"Port number of host for proxy verification.",
	).Default("443").Short('p').Uint16()
	verbose = kingpin.Flag(
		"verbose",
		"Verbose logging mode.",
	).Short('v').Bool()
	token = kingpin.Arg(
		"token",
		"Bot's token.",
	).Required().String()
)

func main() {
	kingpin.Parse()
	opts := fckrkn.NewOptions(
		*capacity,
		*timeout,
		*checkCap,
		*strikeCap,
		*strikeTimeout,
		*opTimeout,
		*subTimeout,
		*dbpath,
		*host,
		*port,
		*verbose,
	)

	bot := fckrkn.New(*token, &opts)
	bot.Start()
}
