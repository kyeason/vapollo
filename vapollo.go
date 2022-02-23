// Copyright © 2022 Carwyn Kong <kong__mo@163.com>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// vapollo is a remote provider of viper(https://github.com/spf13/viper)
// for c-trip apollo config
// It reads configurations of a apollo appId, and watch modifications

package vapollo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
)

// Apollo parameters definition
type Apollo struct {
	cluster       string
	server        string
	namespaceName string
	appID         string
	releaseKey    string
	ip            string
	notifications []notification

	// Pointer of viper instance
	viper        *viper.Viper
	// If a struct interface was provided, vapollo will unmarshal the
	// key/values to the object
	object        interface{}
}

// apollo notification structure
type notification struct {
	NamespaceName  string `json:"namespaceName"`
	NotificationID int    `json:"notificationId"`
}

// apollo configuration content structure
type apolloResponse struct {
	Configurations json.RawMessage `json:"configurations"`
	ReleaseKey     string          `json:"releaseKey"`
	Cluster        string          `json:"cluster"`
	NamespaceName  string          `json:"namespaceName"`
	AppID          string          `json:"appId"`
}

type Option interface {
	apply(a* Apollo)
}

type optionFunc func(a *Apollo)

func (fn optionFunc) apply(a *Apollo) {
	fn(a)
}

func Cluster(c string) Option {
	return optionFunc(func(a *Apollo) {
		a.cluster = c
	})
}

func NamespaceName(n string) Option {
	return optionFunc(func(a *Apollo) {
		a.namespaceName = n
	})
}

func Server(s string) Option {
	return optionFunc(func(a* Apollo) {
		a.server = s
	})
}

func AppId(app string) Option {
	return optionFunc(func(a* Apollo) {
		a.appID = app
	})
}

// InitApollo initiate apollo with options which server, appId are mandatory.
// e.g. InitApollo(vapollo.Server("127.0.0.1"), vapollo.AppID("TestApp"))
func InitApollo(opts ... Option) *Apollo {
	apollo := &Apollo {
		cluster: "default",
		namespaceName: "application",
	}
	for _, opt := range opts {
		opt.apply(apollo)
	}

	if apollo.server == "" || apollo.appID == "" {
		log.Panicln("Can't not init apollo, missing arguments(server, appId)")
		return nil
	}

	apollo.notifications = []notification {
		{
			NamespaceName:  apollo.namespaceName,
			NotificationID: -1,
		},
	}

	return apollo
}

// InitViperRemote initiate viper and apollo remote.
// Here viper.Options are exposed because if any keys of a app are in nested
// style like "a.b", then viper can NOT read it correctly. So we can set the
// KeyDelimiter option of viper to ':' or else instead of '.'
func InitViperRemote(apollo *Apollo, obj interface{}, opts ...viper.Option) (*viper.Viper, error) {
	if apollo == nil {
		log.Panicln("Can not init viper remote with apollo: Please check and init apollo first")
	}

	if !strings.Contains(apollo.server, "http") || !strings.Contains(apollo.server, "https") {
		apollo.server = "http://" + apollo.server
	}
	viper.RemoteConfig = apollo
	if len(opts) > 0 {
		apollo.viper = viper.NewWithOptions(opts...)
	} else {
		apollo.viper = viper.New()
	}
	err := apollo.viper.AddRemoteProvider("consul", apollo.server, apollo.appID)
	if err != nil {
		return nil, err
	}
	apollo.viper.SetConfigType("json")
	if err := apollo.viper.ReadRemoteConfig(); err != nil {
		return nil, err
	}
	// Map values to object member if a object interface was provided
	if obj != nil {
		apollo.object = obj
		_ = apollo.viper.Unmarshal(apollo.object)
	}
	// Watch modifications on remote
	_ = apollo.viper.WatchRemoteConfigOnChannel()
	return apollo.viper, nil
}

func (a Apollo) Get(rp viper.RemoteProvider) (io.Reader, error) {
	b, err := a.load()
	r := bytes.NewReader(b)
	return r, err
}

func (a Apollo) Watch(rp viper.RemoteProvider) (io.Reader, error) {
	b, err := a.loadFromCache()
	r := bytes.NewReader(b)
	return r, err
}

func (a Apollo) WatchChannel(rp viper.RemoteProvider) (<-chan *viper.RemoteResponse, chan bool) {
	ch := make(chan *viper.RemoteResponse)
	quitCh := make(chan bool)
	go func(vc chan<- *viper.RemoteResponse, quit <-chan bool) {
		for {
			select {
			case <-quit:
				return
			default:
				// get modification notify from apollo
				modified, err := a.getNotifications()
				if err != nil {
					vc <- &viper.RemoteResponse{Error: err}
					return
				}

				// read content if modified(notification with HTTP status 200)
				if modified {
					err = a.viper.ReadRemoteConfig()
					if err != nil {
						log.Println("Failed reading apollo config: ", err)
						continue
					}
					if a.object != nil {
						err = a.viper.Unmarshal(a.object)
						if err != nil {
							log.Println("Failed saving apollo changes to object: ", err)
						}
					}
				}
			}
		}
	}(ch, quitCh)
	return ch, quitCh
}

func (a Apollo) getNotificationsBody() string {
	b, err := json.Marshal(a.notifications)
	if err != nil {
		return ""
	}
	return string(b)
}

func (a *Apollo) loadFromCache() ([]byte, error) {
	uri := fmt.Sprintf(
		"%s/configfiles/json/%s/%s/%s",
		a.server,
		a.appID,
		a.cluster,
		a.namespaceName,
	)

	params := url.Values{}
	if a.ip != "" {
		params.Add("ip", a.ip)
		uri = uri + "?" + params.Encode()
	}
	return a.get(uri)
}

func (a *Apollo) load() ([]byte, error) {
	uri := fmt.Sprintf(
		"%s/configs/%s/%s/%s",
		a.server,
		a.appID,
		a.cluster,
		a.namespaceName,
	)

	params := url.Values{}
	if a.ip != "" {
		params.Add("ip", a.ip)
		uri = uri + "?" + params.Encode()
	}

	return a.get(uri)
}

// get Read content of the specified appId from apollo
func (a *Apollo) get(uri string) ([]byte, error) {
	resp, err := http.Get(uri)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var apolloResp apolloResponse
	if err := json.Unmarshal(b, &apolloResp); err != nil {
		return nil, err
	}

	a.releaseKey = apolloResp.ReleaseKey
	return apolloResp.Configurations, nil
}

// getNotifications Read notification of the specified appId from apollo
func (a *Apollo) getNotifications() (bool, error) {
	params := url.Values{}
	params.Add("appId", a.appID)
	params.Add("cluster", a.cluster)
	params.Add("notifications", a.getNotificationsBody())
	resp, err := http.Get(fmt.Sprintf(
		"%s/notifications/v2?%s",
		a.server,
		params.Encode(),
	))
	if err != nil {
		return false, err
	}

	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotModified {
		return false, nil
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	err = json.Unmarshal(b, &a.notifications)
	return true, err
}
