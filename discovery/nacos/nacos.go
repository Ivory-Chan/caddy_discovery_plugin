package nacos

import (
	nacos "github.com/go-kratos/kratos/contrib/registry/nacos/v2"
	"github.com/nacos-group/nacos-sdk-go/clients"
	"github.com/nacos-group/nacos-sdk-go/common/constant"
	"github.com/nacos-group/nacos-sdk-go/vo"
	"go.uber.org/zap"
)

type Config struct {
	Addr        string `json:"addr,omitempty"`
	Port        uint64 `json:"port,omitempty"`
	NamespaceId string `json:"namespaceId,omitempty"`
}

// New 创建 etcd 服务发现
func New(config *Config, logger *zap.Logger) (*nacos.Registry, error) {

	var namespaceId string

	if config == nil {
		return nil, nil
	}

	sc := []constant.ServerConfig{
		*constant.NewServerConfig(config.Addr, config.Port),
	}

	if config.NamespaceId == "" {
		namespaceId = "public"
	} else {
		namespaceId = config.NamespaceId
	}

	cc := &constant.ClientConfig{
		//NamespaceId:         "public",
		NamespaceId:          namespaceId,
		TimeoutMs:            5000,
		NotLoadCacheAtStart:  true,
		LogDir:               "./nacos_cache/log",
		CacheDir:             "./nacos_cache/cache",
		LogLevel:             "error",
		UpdateCacheWhenEmpty: true,
	}

	client, err := clients.NewNamingClient(
		vo.NacosClientParam{
			ClientConfig:  cc,
			ServerConfigs: sc,
		},
	)

	if err != nil {
		logger.Panic(err.Error())
	}

	return nacos.New(client), nil
}
