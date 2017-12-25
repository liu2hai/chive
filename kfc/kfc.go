/*
  kfc --- kafka connector

  sarama暂时不支持设定consumer分组名称，经测试每次sarama.NewConsumer出来的
  consumer都是在不同的分组里，刚好满足需求。如果需要设定consumer的分组名称，
  需要引入sarama-cluster
*/
package kfc

import (
	"time"

	"github.com/liu2hai/chive/logs"

	"github.com/Shopify/sarama"
)

/*
  为了效率，producer和consumer使用不同的sarama.client
*/
type kfclient struct {
	brokers []string

	producer struct {
		client sarama.Client
		msgq   chan *sarama.ProducerMessage
		exit   chan int
		alive  bool
	}

	consumer struct {
		client sarama.Client
		msgq   chan *sarama.ConsumerMessage
		topics []string
		exit   chan int
		alive  bool
	}
}

var kfc *kfclient = &kfclient{}

const MAX_QUEEN_LEN = 10

////////////////////////////////////////////////////////////////////

func InitClient(brokers []string) {
	kfc.brokers = brokers
	kfc.producer.msgq = make(chan *sarama.ProducerMessage, MAX_QUEEN_LEN)
	kfc.producer.exit = make(chan int)
	kfc.producer.alive = false

	kfc.consumer.msgq = make(chan *sarama.ConsumerMessage, MAX_QUEEN_LEN)
	kfc.consumer.exit = make(chan int)
	kfc.producer.alive = false
}

func TobeProducer() error {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll
	config.Producer.Partitioner = sarama.NewRoundRobinPartitioner
	config.Producer.Return.Successes = true

	c, err := sarama.NewClient(kfc.brokers, config)
	if err != nil {
		logs.Error("kfc producer初始化client失败，err[%s]", err.Error())
		return err
	}
	kfc.producer.client = c
	kfc.producer.alive = true

	// syncProducer是一个interface，也就是返回的实例是new出来的
	sp, err := sarama.NewSyncProducerFromClient(c)
	if err != nil {
		logs.Error("kfc初始化Producer失败，err[%s]", err.Error())
		return err
	}

	go syncProducerLoop(sp)
	return nil
}

func TobeConsumer(topics []string) error {
	config := sarama.NewConfig()

	c, err := sarama.NewClient(kfc.brokers, config)
	if err != nil {
		logs.Error("kfc consumer初始化client失败，err[%s]", err.Error())
		return err
	}
	kfc.consumer.client = c
	kfc.consumer.alive = true
	kfc.consumer.topics = topics

	consumer, err := sarama.NewConsumerFromClient(c)
	if err != nil {
		logs.Error("kfc初始化Consumer失败，err[%s]", err.Error())
		return err
	}
	pcs, err := collectPartitionConsumer(consumer, topics)
	if err != nil || len(pcs) <= 0 {
		logs.Error("kfc获得分区消费者失败，err[%s]", err.Error())
		return err
	}

	go consumerLoop(consumer, pcs)
	return nil
}

func SendMessage(topic string, key string, value []byte) {
	if !kfc.producer.alive {
		return
	}

	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(value),
	}
	kfc.producer.msgq <- msg
}

func ReadMessages() <-chan *sarama.ConsumerMessage {
	return kfc.consumer.msgq
}

func ExitProducer() {
	if !kfc.producer.alive {
		return
	}
	kfc.producer.exit <- 1
	<-time.After(time.Second)

	// 暂停一会后再关闭client
	kfc.producer.client.Close()
	kfc.producer.alive = false
}

func ExitConsumer() {
	if !kfc.consumer.alive {
		return
	}
	kfc.consumer.exit <- 1
	<-time.After(time.Second)

	// 暂停一会后再关闭client
	kfc.consumer.client.Close()
	kfc.consumer.alive = false
}

////////////////////////////////////////////////////////////////////

func collectPartitionConsumer(consumer sarama.Consumer, topics []string) ([]sarama.PartitionConsumer, error) {
	pcs := []sarama.PartitionConsumer{}
	for _, t := range topics {
		partitions, err := consumer.Partitions(t)
		if err != nil {
			return pcs, err
		}
		for _, p := range partitions {
			pc, err := consumer.ConsumePartition(t, p, sarama.OffsetNewest)
			if err != nil {
				return pcs, err
			}
			pcs = append(pcs, pc)
		}
	}
	return pcs, nil
}

func syncProducerLoop(sp sarama.SyncProducer) {
	defer sp.Close()

	for {
		select {
		case msg, _ := <-kfc.producer.msgq:
			p, offset, err := sp.SendMessage(msg)
			if err != nil {
				logs.Error("sarama send fail, partition[%d], offset[%d], err[%s]", p, offset, err.Error())
			}

		case <-kfc.producer.exit:
			logs.Info("kfc producer loop exit.")
			return
		}
	}
}

func consumerLoop(consumer sarama.Consumer, pcs []sarama.PartitionConsumer) {
	defer consumer.Close()

	ch := make(chan struct{})
	for _, p := range pcs {
		go consumerOnePartition(consumer, p, ch)
	}

	<-kfc.consumer.exit
	close(ch)
	logs.Info("kfc consumer loop exit.")
}

func consumerOnePartition(consumer sarama.Consumer, pc sarama.PartitionConsumer, ch chan struct{}) {
	defer pc.Close()

	for {
		select {
		case msg := <-pc.Messages():
			kfc.consumer.msgq <- msg

		case <-ch:
			logs.Info("kfc partition loop exit.")
			return
		}
	}
}
