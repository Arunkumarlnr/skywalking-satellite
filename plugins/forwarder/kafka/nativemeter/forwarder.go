// Licensed to Apache Software Foundation (ASF) under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Apache Software Foundation (ASF) licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package nativemeter

import (
	"fmt"
	"reflect"
	"time"

	"github.com/Shopify/sarama"
	"google.golang.org/protobuf/proto"
	"k8s.io/apimachinery/pkg/util/cache"

	"github.com/apache/skywalking-satellite/internal/pkg/config"
	"github.com/apache/skywalking-satellite/internal/satellite/event"

	v3 "skywalking.apache.org/repo/goapi/collect/language/agent/v3"
	v1 "skywalking.apache.org/repo/goapi/satellite/data/v1"
)

const (
	Name     = "nativemeter-kafka-forwarder"
	ShowName = "Native Meter Kafka Forwarder"
)

type Forwarder struct {
	config.CommonFields
	RoutingRuleLRUCacheSize int `mapstructure:"routing_rule_lru_cache_size"`
	// The TTL of the LRU cache size for hosting routine rules of service instance.
	RoutingRuleLRUCacheTTL int    `mapstructure:"routing_rule_lru_cache_ttl"`
	Topic                  string `mapstructure:"topic"` // The forwarder topic.
	producer               sarama.SyncProducer
	meterClient            v3.MeterReportServiceClient
	upstreamCache          *cache.LRUExpireCache
	upstreamCacheExpire    time.Duration
}

func (f *Forwarder) Name() string {
	return Name
}

func (f *Forwarder) ShowName() string {
	return ShowName
}

func (f *Forwarder) Description() string {
	return "This is a synchronization Kafka forwarder with the SkyWalking native meter protocol."
}

func (f *Forwarder) DefaultConfig() string {
	return `
# The remote topic. 
topic: "skywalking-meters"
`
}

func (f *Forwarder) Prepare(connection interface{}) error {
	client, ok := connection.(sarama.Client)
	if !ok {
		return fmt.Errorf("the %s is only accepet the kafka client, but receive a %s",
			f.Name(), reflect.TypeOf(connection).String())
	}
	producer, err := sarama.NewSyncProducerFromClient(client)
	f.upstreamCache = cache.NewLRUExpireCache(f.RoutingRuleLRUCacheSize)
	f.upstreamCacheExpire = time.Second * time.Duration(f.RoutingRuleLRUCacheTTL)

	if err != nil {
		return err
	}
	f.producer = producer
	return nil
}

func (f *Forwarder) Forward(batch event.BatchEvents) error {
	var message []*sarama.ProducerMessage
	for _, e := range batch {
		data := e.GetData().(*v1.SniffData_Meter)
		rawdata, ok := proto.Marshal(data.Meter)
		if ok != nil {
			return ok
		}
		message = append(message, &sarama.ProducerMessage{
			Topic: f.Topic,
			Value: sarama.ByteEncoder(rawdata),
		})
	}
	return f.producer.SendMessages(message)
}

func (f *Forwarder) ForwardType() v1.SniffType {
	return v1.SniffType_MeterType
}

func (f *Forwarder) SyncForward(_ *v1.SniffData) (*v1.SniffData, error) {
	return nil, fmt.Errorf("unsupport sync forward")
}

func (f *Forwarder) SupportedSyncInvoke() bool {
	return false
}