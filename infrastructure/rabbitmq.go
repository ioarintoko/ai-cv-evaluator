package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Job message untuk dikirim ke queue
type EvaluationJob struct {
	EvaluationID uint   `json:"evaluation_id"`
	UploadID     uint   `json:"upload_id"`
	JobID        uint   `json:"job_id"`
	CVText       string `json:"cv_text"`
	ProjectText  string `json:"project_text"`
}

// Struct untuk RabbitMQ client
type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   amqp.Queue
}

// Inisialisasi koneksi RabbitMQ
func NewRabbitMQ() *RabbitMQ {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/" // default
	}

	conn, err := amqp.Dial(url)
	if err != nil {
		log.Fatalf("failed to connect to RabbitMQ: %v", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to open channel: %v", err)
	}

	q, err := ch.QueueDeclare(
		"evaluation_queue", // queue name
		true,               // durable
		false,              // delete when unused
		false,              // exclusive
		false,              // no-wait
		nil,                // args
	)
	if err != nil {
		log.Fatalf("failed to declare queue: %v", err)
	}

	fmt.Println("✅ Connected to RabbitMQ and declared queue")

	return &RabbitMQ{conn: conn, channel: ch, queue: q}
}

// Publish job ke queue
func (r *RabbitMQ) PublishJob(job EvaluationJob) error {
	body, err := json.Marshal(job)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return r.channel.PublishWithContext(
		ctx,
		"",           // exchange
		r.queue.Name, // routing key
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)
}

// Consume job dari queue (untuk worker)
func (r *RabbitMQ) ConsumeJobs(handler func(EvaluationJob)) {
	msgs, err := r.channel.Consume(
		r.queue.Name,
		"",
		true,  // auto-ack
		false, // exclusive
		false, // no-local
		false, // no-wait
		nil,
	)
	if err != nil {
		log.Fatalf("failed to register consumer: %v", err)
	}

	go func() {
		for d := range msgs {
			var job EvaluationJob
			if err := json.Unmarshal(d.Body, &job); err != nil {
				log.Printf("invalid job format: %v", err)
				continue
			}
			handler(job)
		}
	}()
}
