# vapollo
vapollo 是基于 viper 的远程配置，添加了对携程 apollo 配置系统的支持。指定 apollo 服务器/appId 后，可以从远端读取配置项，结合 'long-pulling' 方法，实时监控配置变更。更多关于 viper 的内容参见 https://github.com/spf13/viper

# 安装

```sh
go get github.com/kyeason/vapollo
```

**注意:** `vapollo` 使用 [Go Modules](https://github.com/golang/go/wiki/Modules) 管理组件的依赖项

# 如何使用

## 1. 快速初始化 Init
`Init` 方法从本地配置文件加载，并根据本地配置项，完成远端 Apollo 配置加载。此方法默认使用 `env` 变量，从配置文件内读取指定环境下的配置项。如：
```json
{
  "dev": {
    //...
  },
  "qa": {
    //...
  },
  //...
}
```
### 示例代码
```go
import (
  "github.com/kyeason/vapollo"
  "github.com/spf13/pflag"
  "github.com/spf13/viper"
  "log"
)

func main() {
    type Config struct {
      PropertyA string    `mapstructure:"property_a"`,
      PropertyB int       `mapstructure:"property_b"`,
      //...
    }
    appConfig := &Config{}
    err := vapollo.Init("app.json", "json", appConfig)
	if err != nil {
        log.Panicln("Failed reading configuration file:", err)
    } 
    //...
}
```
## 2. 自定义配置

### 初始化 Apollo

`InitApollo` 方法用于准备后续调用中与 Apollo 相关的参数，具体参数如下：
- 常规参数

```json
{
  "server": "127.0.0.1",
  "appId": "apollo-app",
  //注释中的参数为可选参数，内建使用对应的默认值
  //"cluster": "default",
  //"namespaceName": "application",
  //"ip": "",
  //"releaseKey": ""
}
```

- 结构体输出参数及变更通知管道
  
```go
func Struct(obj interface{}) Option
func Notify(notify chan bool) Option
````

初始化 Apollo 时，使用 `Struct` 选项方法传入结构体指针，可以实时将配置变更反射到目标结构体中; 如需要获取变更通知，可使用 `Notify` 选项传入 `chan bool` 类型的管道，并在新建的协程中监测其变化，示例如下：
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

### 初始化 Viper

`InitViperRemote` 方法接收 2 个参数：

1. apollo 对象，由 `InitApollo` 返回
2. Viper 的初始化选项列表（可变参数） `viper.Option`

当未提供配置结构体时, 只能使用 viper api 来读取配置项的值，如：

```go
vapollo.Remote.GetString("property_a")
vapollo.Remote.GetInt("property_b")
// ...
```

提供正确的配置结构体对象后，可以直接访问该对象的成员来读取配置项的值，如：

```go
appConfig.PropertyA
appConfig.PropertyB
// ...
```

> **注意:**  如果 Apollo 中的 Key 使用了点分命名方式如"a.b"，则无法读取该 Key（Viper 不支持从远程配置读取嵌套类型 Key）。因此可以指定 Viper 的 KeyDelimiter 参数，使用 ':' 代替默认的 '.'。

## 3.示例代码

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



