package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

type TrafficData struct {
	count    int
	inbytes  int
	outbytes int
	ttfbsum  int
}

type TrafficEntry struct {
	mu       sync.Mutex
	handlers map[string]*TrafficData
}

type TrafficList struct {
	mu     sync.Mutex
	vhlist map[string]*TrafficEntry
	data   TrafficData
}

type TrafficMap struct {
	mu        sync.Mutex
	timeslots map[time.Time]*TrafficList
}

func NewTrafficList() *TrafficList {
	tl := TrafficList{}
	tl.mu = sync.Mutex{}
	tl.vhlist = make(map[string]*TrafficEntry)
	tl.data.count = 0
	tl.data.inbytes = 0
	tl.data.outbytes = 0
	tl.data.ttfbsum = 0
	return &tl
}

func (tm *TrafficMap) SendTraffic(c net.Conn, prefix, hostname string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	hn := strings.ReplaceAll(hostname, ".", "_")
	now := time.Now().Truncate(time.Minute)

	var tsc, vhc, hdlc, rc, rxc, txc int
	var vhcnt, vhib, vhob, vhttfb int

	for ts := range tm.timeslots {
		if ts.Before(now) {
			tsc++
			tl := tm.timeslots[ts]
			for vh := range tl.vhlist {
				vhc++
				te := tl.vhlist[vh]
				vhcnt = 0
				vhib = 0
				vhob = 0
				vhttfb = 0
				vhost := strings.ReplaceAll(vh, ".", "_")
				for hdl := range te.handlers {
					hdlc++
					td := te.handlers[hdl]
					handler := strings.ReplaceAll(hdl, ".", "_")
					if handler == "" {
						handler = "static_content"
					}
					rc += td.count
					rxc += td.inbytes
					txc += td.outbytes
					vhcnt += td.count
					vhib += td.inbytes
					vhob += td.outbytes
					vhttfb += td.ttfbsum
					fmt.Fprintf(c, prefix+"."+hn+".virtualhosts_byhandler."+vhost+"."+handler+".requestcount "+strconv.Itoa(td.count)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
					fmt.Fprintf(c, prefix+"."+hn+".virtualhosts_byhandler."+vhost+"."+handler+".rxbytes "+strconv.Itoa(td.inbytes)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
					fmt.Fprintf(c, prefix+"."+hn+".virtualhosts_byhandler."+vhost+"."+handler+".txbytes "+strconv.Itoa(td.outbytes)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
					fmt.Fprintf(c, prefix+"."+hn+".virtualhosts_byhandler."+vhost+"."+handler+".avgttfb "+strconv.Itoa(td.ttfbsum/td.count)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
				}
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".requestcount "+strconv.Itoa(vhcnt)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".rxbytes "+strconv.Itoa(vhib)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".txbytes "+strconv.Itoa(vhob)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".avgttfb "+strconv.Itoa(vhttfb/vhcnt)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
			}
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.requestcount "+strconv.Itoa(tl.data.count)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.rxbytes "+strconv.Itoa(tl.data.inbytes)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.txbytes "+strconv.Itoa(tl.data.outbytes)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.avgttfb "+strconv.Itoa(tl.data.ttfbsum/tl.data.count)+" "+strconv.FormatInt(ts.Unix(), 10)+"\n")
			delete(tm.timeslots, ts)
		}
	}

	logger.Println("Sent data for", tsc, "timestamps to graphite. Stats: active vhosts", vhc, "requests", rc, "RX", rxc, "TX", txc, "bytes")
}

func (tm *TrafficMap) Get(ts time.Time) (*TrafficList, error) {
	tm.mu.Lock()
	defer tm.mu.Unlock()
	tl, ok := tm.timeslots[ts]
	if !ok {
		return nil, fmt.Errorf("key not found")
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

func (te *TrafficEntry) Add(ib, ob, ttfb int, handler string) {
	te.mu.Lock()
	defer te.mu.Unlock()
	if te.handlers == nil {
		te.handlers = make(map[string]*TrafficData)
	}
	td, ok := te.handlers[handler]
	if !ok {
		td = &TrafficData{
			count:    1,
			inbytes:  ib,
			outbytes: ob,
			ttfbsum:  ttfb,
		}
		te.handlers[handler] = td
		return
	}
	td.count++
	td.inbytes += ib
	td.outbytes += ob
	td.ttfbsum += ttfb
}

func (tl *TrafficList) get(vh string) (*TrafficEntry, error) {
	te, ok := tl.vhlist[vh]
	if !ok {
		return nil, fmt.Errorf("cannot find vh entry")
	}
	return te, nil
}

func (tl *TrafficList) AddEntry(vh string, ib, ob, ttfb int, handler string) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.data.count++
	tl.data.inbytes += ib
	tl.data.outbytes += ob
	tl.data.ttfbsum += ttfb
	te, err := tl.get(vh)
	if err != nil {
		td := TrafficData{
			count:    1,
			inbytes:  ib,
			outbytes: ob,
			ttfbsum:  ttfb,
		}
		tex := TrafficEntry{
			mu:       sync.Mutex{},
			handlers: map[string]*TrafficData{},
		}
		tex.handlers[handler] = &td
		tl.vhlist[vh] = &tex
		return
	}
	te.Add(ib, ob, ttfb, handler)
}
