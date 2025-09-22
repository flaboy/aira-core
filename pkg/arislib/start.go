package arislib

import (
	"github.com/flaboy/aira/aira-core/pkg/cluster"
	"github.com/flaboy/aira/aira-core/pkg/config"
	"github.com/flaboy/aira/aira-core/pkg/database"
	"github.com/flaboy/aira/aira-core/pkg/mailer"
	"github.com/flaboy/aira/aira-core/pkg/redis"
	"github.com/flaboy/aira/aira-core/pkg/storage"
	"github.com/flaboy/aira/aira-core/pkg/tasklib"
)

func Start(cfg *config.InfraConfig) error {
	config.Config = cfg

	// 启动基础设施组件
	if err := database.Start(); err != nil {
		return err
	}

	if err := storage.Init(); err != nil {
		return err
	}

	if err := redis.InitRedis(); err != nil {
		return err
	}

	if err := tasklib.Init(); err != nil {
		return err
	}

	if err := tasklib.StartServer(); err != nil {
		return err
	}

	if err := cluster.Start(); err != nil {
		return err
	}

	mailer.InitSMTP()

	return nil
}

// 兼容性函数 - 保持向后兼容
func Init(cfg *config.InfraConfig) error {
	return Start(cfg)
}
