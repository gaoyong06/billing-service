package server

import (
	"context"
	"encoding/json"
	"strings"

	"billing-service/internal/biz"
	"billing-service/internal/conf"

	"github.com/apache/rocketmq-client-go/v2"
	"github.com/apache/rocketmq-client-go/v2/consumer"
	"github.com/apache/rocketmq-client-go/v2/primitive"
	"github.com/go-kratos/kratos/v2/log"
)

// MQConsumerServer consumes deduction events from RocketMQ
type MQConsumerServer struct {
	c       rocketmq.PushConsumer
	repo    biz.BillingRepo
	conf    *conf.Data
	log     *log.Helper
	enabled bool
}

// NewMQConsumerServer creates a RocketMQ consumer server
func NewMQConsumerServer(c *conf.Data, repo biz.BillingRepo, logger log.Logger) *MQConsumerServer {
	if c.Rocketmq == nil || !c.Rocketmq.Enabled {
		return &MQConsumerServer{enabled: false}
	}

	r, err := rocketmq.NewPushConsumer(
		consumer.WithNsResolver(primitive.NewPassthroughResolver(c.Rocketmq.NameServers)),
		consumer.WithGroupName(c.Rocketmq.GroupName),
		consumer.WithRetry(int(c.Rocketmq.RetryTimes)),
		consumer.WithConsumeMessageBatchMaxSize(100), // Process up to 100 messages at once
	)
	if err != nil {
		log.NewHelper(logger).Errorf("init consumer error: %v", err)
		return &MQConsumerServer{enabled: false} // Fallback to disabled if init fails? Or panic?
	}

	s := &MQConsumerServer{
		c:       r,
		repo:    repo,
		conf:    c,
		log:     log.NewHelper(logger),
		enabled: true,
	}
	return s
}

// Start starts the consumer
func (s *MQConsumerServer) Start(ctx context.Context) error {
	if !s.enabled {
		s.log.Infof("MQConsumerServer is disabled, skipping startup")
		return nil
	}

	if s.c == nil {
		s.log.Warnf("MQConsumerServer consumer is nil, skipping startup")
		return nil
	}

	s.log.Infof("Starting MQConsumerServer, topic: %s", s.conf.Rocketmq.Topic)

	// Subscribe
	err := s.c.Subscribe(s.conf.Rocketmq.Topic, consumer.MessageSelector{}, s.handler)
	if err != nil {
		errMsg := strings.ToLower(err.Error())
		// Topic 不存在或路由信息未找到是预期情况（特别是在开发环境中）
		// broker 配置了 autoCreateTopicEnable = true，topic 会在第一次发送消息时自动创建
		if strings.Contains(errMsg, "route info not found") || strings.Contains(errMsg, "topic not exist") {
			s.log.Warnf("Topic '%s' does not exist yet (this is expected). It will be auto-created when first message is sent. Error: %v", s.conf.Rocketmq.Topic, err)
		} else {
			s.log.Errorf("Failed to subscribe to topic %s: %v", s.conf.Rocketmq.Topic, err)
		}
		// 不返回错误，避免导致整个应用启动失败
		// 在开发环境中，RocketMQ 可能不可用
		return nil
	}

	err = s.c.Start()
	if err != nil {
		errMsg := strings.ToLower(err.Error())
		// RETRY topic 不存在是预期情况，RocketMQ 会在需要重试时自动创建
		// 格式: %RETRY%ConsumerGroupName
		if strings.Contains(errMsg, "retry") && (strings.Contains(errMsg, "topic") || strings.Contains(errMsg, "not exist")) {
			s.log.Debugf("RETRY topic does not exist yet (this is expected). It will be auto-created when messages need retry. Error: %v", err)
		} else {
			s.log.Errorf("Failed to start RocketMQ consumer: %v", err)
		}
		// 不返回错误，避免导致整个应用启动失败
		return nil
	}

	return nil
}

// Stop stops the consumer
func (s *MQConsumerServer) Stop(ctx context.Context) error {
	if !s.enabled || s.c == nil {
		return nil
	}
	s.log.Info("Stopping MQConsumerServer")
	return s.c.Shutdown()
}

func (s *MQConsumerServer) handler(ctx context.Context, msgs ...*primitive.MessageExt) (consumer.ConsumeResult, error) {
	if len(msgs) == 0 {
		return consumer.ConsumeSuccess, nil
	}

	var events []*biz.DeductEvent
	for _, msg := range msgs {
		var event biz.DeductEvent
		if err := json.Unmarshal(msg.Body, &event); err != nil {
			s.log.Errorf("Unmarshal message failed: %v, body: %s", err, string(msg.Body))
			continue
		}
		events = append(events, &event)
	}

	if len(events) > 0 {
		// Call BatchDeductQuota
		// Use a context with timeout for DB operations?
		// We can use the passed ctx? No, primitive.MessageExt doesn't give context for the whole batch?
		// Use background context with timeout
		// Note: handler ctx is usually valid.

		err := s.repo.BatchDeductQuota(ctx, events)
		if err != nil {
			s.log.Errorf("BatchDeductQuota failed: %v", err)
			return consumer.ConsumeRetryLater, nil
		}
	}
	return consumer.ConsumeSuccess, nil
}
