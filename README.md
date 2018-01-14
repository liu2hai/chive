# chive
![logo](https://github.com/liu2hai/chive/raw/master/img/logo.jpg)
chive(韭菜)是一个加密数字货币期货的量化交易系统，现在的数字币交易所都是7*24小时交易，玩家不可能一直盯盘操作，肉身炒币真的很累，而且往往无法坚持止盈止损的交易纪律，在暴涨暴跌中死无全尸。

据此本人特意开发了此自动交易系统, 提供了多种策略参数实现自动交易，晚上也可以安心睡眠了，因为钱输光了。系统还提供一定的行情K线指标分析，用户可以据此开发自己的策略，从此飞向星辰大海！

目前只支持okex的期货交易，后续会加入其它交易所的期货和现货交易，投资有风险，请小心谨慎，毕竟这个系统叫chive是有原因的。


## Quick Start

#### 下载

    go get github.com/liu2hai/chive

#### 搭建环境
本系统依赖kafka和influxDB，安装好后，kafka需要实现声明topic，使用下面的语句

    ./bin/kafka-topics.sh  --create  --zookeeper  localhost:2181  --replication-factor 1  --partitions  1  --topic okex_quote_pub
    ./bin/kafka-topics.sh  --create  --zookeeper  localhost:2181  --replication-factor 1  --partitions  1  --topic okex_archer_req
    ./bin/kafka-topics.sh  --create  --zookeeper  localhost:2181  --replication-factor 1  --partitions  1  --topic okex_archer_rsp

influxDB安装下载见[官网](https://www.influxdata.com/)

#### 使用
进入build目录，使用build.sh生成可执行文件，如果生成失败，请自行安装所需要的第三方库

	go get github.com/Shopify/sarama
	go get github.com/golang/protobuf/proto
	go get github.com/bitly/go-simplejson
	go get github.com/syndtr/goleveldb/leveldb
	go get github.com/gorilla/websocket

将前面安装好的kafka和influxDB的地址和端口参数，写到build/lapf.cnf里，在okex开户后，往合约账户充值，然后申请api交易权限，将分配的api key和secret key写到build/lapf.cnf里，如下

    "archer" : {
        "okex": {
            "apikey": "xxxxx",
            "secretkey": "xxxxxxx"
        }
    }

执行build/run.sh

## 代码说明

    archer  下单程序
    spider  订阅收集行情程序
    krang   运行策略和计算行情指标程序
    stg     行情存储，将交易所一天的行情全部存到一个leveldb数据库，这些数据用于回放
    replay  回放程序，用于调试策略
    strategy 策略模块，新加策略放到该模块下

#### 新加策略
本系统实现了一个简单的均线策略，在strategy/mavg下，新加策略可参照此策略实现, 策略接口如下

```go
type Strategy interface {
	/*
	  策略初始化函数
	*/
	Init(ctx Context)

	/*
	  检查反馈函数,该函数会在OnTick前被调用
	  如果返回false，则OnTick不会被调用
	*/
	CheckFeedBack(ctx Context) bool

	/*
	  行情更新函数
	*/
	OnTick(ctx Context, tick *Tick)
}
```
