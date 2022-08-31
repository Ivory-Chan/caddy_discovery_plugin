### Caddy Discovery Plugin

----

- caddy dynamic_upstreams 扩展插件，允许caddy动态的从注册中心获取服务信息
- 支持的注册中心有Nacos、Etcd、Consul
- 服务注册实现基于微服务框架Kratos，项目中的服务发现依赖kratos的服务发现接口实现

----

#### Part.1 Install

```shell
go get -u github.com/Ivory-Chan/caddy_discovery_plugin
```

#### Part.2 使用

本项目作为caddy的插件使用，具体使用方式可以参考caddy官方文档

#### Part.2 配置

本插件在caddy的module配置中属于dynamic_upstreams模块的一种扩展，配置层级处于dynamic_upstreams结构下级位置

```json
// Nacos 配置
{
  "dynamic_upstreams": {
    "source": "discovery",
    "service": "xxx.http",
    "refresh": "1m",
    "config": {
      "nacos": {
        "addr": "localhost",
        "port": 8848,
        "namespaceId": "public"
      }
    }
  }
}


// Etcd 配置
{
  "dynamic_upstreams": {
    "source": "discovery",
    "service": "xxx.http",
    "refresh": "1m",
    "config": {
      "Etcd": {
        "endpoints": "localhost"
      }
    }
  }
}


// Consul 配置
{
  "dynamic_upstreams": {
    "source": "discovery",
    "service": "xxx.http",
    "refresh": "1m",
    "config": {
      "consul": {
        "addr": "localhost",
        "schema": "http"
      }
    }
  }
}

```

#### Part.2 Caddy 配置示例
```json
{
  "apps": {
    "http": {
      "servers": {
        "auth": {
          "@id": "service.auth.http",
          "listen": [
            ":9000"
          ],
          "routes": [
            {
              "match": [
                {
                  "path": [
                    "/service/*"
                  ]
                }
              ],
              "handle": [
                {
                  "handler": "subroute",
                  "routes": [
                    {
                      "match": [
                        {
                          "path": [
                            "*auth/*"
                          ]
                        }
                      ],
                      "handle": [
                        {
                          "handler": "reverse_proxy",
                          "rewrite": {
                            "strip_path_prefix": "/service/auth"
                          },
                          "dynamic_upstreams": {
                            "source": "discovery",
                            "service": "auth.http",
                            "refresh": "1m",
                            "config": {
                              "nacos": {
                                "addr": "localhost",
                                "port": 8848,
                                "namespaceId": "public"
                              }
                            }
                          }
                        }
                      ]
                    }
                  ]
                }
              ]
            }
          ]
        }
      }
    }
  }
}
```