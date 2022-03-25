# vapollo
vapollo is a remote provider of viper for c-trip apollo. It reads configurations of a apollo server/appId, and watch modifications on remote server with 'long-pulling' method.

For more information of viper, see https://github.com/spf13/viper
# Install

```sh
go get github.com/kyeason/vapollo
```

**Note:** `vapollo` uses [Go Modules](https://github.com/golang/go/wiki/Modules) to manage dependencies.

# Usage

## Initialize Apollo

The `InitApollo` method prepares required parameters for further step. Usually there are following parameters:
- Apollo parameters
```json
{
  "server": "127.0.0.1",
  "appId": "apollo-app",
  // Parameters commented are optional which has built-in default value
  // "cluster": "default",
  // "namespaceName": "application",
  // "ip": "",
  // "releaseKey": ""
}
```

- Output structure and notification channel
  
```go
func Struct(obj interface{}) Option
func Notify(notify chan bool) Option
````

It's possible to reflect remote changing by passing a `Struct` option to `InitApollo` method.
Further, if we want to know about the changing, pass a `Notify` option with a `chan bool` channel, for example:
```go
type config struct {
	PropA `mapstructure:"prop_a"`
	PropB `mapstructure:"prop_b"`
}
configObj := config{}
watchingCh := make(chan bool)
go func(ch <- chan bool ) {
    for {
        log.Printf("Watching channel")
        select {
        case changed := <- ch:
            if changed {
            log.Printf("Parsed values=%v", configObj)
            } else {
            log.Printf("Nothing changed")
            }
        }
    }
}(watchingCh)
opts := []Option {
    vapollo.Server("xxx.xxx.xxx.xxx"),
    vapollo.AppId("app_xxx"),
    vapollo.Struct(&appConfig),
    vapollo.Notify(watchingCh),
}
apollo := InitApollo(opts...)
```
## Initialize Viper

The `InitViperRemote` method takes 2 parameters: 

1. apollo object returned by `InitApollo`
2. Various `viper.Option`

When the configuration struct is not provided, then values can be fetched using viper api `v.GetString("property_a")`, `v.GetInt("property_b")`.

```go
vapollo.Remote.GetString("property_a")
vapollo.Remote.GetInt("property_b")
// ...
```
Otherwise, values can be accessed directly from the struct object, e.g. `appConfig.PropertyA`, `appConfig.PropertyB`.

```go
appConfig.PropertyA
appConfig.PropertyB
// ...
```

> **Note:**  If any keys of a app are in nested style like "a.b", then viper can NOT read it correctly. So we can set the KeyDelimiter option of viper to ':' or else instead of '.'

## Sample code

```go
import (
  "github.com/kyeason/vapollo"
  "github.com/spf13/pflag"
  "github.com/spf13/viper"
  "log"
)

func main() {
    viper.SetConfigFile("app.properties")
    viper.SetConfigType("json")
    err := viper.ReadInConfig()
    if err != nil {
      log.Panicln("Failed reading configuration file:", err)
      return
    }

    type Config struct {
      PropertyA string    `mapstructure:"property_a"`,
      PropertyB int       `mapstructure:"property_b"`,
      //...
    }
    appConfig := &Config{}
    opts := []Option {
    	vapollo.Server("xxx.xxx.xxx.xxx"),
    	vapollo.AppId("app_xxx"),
        vapollo.Struct(&appConfig),
    }
    apollo := vapollo.InitApollo(opts...)
    v, err = vapollo.InitViperRemote(apollo, viper.KeyDelimiter(":"))
}
```



