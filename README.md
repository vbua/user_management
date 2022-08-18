# Simple user CRUD (Go + Redis + Kafka + Clickhouse)

## Installation
Copy .env.example into .env. You don't have to change it, you can use default settings.
```
docker compose up -d
```

## Usage
- You can use Postman for testing requests (https://blog.postman.com/postman-now-supports-grpc/).
As a Protobuf Definition you can use ./proto/user.proto file.
- The Clickhouse database is populated every time a request is made to add a user. To test it, add a user through Postman and run:
```
docker compose exec clickhouse sh
clickhouse-client
select * from user_adds;
```
- To see that the records go through Kafka, you have to run:
```
docker compose exec kafka sh
$KAFKA_HOME/bin/kafka-console-consumer.sh --topic=user_adds --from-beginning --bootstrap-server `broker-list.sh`
```