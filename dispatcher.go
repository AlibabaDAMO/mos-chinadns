//     Copyright (C) 2020, IrineSistiana
//
//     This file is part of mos-chinadns.
//
//     mosdns is free software: you can redistribute it and/or modify
//     it under the terms of the GNU General Public License as published by
//     the Free Software Foundation, either version 3 of the License, or
//     (at your option) any later version.
//
//     mosdns is distributed in the hope that it will be useful,
//     but WITHOUT ANY WARRANTY; without even the implied warranty of
//     MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//     GNU General Public License for more details.
//
//     You should have received a copy of the GNU General Public License
//     along with this program.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/miekg/dns"

	"github.com/IrineSistiana/mosdns/core/ipv6"
	"github.com/Sirupsen/logrus"
)

type dispatcher struct {
	bindAddr     string
	localServer  string
	remoteServer string

	localClient  dns.Client
	remoteClient dns.Client

	localAllowedIPList     *ipv6.NetList
	localBlockedIPList     *ipv6.NetList
	localBlockedDomainList *regExpDomainList
	remoteECS              *dns.EDNS0_SUBNET
}

const (
	queryTimeout = time.Second * 2
)

func initDispather(conf *Config) (*dispatcher, error) {
	d := new(dispatcher)

	if len(conf.BindAddr) == 0 || len(conf.LocalServer) == 0 || len(conf.RemoteServer) == 0 {
		return nil, errors.New("missing args: bind address or local server address or remote server address")
	}
	d.bindAddr = conf.BindAddr
	d.localServer = conf.LocalServer
	d.remoteServer = conf.RemoteServer

	d.localClient = dns.Client{
		SingleInflight: false,
	}
	d.remoteClient = dns.Client{
		SingleInflight: false,
	}
	switch conf.UseTCP {
	case "l":
		d.localClient.Net = "tcp"
	case "r":
		d.remoteClient.Net = "tcp"
	case "l_r":
		d.localClient.Net = "tcp"
		d.remoteClient.Net = "tcp"
	case "":
	default:
		logrus.Warnf("unknown mode [%s], use udp by default", conf.UseTCP)
	}

	if len(conf.LocalAllowedIPList) != 0 {
		allowedIPList, err := ipv6.NewNetListFromFile(conf.LocalAllowedIPList)
		if err != nil {
			return nil, fmt.Errorf("failed to load allowed ip file, %w", err)
		}
		d.localAllowedIPList = allowedIPList
	}

	if len(conf.LocalBlockedIPList) != 0 {
		blockIPList, err := ipv6.NewNetListFromFile(conf.LocalBlockedIPList)
		if err != nil {
			return nil, fmt.Errorf("failed to load block ip file, %w", err)
		}
		d.localBlockedIPList = blockIPList
	}

	if len(conf.LocalBlockedDomainList) != 0 {
		reList, err := loadRegExpDomainListFormFile(conf.LocalBlockedDomainList)
		if err != nil {
			return nil, fmt.Errorf("failed to load block domain file, %w", err)
		}
		d.localBlockedDomainList = reList
	}

	if len(conf.RemoteECSSubnet) != 0 {

		strs := strings.SplitN(conf.RemoteECSSubnet, "/", 2)
		if len(strs) != 2 {
			return nil, fmt.Errorf("invalid ECS address [%s], not a CIDR notation", conf.RemoteECSSubnet)
		}

		ip := net.ParseIP(strs[0])
		if ip == nil {
			return nil, fmt.Errorf("invalid ECS address [%s], invalid ip", conf.RemoteECSSubnet)
		}
		sourceNetmask, err := strconv.Atoi(strs[1])
		if err != nil || sourceNetmask > 128 || sourceNetmask < 0 {
			return nil, fmt.Errorf("invalid ECS address [%s], invalid net mask", conf.RemoteECSSubnet)
		}

		ednsSubnet := new(dns.EDNS0_SUBNET)
		// edns family: https://www.iana.org/assignments/address-family-numbers/address-family-numbers.xhtml
		// ipv4 = 1
		// ipv6 = 2
		if ip4 := ip.To4(); ip4 != nil {
			ednsSubnet.Family = 1
			ednsSubnet.SourceNetmask = uint8(sourceNetmask)
			ip = ip4
		} else {
			if ip6 := ip.To16(); ip6 != nil {
				ednsSubnet.Family = 2
				ednsSubnet.SourceNetmask = uint8(sourceNetmask)
				ip = ip6
			} else {
				return nil, fmt.Errorf("invalid ECS address [%s], it's not an ipv4 or ipv6 address", conf.RemoteECSSubnet)
			}
		}

		ednsSubnet.Code = dns.EDNS0SUBNET
		ednsSubnet.Address = ip
		ednsSubnet.SourceScope = 0
		d.remoteECS = ednsSubnet
	}

	return d, nil
}

func (d *dispatcher) ListenAndServe() error {
	return dns.ListenAndServe(d.bindAddr, "udp", d)
}

// ServeDNS impliment the interface
func (d *dispatcher) ServeDNS(w dns.ResponseWriter, q *dns.Msg) {
	r := d.serveDNS(q)
	if r != nil {
		w.WriteMsg(r)
	}
}

func isUnusualType(q *dns.Msg) bool {
	return q.Opcode != dns.OpcodeQuery || len(q.Question) != 1 || q.Question[0].Qclass != dns.ClassINET || (q.Question[0].Qtype != dns.TypeA && q.Question[0].Qtype != dns.TypeAAAA)
}

func isBlockedDomain(q *dns.Msg, reList *regExpDomainList) bool {
	for i := range q.Question {
		if reList.match(q.Question[i].Name) {
			return true
		}
	}
	return false
}

// serveDNS: r might be nil
func (d *dispatcher) serveDNS(q *dns.Msg) *dns.Msg {
	requestLogger := logrus.WithFields(logrus.Fields{
		"id":       q.Id,
		"question": q.Question,
	})

	ctx, cancelQuery := context.WithTimeout(context.Background(), queryTimeout)
	defer cancelQuery()

	if isUnusualType(q) || isBlockedDomain(q, d.localBlockedDomainList) {
		r, rtt, err := d.queryRemote(ctx, q)
		if err != nil {
			requestLogger.Warnf("remote server failed with err: %v", err)
			return nil
		}
		requestLogger.Debugf("get reply remote local server, rtt: %s", rtt)
		return r
	}

	resChan := make(chan *dns.Msg, 1)

	wgChan := make(chan struct{}, 0)
	wg := sync.WaitGroup{}
	wg.Add(2) // local and remote

	// local
	go func() {
		defer wg.Done()
		res, rtt, err := d.queryLocal(ctx, q)
		if err != nil {
			requestLogger.Warnf("local server failed with err: %v", err)
			return
		}
		requestLogger.Debugf("get reply from local server, rtt: %s", rtt)

		if d.dropLoaclRes(res, requestLogger) {
			requestLogger.Debug("local result droped")
			return
		}

		select {
		case resChan <- res:
		default:
		}
	}()

	// remote
	go func() {
		defer wg.Done()
		res, rtt, err := d.queryRemote(ctx, q)
		if err != nil {
			requestLogger.Warnf("remote server failed with err: %v", err)
			return
		}
		requestLogger.Debugf("get reply from local server, rtt: %s", rtt)

		select {
		case resChan <- res:
		default:
		}
	}()

	// watcher
	go func() {
		wg.Wait()
		close(wgChan)
	}()

	select {
	case r := <-resChan:
		return r
	case <-wgChan:
		r := new(dns.Msg)
		r.SetReply(q)
		r.Rcode = dns.RcodeServerFailure
		return r
	case <-ctx.Done():
		requestLogger.Debugf("query failed %v", ctx.Err())
		return nil
	}
}

func (d *dispatcher) queryLocal(ctx context.Context, q *dns.Msg) (*dns.Msg, time.Duration, error) {
	return d.localClient.ExchangeContext(ctx, q, d.localServer)
}

//queryRemote WARNING: to save memory we may modify q directly.
func (d *dispatcher) queryRemote(ctx context.Context, q *dns.Msg) (*dns.Msg, time.Duration, error) {
	if d.remoteECS != nil {
		opt := q.IsEdns0()
		if opt == nil { // we need a new opt
			o := new(dns.OPT)
			o.SetUDPSize(2048) // TODO: is this big enough?
			o.Hdr.Name = "."
			o.Hdr.Rrtype = dns.TypeOPT
			o.Option = []dns.EDNS0{d.remoteECS}
			q.Extra = append(q.Extra, o)
		} else {
			var hasECS bool = false // check if msg q already has a ECS section
			for o := range opt.Option {
				if opt.Option[o].Option() == dns.EDNS0SUBNET {
					hasECS = true
					break
				}
			}

			if !hasECS {
				opt.Option = append(opt.Option, d.remoteECS)
			}
		}
	}

	return d.remoteClient.ExchangeContext(ctx, q, d.remoteServer)
}

func (d *dispatcher) dropLoaclRes(res *dns.Msg, requestLogger *logrus.Entry) bool {
	if res == nil {
		requestLogger.Debug("local result is nil")
		return true
	}

	if res.Rcode != dns.RcodeSuccess {
		requestLogger.Debug("local result Rcode != 0")
		return true
	}

	if len(res.Answer) == 0 {
		requestLogger.Debug("local result has empty answer")
		return true
	}

	if anwsersMatchNetList(res.Answer, d.localBlockedIPList) {
		requestLogger.Debug("local result is blocked")
		return true
	}

	if d.localAllowedIPList != nil {
		if anwsersMatchNetList(res.Answer, d.localAllowedIPList) {
			requestLogger.Debug("local result is alloweded")
			return false
		}
		requestLogger.Debug("local result can not be alloweded")
		return true
	}

	return false
}

func anwsersMatchNetList(anwser []dns.RR, list *ipv6.NetList) bool {
	if list == nil {
		return false
	}

	for i := range anwser {
		switch tmp := anwser[i].(type) {
		case *dns.A:
			ipv6, err := ipv6.Conv(tmp.A)
			if err != nil {
				continue
			}
			if list.Contains(ipv6) {
				return true
			}
		case *dns.AAAA:
			ipv6, err := ipv6.Conv(tmp.AAAA)
			if err != nil {
				continue
			}
			if list.Contains(ipv6) {
				return true
			}
		default:
			logrus.Warn("unknown answer type")
		}
	}
	return false
}
