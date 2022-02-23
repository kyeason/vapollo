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

## Initialize Viper

The `InitViperRemote` method takes 3 parameters: 

1. apollo object returned by `InitApollo`
2. Configuration struct that mapped to the keys
3. Various `viper.Option`

When the configuration struct is not provided, then values can be fetched using viper api `v.GetString("property_a")`, `v.GetInt("property_b")`.

Otherwise, values can be accessed directly from the struct object, e.g. `appConfig.PropertyA`, `appConfig.PropertyB`.

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
    pflag.String("env", "prod", "Running environment(dev/qa/pre/prod)")
    pflag.Parse()
    viper.BindPFlags(pflag.CommandLine)
    env := viper.GetString("env")
    viper.SetConfigFile("app.properties")
    viper.SetConfigType("json")
    err := viper.ReadInConfig()
    if err != nil {
      log.Panicln("Failed reading configuration file:", err)
      return
    }
    v := viper.Sub(env)

    type Config struct {
      PropertyA string    `mapstructure:"property_a"`,
      PropertyB int       `mapstructure:"property_b"`,
      //...
    }
    appConfig := &Config{}
    apollo := vapollo.InitApollo(vapollo.Server(v.GetString("server")),vapollo.AppId(v.GetString("appId")))
    v, err = vapollo.InitViperRemote(apollo, appConfig, viper.KeyDelimiter(":"))
}
```



