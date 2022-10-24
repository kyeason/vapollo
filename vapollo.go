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
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"
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

	// If a struct interface was provided, vapollo will unmarshal the
	// key/values to the object
	object interface{}
	notify chan bool
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
	apply(a *Apollo)
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
	return optionFunc(func(a *Apollo) {
		a.server = s
	})
}

func AppId(app string) Option {
	return optionFunc(func(a *Apollo) {
		a.appID = app
	})
}

func Struct(obj interface{}) Option {
	return optionFunc(func(a *Apollo) {
		a.object = obj
	})
}

func Notify(notify chan bool) Option {
	return optionFunc(func(a *Apollo) {
		a.notify = notify
	})
}

// Init initialize configuration from local file, assuming that there is a variable "env" that determines the
// configuration of a specific runtime environment. e.g.
//
//	{
//		"dev": {
//			...
//		},
//		"qa": {
//			...
//		},
//		...
//	}
//
//	Parameters
//		fileName:  Name of local file
//		fileType:  Type of file contents, e.g. "json", "yaml", "properties" etc. , see https://github.com/spf13/viper
//		apolloKey: Apollo sub configuration key
//		dStruct:   Struct interface corresponding to the structured data
//
//	Usage:
//		Init("app.json", "json", "apollo", nil)
//		Init("app.yml", "yaml", "", &config)
func Init(fileName, fileType, apolloKey string, dStruct interface{}) (v *viper.Viper, err error) {
	pflag.String("env", "prod", "Running environment(dev/qa/pre/prod)")
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	env := viper.GetString("env")
	viper.AddConfigPath(filepath.Dir(os.Args[0]))
	viper.SetConfigName(filepath.Base(fileName))
	viper.SetConfigType(fileType)
	err = viper.ReadInConfig()
	if err != nil {
		log.Panicln("Failed reading local config: ", err)
	}
	key := apolloKey
	if len(apolloKey) > 0 {
		key = apolloKey + "."
	}
	v = viper.Sub(env)
	notify := make(chan bool)
	opts := []Option{
		Server(v.GetString(key + "ip")),
		AppId(v.GetString(key + "appId")),
		NamespaceName(v.GetString(key + "namespaceName")),
		Struct(dStruct),
		Notify(notify),
	}
	apollo := InitApollo(opts...)
	v, err = InitViperRemote(apollo, viper.KeyDelimiter(":"))
	_ = v.BindPFlags(pflag.CommandLine)
	if err != nil {
		log.Panicln("Failed init apollo config: ", err)
	}
	// Waiting for remote configuration
	go func(quit chan bool) {
		time.Sleep(time.Millisecond * 800)
		quit <- true
	}(notify)
	select {
	case <-notify:
		return v, nil
	}
	return v, errors.New("failed retrieving remote config:[EMPTY]")
}

// InitApollo initiate apollo with options which server, appId are mandatory.
// e.g. InitApollo(vapollo.Server("127.0.0.1"), vapollo.AppID("TestApp"))
func InitApollo(opts ...Option) *Apollo {
	apollo := &Apollo{
		cluster:       "default",
		namespaceName: "application",
	}
	for _, opt := range opts {
		opt.apply(apollo)
	}

	if apollo.server == "" || apollo.appID == "" {
		log.Panicln("Can't not init apollo, missing arguments(server, appId)")
		return nil
	}

	apollo.notifications = []notification{
		{
			NamespaceName:  apollo.namespaceName,
			NotificationID: -1,
		},
	}

	return apollo
}

var Remote *viper.Viper

// InitViperRemote initiate viper and apollo remote.
// Here viper.Options are exposed because if any keys of an app are in nested
// style like "a.b", then viper can NOT read it correctly. So we can set the
// KeyDelimiter option of viper to ':' or else instead of '.'
func InitViperRemote(apollo *Apollo, opts ...viper.Option) (*viper.Viper, error) {
	if apollo == nil {
		log.Panicln("Can not init viper remote with apollo: Please check and init apollo first")
	}

	if !strings.Contains(apollo.server, "http") && !strings.Contains(apollo.server, "https") {
		apollo.server = "http://" + apollo.server
	}
	viper.RemoteConfig = apollo
	if len(opts) > 0 {
		Remote = viper.NewWithOptions(opts...)
	} else {
		Remote = viper.GetViper()
	}

	err := Remote.AddRemoteProvider("consul", apollo.server, apollo.appID)
	if err != nil {
		return nil, err
	}
	Remote.SetConfigType("json")
	// Watch modifications on remote
	_ = Remote.WatchRemoteConfigOnChannel()
	// Map values to object member if an object interface was provided
	_ = apollo.ParseStruct(viper.AllSettings(), Remote.AllSettings())
	return Remote, nil
}

func (a *Apollo) Get(rp viper.RemoteProvider) (io.Reader, error) {
	b, err := a.load()
	r := bytes.NewReader(b)
	return r, err
}

func (a *Apollo) Watch(rp viper.RemoteProvider) (io.Reader, error) {
	b, err := a.loadFromCache()
	r := bytes.NewReader(b)
	return r, err
}

func (a *Apollo) WatchChannel(rp viper.RemoteProvider) (<-chan *viper.RemoteResponse, chan bool) {
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
					log.Printf("Watch remote channel error=%v", err)
					continue
				}

				// read content if modified(notification with HTTP status 200)
				if modified {
					err = Remote.ReadRemoteConfig()
					if err != nil {
						log.Println("Failed reading apollo config: ", err)
						continue
					}
					if a.object != nil {
						settings := Remote.AllSettings()
						log.Printf("All settings: %v", settings)
						// Parse all settings to the struct interface provided
						_ = a.ParseStruct(nil, settings)
					}
					if a.notify != nil {
						a.notify <- true
					}
				}
			}
		}
	}(ch, quitCh)
	return ch, quitCh
}

func (a *Apollo) getNotificationsBody() string {
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

func JsonStructInMapHookFunc() mapstructure.DecodeHookFunc {
	return func(f reflect.Value, t reflect.Value) (interface{}, error) {
		if f.Kind() == reflect.String && (t.Kind() == reflect.Struct || t.Kind() == reflect.Map) {
			o := map[string]interface{}{}
			err := json.Unmarshal([]byte(f.String()), &o)
			if err != nil {
				return f.Interface(), err
			}
			return o, nil
		} else if f.Kind() == reflect.String && t.Kind() != reflect.String {
			result := reflect.New(t.Type()).Interface()
			result, err := strconv.Atoi(f.String())
			if err != nil {
				return f.Interface(), nil
			}
			return result, nil
		}
		return f.Interface(), nil
	}
}

func (a *Apollo) ParseStruct(local map[string]interface{}, remote map[string]interface{}) error {
	if a.object == nil {
		return errors.New("failed parsing struct: no interface")
	}
	deCfg := &mapstructure.DecoderConfig{
		DecodeHook: JsonStructInMapHookFunc(),
		Result:     a.object,
	}
	d, _ := mapstructure.NewDecoder(deCfg)
	if local != nil {
		err := d.Decode(local)
		if err != nil {
			log.Printf("Read LOCAL config with error=%v", err)
		}
	}
	err := d.Decode(remote)
	if err != nil {
		log.Printf("Read REMOTE config with error=%v", err)
	}
	return err
}
