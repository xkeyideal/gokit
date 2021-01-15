// Copyright 2016 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package concurrency

import (
	"context"
	"fmt"

	pb "github.com/coreos/etcd/etcdserver/etcdserverpb"
	"github.com/coreos/etcd/mvcc/mvccpb"
	v3 "go.etcd.io/etcd/clientv3"
)

func waitDelete(ctx context.Context, client *v3.Client, key string, rev int64) error {
	cctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wr v3.WatchResponse
	wch := client.Watch(cctx, key, v3.WithRev(rev))
	for wr = range wch {
		for _, ev := range wr.Events {
			if ev.Type == mvccpb.DELETE {
				return nil
			}
		}
	}
	if err := wr.Err(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("lost watcher waiting for delete")
}

// waitDeletes efficiently waits until all keys matching the prefix and no greater
// than the create revision.
func waitDeletes(ctx context.Context, client *v3.Client, pfx string, maxCreateRev int64) (*pb.ResponseHeader, error) {
	getOpts := append(v3.WithLastCreate(), v3.WithMaxCreateRev(maxCreateRev))

	// 逐步等待 createRevision <= maxCreateRev 的key被删除，防止之前的操作没有释放此pfx
	// 保证先申请，先得到锁
	for {
		resp, err := client.Get(ctx, pfx, getOpts...)
		if err != nil {
			return nil, err
		}

		// 全部被删除时，正确退出
		if len(resp.Kvs) == 0 {
			return resp.Header, nil
		}

		// 根据ctx控制是阻塞等待或超时等待，最大的createRevision被删除
		lastKey := string(resp.Kvs[0].Key)
		if err = waitDelete(ctx, client, lastKey, resp.Header.Revision); err != nil {
			return nil, err
		}
	}
}
