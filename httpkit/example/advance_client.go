package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/xkeyideal/gokit/httpkit"
)

var httpClients *sync.Map

func getHttpClient(addr string) *httpkit.AdvanceHttpClient {
	c, ok := httpClients.Load(addr)

	if ok {
		return c.(*httpkit.AdvanceHttpClient)
	}

	cc := httpkit.NewAdvanceHttpClient("http", "service.google.com", ConnTimout, nil)

	httpClients.Store(addr, cc)

	return cc
}

func init() {
	httpClients = new(sync.Map)
}

type Version struct {
	Version string `json:"code"`
	Host    int    `json:"OnLineHost"`
}

type VersionRealData struct {
	Code int       `json:"code"`
	Data []Version `json:"data"`
}

type VersionData struct {
	Data VersionRealData `json:"data"`
}

type AppVersion struct {
	Version string
	Host    int
	Va      int
	Vb      int
	Vc      int
	Vd      int
}

func AdvanceClient() ([]string, error) {
	setting := httpkit.NewAdvanceSettings(RwTimeout, Retry, RetryInterval)
	setting = setting.SetHeader("cache-control", "no-cache").SetHeader("token", "XXXX-xxxxx-uuuuuuuuu-yyyyyyyyyyy")
	setting = setting.SetParam("name", "advance_client")

	client := getHttpClient("service.google.com")
	resp, err := client.Get("/googlegateway/public/api/queryservice/versionlist", setting)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("version api error: %s, code: %d", resp.Status, resp.StatusCode)
	}

	data := VersionData{}
	err = json.Unmarshal(resp.Body, &data)
	if err != nil {
		return nil, err
	}

	appVersions := []AppVersion{}
	for _, version := range data.Data.Data {
		v := AppVersion{
			Version: version.Version,
			Host:    version.Host,
		}
		ss := strings.Split(version.Version, ".")
		for i, s := range ss {
			num, _ := strconv.ParseInt(s, 10, 32)
			if i == 0 {
				v.Va = int(num)
			} else if i == 1 {
				v.Vb = int(num)
			} else if i == 2 {
				v.Vc = int(num)
			} else if i == 3 {
				v.Vd = int(num)
			}
		}
		appVersions = append(appVersions, v)
	}

	sort.Slice(appVersions, func(i, j int) bool {
		if appVersions[i].Va == appVersions[j].Va {
			if appVersions[i].Vb == appVersions[j].Vb {
				if appVersions[i].Vc == appVersions[j].Vc {
					return appVersions[i].Vd > appVersions[j].Vd
				}
				return appVersions[i].Vc > appVersions[j].Vc
			}
			return appVersions[i].Vb > appVersions[j].Vb
		}
		return appVersions[i].Va > appVersions[j].Va
	})

	num := len(appVersions)
	if num == 0 {
		return []string{}, nil
	}

	vs := []string{}
	for i := 0; i < 5 && i < num; i++ {
		if appVersions[i].Host == 0 && len(vs) != 0 {
			break
		}
		vs = append(vs, appVersions[i].Version)
	}

	return vs, nil
}
