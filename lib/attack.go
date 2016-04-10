package stress

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

var (
	DefaultRedirects = 10
	DefaultTimeout   = 30 * time.Second
	DefaultLocalAddr = net.IPAddr{IP: net.IPv4zero}
)

var (
	remain int64
)

var DefaultAttacker = NewAttacker(DefaultRedirects, DefaultTimeout, DefaultLocalAddr)

type Attacker struct {
	client http.Client
}

func NewAttacker(redirects int, timeout time.Duration, laddr net.IPAddr) *Attacker {
	return &Attacker{http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			Dial: (&net.Dialer{
				Timeout:   timeout,
				KeepAlive: 30 * time.Second,
				LocalAddr: &net.TCPAddr{IP: laddr.IP, Zone: laddr.Zone},
			}).Dial,
			ResponseHeaderTimeout: timeout,
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
			TLSHandshakeTimeout: 10 * time.Second,
		},
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			if len(via) > redirects {
				return fmt.Errorf("stopped after %d redirects", redirects)
			}
			return nil
		},
	}}
}

func AttackRate(targets Targets, rate uint64, du time.Duration) Results {
	return DefaultAttacker.AttackRate(targets, rate, du)
}

func (a *Attacker) AttackRate(tgts Targets, rate uint64, du time.Duration) Results {
	hits := int(rate * uint64(du.Seconds()))
	resc := make(chan Result)
	throttle := time.NewTicker(time.Duration(1e9 / rate))
	defer throttle.Stop()
	for i := 0; i < hits; i++ {
		<-throttle.C
		go func(tgt Target) { resc <- a.hit(tgt) }(tgts[i%len(tgts)])
	}
	results := make(Results, 0, hits)
	for len(results) < cap(results) {
		results = append(results, <-resc)
	}

	return results.Sort()
}

func (a *Attacker) hit(tgt Target) (res Result) {
	req, err := tgt.Request()
	if err != nil {
		res.Error = err.Error()
		return res
	}

	res.Timestamp = time.Now()
	r, err := a.client.Do(req)
	if err != nil {
		res.Error = err.Error()
		return res
	}

	res.BytesOut = uint64(req.ContentLength)
	res.Code = uint16(r.StatusCode)
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		if res.Code >= 300 || res.Code < 200 {
			res.Error = fmt.Sprintf("%s %s: %s", tgt.Method, tgt.URL, r.Status)
		}
		return res
	}

	res.Latency = time.Since(res.Timestamp)
	res.BytesIn = uint64(len(body))
	if res.Code >= 300 || res.Code < 200 {
		res.Error = fmt.Sprintf("%s %s: %s", tgt.Method, tgt.URL, r.Status)
	} else {
		if strings.Contains(tgt.File, "md5") {
			kv := strings.Split(tgt.File, ":")
			if len(kv) == 2 {
				if kv[1] != "" && len(kv[1]) == 32 {
					h := md5.New()
					h.Write(body)
					rspMd5 := hex.EncodeToString(h.Sum(nil))
					if rspMd5 != kv[1] {
						res.Code = 250
						res.Error = fmt.Sprintf("%s %s:MD5 not matched", tgt.Method, tgt.URL)
					}
				}
			}
		}
	}

	if res.Code >= 250 || res.Code < 200 {
		log.Printf("%s \n", res.Error)
	}

	return res
}

func AttackConcy(tgts Targets, concurrency uint64, number uint64) Results {
	return DefaultAttacker.AttackConcy(tgts, concurrency, number)
}

func (a *Attacker) AttackConcy(tgts Targets, concurrency uint64, number uint64) Results {
	retsc := make(chan Results)
	atomic.StoreInt64(&remain, int64(number))

	if concurrency > number {
		concurrency = number
	}

	var i uint64
	for i = 0; i < concurrency; i++ {
		go func(tgts Targets) {
			retsc <- a.shoot(tgts)
		}(tgts)
	}
	results := make(Results, 0, number)
	for i = 0; i < concurrency; i++ {
		results = append(results, <-retsc...)
	}
	return results.Sort()
}

func (a *Attacker) shoot(tgts Targets) Results {
	return nil
}
