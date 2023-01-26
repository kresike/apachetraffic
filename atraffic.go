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
	count       int
	inbytes     int
	inbytesmin  int
	inbytesmax  int
	outbytes    int
	outbytesmin int
	outbytesmax int
	ttfbsum     int
	ttfbmin     int
	ttfbmax     int
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

func (tm *TrafficMap) SendTraffic(c net.Conn, prefix, serverprefix, hostname string) {
	tm.mu.Lock()
	defer tm.mu.Unlock()

	hn := strings.ReplaceAll(hostname, ".", "_")
	now := time.Now().Truncate(time.Minute)

	var tsc, vhc, hdlc, rc, rxc, txc int
	var vhcnt, vhib, vhob, vhttfb int
	var sTraffic TrafficEntry

	for ts := range tm.timeslots {
		tstamp := strconv.FormatInt(ts.Unix(), 10)
		if ts.Before(now) {
			tsc++
			tl := tm.timeslots[ts]
			sTraffic = TrafficEntry{
				mu:       sync.Mutex{},
				handlers: map[string]*TrafficData{},
			}
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
					fmt.Fprintf(c, prefix+"."+hn+".virtualhosts_byhandler."+vhost+"."+handler+".requestcount "+strconv.Itoa(td.count)+" "+tstamp+"\n")
					fmt.Fprintf(c, prefix+"."+hn+".virtualhosts_byhandler."+vhost+"."+handler+".rxbytes "+strconv.Itoa(td.inbytes)+" "+tstamp+"\n")
					fmt.Fprintf(c, prefix+"."+hn+".virtualhosts_byhandler."+vhost+"."+handler+".txbytes "+strconv.Itoa(td.outbytes)+" "+tstamp+"\n")
					fmt.Fprintf(c, prefix+"."+hn+".virtualhosts_byhandler."+vhost+"."+handler+".avgttfb "+strconv.Itoa(td.ttfbsum/td.count)+" "+tstamp+"\n")
					sTraffic.Aggregate(td.count, td.inbytes, td.inbytesmin, td.inbytesmax, td.outbytes, td.outbytesmin, td.outbytesmax, td.ttfbsum, td.ttfbmin, td.ttfbmax, handler)
				}
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".requestcount "+strconv.Itoa(vhcnt)+" "+tstamp+"\n")
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".rxbytes "+strconv.Itoa(vhib)+" "+tstamp+"\n")
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".txbytes "+strconv.Itoa(vhob)+" "+tstamp+"\n")
				fmt.Fprintf(c, prefix+"."+hn+".virtualhosts."+vhost+".avgttfb "+strconv.Itoa(vhttfb/vhcnt)+" "+tstamp+"\n")
			}
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.requestcount "+strconv.Itoa(tl.data.count)+" "+tstamp+"\n")
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.rxbytes "+strconv.Itoa(tl.data.inbytes)+" "+tstamp+"\n")
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.txbytes "+strconv.Itoa(tl.data.outbytes)+" "+tstamp+"\n")
			fmt.Fprintf(c, prefix+"."+hn+".servertraffic.avgttfb "+strconv.Itoa(tl.data.ttfbsum/tl.data.count)+" "+tstamp+"\n")
			for hdl := range sTraffic.handlers {
				td := sTraffic.handlers[hdl]
				handler := strings.ReplaceAll(hdl, ".", "_")
				if handler == "" {
					handler = "static_content"
				}
				fmt.Fprintf(c, serverprefix+"."+hn+"."+handler+".requestcount "+strconv.Itoa(td.count)+" "+tstamp+"\n")
				fmt.Fprintf(c, serverprefix+"."+hn+"."+handler+".ttfbsum "+strconv.Itoa(td.ttfbsum)+" "+tstamp+"\n")
				fmt.Fprintf(c, serverprefix+"."+hn+"."+handler+".ttfbmin "+strconv.Itoa(td.ttfbmin)+" "+tstamp+"\n")
				fmt.Fprintf(c, serverprefix+"."+hn+"."+handler+".ttfbmax "+strconv.Itoa(td.ttfbmax)+" "+tstamp+"\n")
				fmt.Fprintf(c, serverprefix+"."+hn+"."+handler+".rxbytes "+strconv.Itoa(td.inbytes)+" "+tstamp+"\n")
				fmt.Fprintf(c, serverprefix+"."+hn+"."+handler+".rxbytesmin "+strconv.Itoa(td.inbytesmin)+" "+tstamp+"\n")
				fmt.Fprintf(c, serverprefix+"."+hn+"."+handler+".rxbytesmax "+strconv.Itoa(td.inbytesmax)+" "+tstamp+"\n")
				fmt.Fprintf(c, serverprefix+"."+hn+"."+handler+".txbytes "+strconv.Itoa(td.outbytes)+" "+tstamp+"\n")
				fmt.Fprintf(c, serverprefix+"."+hn+"."+handler+".txbytesmin "+strconv.Itoa(td.outbytesmin)+" "+tstamp+"\n")
				fmt.Fprintf(c, serverprefix+"."+hn+"."+handler+".txbytesmax "+strconv.Itoa(td.outbytesmax)+" "+tstamp+"\n")
			}
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

func (te *TrafficEntry) Aggregate(cnt, ib, imin, imax, ob, omin, omax, ttfb, ttfbmin, ttfbmax int, handler string) {
	te.mu.Lock()
	defer te.mu.Unlock()
	if te.handlers == nil {
		te.handlers = make(map[string]*TrafficData)
	}
	td, ok := te.handlers[handler]
	if !ok {
		td = &TrafficData{
			count:       cnt,
			inbytes:     ib,
			inbytesmin:  imin,
			inbytesmax:  imax,
			outbytes:    ob,
			outbytesmin: omin,
			outbytesmax: omax,
			ttfbsum:     ttfb,
			ttfbmin:     ttfbmin,
			ttfbmax:     ttfbmax,
		}
		te.handlers[handler] = td
		return
	}
	td.count += cnt
	td.inbytes += ib
	if imin < td.inbytesmin {
		td.inbytesmin = imin
	}
	if imax > td.inbytesmax {
		td.inbytesmax = imax
	}
	td.outbytes += omin
	if omin < td.outbytesmin {
		td.outbytesmin = omin
	}
	if omax > td.outbytesmax {
		td.outbytesmax = omax
	}
	td.ttfbsum += ttfb
	if ttfbmin < td.ttfbmin {
		td.ttfbmin = ttfbmin
	}
	if ttfbmax > td.ttfbmax {
		td.ttfbmax = ttfbmax
	}
}

func (te *TrafficEntry) Add(cnt, ib, ob, ttfb int, handler string) {
	te.mu.Lock()
	defer te.mu.Unlock()
	if te.handlers == nil {
		te.handlers = make(map[string]*TrafficData)
	}
	td, ok := te.handlers[handler]
	if !ok {
		td = &TrafficData{
			count:       cnt,
			inbytes:     ib,
			inbytesmin:  ib,
			inbytesmax:  ib,
			outbytes:    ob,
			outbytesmin: ob,
			outbytesmax: ob,
			ttfbsum:     ttfb,
			ttfbmin:     ttfb,
			ttfbmax:     ttfb,
		}
		te.handlers[handler] = td
		return
	}
	td.count += cnt
	td.inbytes += ib
	if ib < td.inbytesmin {
		td.inbytesmin = ib
	}
	if ib > td.inbytesmax {
		td.inbytesmax = ib
	}
	td.outbytes += ob
	if ob < td.outbytesmin {
		td.outbytesmin = ob
	}
	if ob > td.outbytesmax {
		td.outbytesmax = ob
	}
	td.ttfbsum += ttfb
	if ttfb < td.ttfbmin {
		td.ttfbmin = ttfb
	}
	if ttfb > td.ttfbmax {
		td.ttfbmax = ttfb
	}
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
			count:       1,
			inbytes:     ib,
			inbytesmin:  ib,
			inbytesmax:  ib,
			outbytes:    ob,
			outbytesmin: ob,
			outbytesmax: ob,
			ttfbsum:     ttfb,
			ttfbmin:     ttfb,
			ttfbmax:     ttfb,
		}
		tex := TrafficEntry{
			mu:       sync.Mutex{},
			handlers: map[string]*TrafficData{},
		}
		tex.handlers[handler] = &td
		tl.vhlist[vh] = &tex
		return
	}
	te.Add(1, ib, ob, ttfb, handler)
}
