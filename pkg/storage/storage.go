package storage

import (
	"github.com/flaboy/aira/aira-core/pkg/config"
)

var storages = make(map[string]Storage)

// Get 获取指定名称的存储实现
func Get(name string) Storage {
	return storages[name]
}

func Init() (err error) {
	err = initStorage("public", true, config.Config.PublicStorage)
	if err != nil {
		return err
	}
	return initStorage("private", false, config.Config.PrivateStorage)
}

func initStorage(key string, public bool, cfg config.StorageInstanceConfig) error {
	var storage Storage
	var err error

	if cfg.Type == "s3" {
		storage, err = NewS3Storage(
			cfg.S3.AccessKey,
			cfg.S3.SecretKey,
			cfg.S3.Bucket,
			cfg.S3.Region,
			cfg.S3.Endpoint,
			cfg.S3.PublicURL,
			public,
		)
		if err != nil {
			return err
		}
	} else {
		storage = NewLocalStorage(
			cfg.Local.BasePath,
			cfg.Local.BaseURL,
		)
	}

	storages[key] = storage
	return nil
}
