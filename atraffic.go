package main

import (
	"fmt"
	"net"
	"strings"
	"strconv"
	"sync"
	"time"
)

type TrafficEntry struct {
	mu       sync.Mutex
	count    int
	inbytes  int
	outbytes int
	ttfbsum  int
}

type TrafficList struct {
	mu       sync.Mutex
	vhlist   map[string]*TrafficEntry
	count    int
	inbytes  int
	outbytes int
	ttfbsum  int
}

type TrafficMap struct {
	mu        sync.Mutex
	timeslots map[time.Time]*TrafficList
}

func NewTrafficList() *TrafficList {
	tl := TrafficList{}
	tl.mu = sync.Mutex{}
	tl.vhlist = make(map[string]*TrafficEntry)
	tl.count = 0
	tl.inbytes = 0
	tl.outbytes = 0
	tl.ttfbsum = 0
	return &tl
}

func (tm *TrafficMap) SendTraffic(c net.Conn, prefix, hostname string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	hn := strings.ReplaceAll(hostname, ".", "_")
	now := time.Now().Truncate(time.Minute)

	var tsc, vhc, rc, rxc, txc int

	for ts := range tm.timeslots {
		if ts.Before(now) {
			tsc++
			tl := tm.timeslots[ts]
			for vh := range tl.vhlist {
				vhc++
				te := tl.vhlist[vh]
				vhost := strings.ReplaceAll(vh, ".", "_")
				rc += te.count
				rxc += te.inbytes
				txc += te.outbytes
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".requestcount "+strconv.Itoa(te.count)+" "+strconv.FormatInt(ts.Unix(),10)+"\n")
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".rxbytes "+strconv.Itoa(te.inbytes)+" "+strconv.FormatInt(ts.Unix(),10)+"\n")
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".txbytes "+strconv.Itoa(te.outbytes)+" "+strconv.FormatInt(ts.Unix(),10)+"\n")
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".avgttfb "+strconv.Itoa(te.ttfbsum/te.count)+" "+strconv.FormatInt(ts.Unix(),10)+"\n")
			}
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.requestcount "+strconv.Itoa(tl.count)+" "+strconv.FormatInt(ts.Unix(),10)+"\n")
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.rxbytes "+strconv.Itoa(tl.inbytes)+" "+strconv.FormatInt(ts.Unix(),10)+"\n")
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.txbytes "+strconv.Itoa(tl.outbytes)+" "+strconv.FormatInt(ts.Unix(),10)+"\n")
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.avgttfb "+strconv.Itoa(tl.ttfbsum/tl.count)+" "+strconv.FormatInt(ts.Unix(),10)+"\n")
			delete(tm.timeslots, ts)
		}
	}

	logger.Println("Sent data for", tsc, "timestamps to graphite. Stats: active vhosts", vhc, "requests", rc, "RX", rxc, "TX", txc, "bytes")
}

func (tm *TrafficMap) Get(ts time.Time) (*TrafficList, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tl, ok := tm.timeslots[ts]
	if ok != true {
		return nil, fmt.Errorf("Key not found")
	}

	return tl, nil
}

func (tm *TrafficMap) Add(ts time.Time, tl *TrafficList) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	if tm.timeslots == nil {
		tm.timeslots = make(map[time.Time]*TrafficList)
	}
	tm.timeslots[ts] = tl
}

func (te *TrafficEntry) Add(ib, ob, ttfb int) {
	te.mu.Lock()
	defer te.mu.Unlock()
	te.count++
	te.inbytes += ib
	te.outbytes += ob
	te.ttfbsum += ttfb
}

func (tl *TrafficList) get(vh string) (*TrafficEntry, error) {
	te, ok := tl.vhlist[vh]
	if ok != true {
		return nil, fmt.Errorf("Cannot find vh entry")
	}
	return te, nil
}

func (tl *TrafficList) AddEntry(vh string, ib, ob, ttfb int) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.count++
	tl.inbytes += ib
	tl.outbytes += ob
	tl.ttfbsum += ttfb
	te, err := tl.get(vh)
	if err != nil {
		tex := TrafficEntry{
			mu:       sync.Mutex{},
			count:    1,
			inbytes:  ib,
			outbytes: ob,
			ttfbsum:  ttfb,
		}
		tl.vhlist[vh] = &tex
		return
	}
	te.Add(ib, ob, ttfb)
}
