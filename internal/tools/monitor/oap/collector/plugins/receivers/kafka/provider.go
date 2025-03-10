// Copyright (c) 2021 Terminus, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kafka

import (
	"fmt"
	"time"

	"github.com/Shopify/sarama"

	"github.com/erda-project/erda-infra/base/logs"
	"github.com/erda-project/erda-infra/base/servicehub"
	"github.com/erda-project/erda/internal/apps/msp/apm/trace"
	"github.com/erda-project/erda/internal/tools/monitor/core/metric"
	"github.com/erda-project/erda/internal/tools/monitor/oap/collector/core/model"
	"github.com/erda-project/erda/internal/tools/monitor/oap/collector/core/model/odata"
	kafkaInf "github.com/erda-project/erda/internal/tools/monitor/oap/collector/lib/kafka"
	"github.com/erda-project/erda/internal/tools/monitor/oap/collector/lib/protoparser/oapspan"
	"github.com/erda-project/erda/internal/tools/monitor/oap/collector/lib/protoparser/spotmetric"
	"github.com/erda-project/erda/internal/tools/monitor/oap/collector/lib/protoparser/spotspan"
)

type parserName string

const (
	oapSpan      parserName = "oapspan"
	oapSpanEvent parserName = "oapspanevent"
	spotSpan     parserName = "spotspan"
	spotMetric   parserName = "spotmetric"
)

type config struct {
	ProtoParser string                   `file:"proto_parser"`
	Concurrency int                      `file:"concurrency" default:"9"`
	BufferSize  int                      `file:"buffer_size" default:"512"`
	ReadTimeout time.Duration            `file:"read_timeout" default:"10s"`
	Consumer    *kafkaInf.ConsumerConfig `file:"consumer"`
}

var _ model.Receiver = (*provider)(nil)

// +provider
type provider struct {
	Cfg    *config
	parser parserName
	Log    logs.Logger
	Kafka  kafkaInf.Interface `autowired:"kafkago"`
	cg     *kafkaInf.ConsumerGroupManager

	consumer          model.ObservableDataConsumerFunc
	consumerInjectedC chan struct{}
}

func (p *provider) ComponentClose() error {
	return p.cg.Close()
}

func (p *provider) ComponentConfig() interface{} {
	return p.Cfg
}

func (p *provider) RegisterConsumer(consumer model.ObservableDataConsumerFunc) {
	p.Log.Infof("register consumer: %+v", consumer)
	p.consumer = consumer
	close(p.consumerInjectedC)
}

// Run this is optional
func (p *provider) Init(ctx servicehub.Context) error {
	if p.Cfg.ProtoParser == "" {
		return fmt.Errorf("proto_parser required")
	}

	p.parser = parserName(p.Cfg.ProtoParser)

	var invokeFunc kafkaInf.ConsumerFuncV2
	switch p.parser {
	case oapSpan:
		invokeFunc = p.parseOapSpan()
	case spotSpan:
		invokeFunc = p.parseSpotSpan()
	case spotMetric:
		invokeFunc = p.parseSpotMetric()
	case oapSpanEvent:
		invokeFunc = p.parseOapSpanEvent()
	default:
		return fmt.Errorf("invalide parser: %q", p.parser)
	}

	cg, err := p.Kafka.NewConsumerGroup(p.Cfg.Consumer, invokeFunc)
	if err != nil {
		return fmt.Errorf("failed create consumer: %w", err)
	}
	p.cg = cg

	p.consumerInjectedC = make(chan struct{})
	return nil
}

func (p *provider) parseOapSpanEvent() kafkaInf.ConsumerFuncV2 {
	return func(msg *sarama.ConsumerMessage) error {
		return oapspan.ParseOapSpanEvent(msg.Value, func(m []*metric.Metric) error {
			if len(m) > 0 {
				for i := 0; i < len(m); i++ {
					p.consumeData(m[i])
				}
			}
			return nil
		})
	}
}

func (p *provider) parseOapSpan() kafkaInf.ConsumerFuncV2 {
	return func(msg *sarama.ConsumerMessage) error {
		return oapspan.ParseOapSpan(msg.Value, func(span *trace.Span) error {
			return p.consumeData(span)
		})
	}
}

func (p *provider) parseSpotSpan() kafkaInf.ConsumerFuncV2 {
	return func(msg *sarama.ConsumerMessage) error {
		return spotspan.ParseSpotSpan(msg.Value, func(span *trace.Span) error {
			return p.consumeData(span)
		})
	}
}

func (p *provider) parseSpotMetric() kafkaInf.ConsumerFuncV2 {
	return func(msg *sarama.ConsumerMessage) error {
		return spotmetric.ParseSpotMetric(msg.Value, func(m *metric.Metric) error {
			return p.consumeData(m)
		})
	}
}

func (p *provider) consumeData(od odata.ObservableData) error {
	if p.consumer == nil { // wait consumer injected
		<-p.consumerInjectedC
	}
	return p.consumer(od)
}

func init() {
	servicehub.Register("erda.oap.collector.receiver.kafka", &servicehub.Spec{
		Services: []string{
			"erda.oap.collector.receiver.kafka",
		},
		Description: "here is description of erda.oap.collector.receiver.kafka",
		ConfigFunc: func() interface{} {
			return &config{}
		},
		Creator: func() servicehub.Provider {
			return &provider{}
		},
	})
}
