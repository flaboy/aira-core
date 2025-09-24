package cluster

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/flaboy/aira-core/pkg/redis"
)

var master *Election

const clusterKey = "cluster_master"

func init() {
	// machine, _ := os.Hostname()
	// pid := os.Getpid()
	master = &Election{
		// runUid:    machine + "_" + string(pid), // 进程唯一id
		lockTime:  30,
		initFuncs: []func(){},
	}
}

// 使用redis的能力，实现选主
func Start() error {
	go master.Run()
	return nil
}

func Master() *Election {
	return master
}

type Election struct {
	runUid    string // 进程唯一id
	isRunNode bool
	lockTime  int
	initFuncs []func()
}

func (k *Election) Run() {
	// 标识当前节点抢到执行权利
	for !k.check() {
		time.Sleep(time.Duration(k.lockTime) * time.Second)
	}

	// 执行初始化函数
	for _, f := range k.initFuncs {
		f()
	}
}

func (k *Election) AddInitFunc(f func()) {
	k.initFuncs = append(k.initFuncs, f)
}

func (k *Election) check() bool {
	if k.isRunNode {
		return true
	}

	// 沉默节点, 尝试检查runNode是否死机, 抢夺执行权利
	ctx := context.Background()
	ok := redis.RedisClient.SetNX(ctx, clusterKey, k.runUid, time.Duration(k.lockTime+10)*time.Second)

	if ok.Val() {
		k.isRunNode = true
		// 设置一个保持心跳的循环
		go func() {
			ex := time.Duration(k.lockTime) * time.Second
			for range time.Tick(ex) {
				redis.RedisClient.Expire(context.Background(), clusterKey, time.Duration(k.lockTime+10)*time.Second)
			}
		}()
		machine, _ := os.Hostname()
		slog.Info("Cluster master elected", "machine", machine, "runUid", k.runUid)
		return true
	} else {
		k.isRunNode = false
		return false
	}
}
