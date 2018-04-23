package fckrkn

type Options struct {
	capacity      uint
	timeout       uint
	checkCap      uint8
	strikeCap     uint
	strikeTimeout uint
	opTimeout     uint
	dbpath        string
	host          string
	port          uint16
	verbose       bool
}

func NewOptions(
	capacity uint,
	timeout uint,
	checkCap uint8,
	strikeCap uint,
	strikeTimeout uint,
	opTimeout uint,
	dbpath string,
	host string,
	port uint16,
	verbose bool,
) Options {
	o := Options{capacity, timeout, checkCap, strikeCap, strikeTimeout, opTimeout, dbpath, host, port, verbose}
	return o
}
