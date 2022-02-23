# vapollo
vapollo 是基于 viper 的远程配置，添加了对携程 apollo 配置系统的支持。指定 apollo 服务器/appId 后，可以从远端读取配置项，结合 'long-pulling' 方法，实时监控配置变更。更多关于 viper 的内容参见 https://github.com/spf13/viper

# 安装

```sh
go get github.com/kyeason/vapollo
```

**注意:** `vapollo` 使用 [Go Modules](https://github.com/golang/go/wiki/Modules) 管理组件的依赖项

# 如何使用

## 初始化 Apollo

`InitApollo` 方法用于准备后续调用中与 Apollo 相关的参数，具体参数如下：

```json
{
  "server": "127.0.0.1",
  "appId": "apollo-app",
  // 注释中的参数为可选参数，内建使用对应的默认值
  // "cluster": "default",
  // "namespaceName": "application",
  // "ip": "",
  // "releaseKey": ""
}
```

## 初始化 Viper

`InitViperRemote` 方法接收 3 个参数：

1. apollo 对象，由 `InitApollo` 返回
2. 用于读取配置的结构体，结构体成员绑定到配置项 key
3. Viper 的初始化选项列表（可变参数） `viper.Option`

当未提供配置结构体时, 只能使用 viper api 来读取配置项的值，如：

```go
v.GetString("property_a")
v.GetInt("property_b")
// ...
```

提供正确的配置结构体对象后，可以直接访问该对象的成员来读取配置项的值，如：

```go
appConfig.PropertyA
appConfig.PropertyB
// ...
```

> **注意:**  如果 Apollo 中的 Key 使用了点分命名方式如"a.b"，则无法读取该 Key（Viper 不支持从远程配置读取嵌套类型 Key）。因此可以指定 Viper 的 KeyDelimiter 参数，使用 ':' 代替默认的 '.'。

## 示例代码

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
    //
    v, err = vapollo.InitViperRemote(apollo, appConfig, viper.KeyDelimiter(":"))
}
```



