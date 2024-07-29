package etcd

import (
	"context"
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"noy/mytest/core/config"
)

// Client 为 etcd 客户端, 封装了一些常用的方法
type Client struct {
	cli      *clientv3.Client
	election *concurrency.Election
	lease    clientv3.Lease
	sync.RWMutex
}

// NewClient 创建 etcd 客户端
func NewClient(cfg *config.ETCDConfig) (*clientv3.Client, error) {
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
		Username:    cfg.Username,
		Password:    cfg.Password,
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (c *Client) GetClient() *clientv3.Client {
	return c.cli
}

// Campaign 竞选 Leader
func (c *Client) Campaign(ctx context.Context, prefix, val string) error {
	s, err := concurrency.NewSession(c.cli)
	if err != nil {
		return err
	}

	e := concurrency.NewElection(s, prefix)
	return e.Campaign(ctx, val)
}

// Leader 获取主节点
func (c *Client) Leader(ctx context.Context, prefix string) (string, error) {
	s, err := concurrency.NewSession(c.cli)
	if err != nil {
		return "", err
	}

	e := concurrency.NewElection(s, prefix)
	resp, err := e.Leader(ctx)
	if err != nil {
		return "", err
	}

	return string(resp.Kvs[0].Value), nil
}

// Resign 释放主节点
func (c *Client) Resign(ctx context.Context, prefix, val string) error {
	if c.election == nil {
		return nil
	}

	return c.election.Resign(ctx)
}

// KeepAlive 主节点保活
func (c *Client) KeepAlive(ctx context.Context, key, val string, ttl int64) (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	resp, err := c.lease.Grant(ctx, ttl)
	if err != nil {
		return nil, err
	}

	_, err = c.cli.Put(ctx, key, val, clientv3.WithLease(resp.ID))
	if err != nil {
		return nil, err
	}

	return c.lease.KeepAlive(ctx, resp.ID)
}

// Watch 监听 key 变化
func (c *Client) Watch(ctx context.Context, key string) clientv3.WatchChan {
	return c.cli.Watch(ctx, key)
}
