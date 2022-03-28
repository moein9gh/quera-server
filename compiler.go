package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/streadway/amqp"
	_ "github.com/streadway/amqp"
	"github.com/tidwall/gjson"
)

var incomingPayload map[string]interface{}

type QuestionPayload struct {
	questionNumber string `json:"questionNumber"`
	byteArray      []byte `json:"byteArray"`
	fileSize       int64  `json:"size"`
	name           string
}

type Test struct {
	Id     int    `json:"id"`
	Input  string `json:"input"`
	Output string `json:"output"`
}

func failOnError(err error, msg string) {
	if err != nil {
		log.Fatalf("%s: %s", msg, err)
	}
}

func (qp *QuestionPayload) runTests(tests []Test) []bool {

	results := []bool{}

	for _, v := range tests {
		cmd := exec.Command(qp.name + ".exe")

		stdin, err := cmd.StdinPipe()
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			io.WriteString(stdin, v.Input)
			defer stdin.Close()
		}()

		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Fatal(err)
		}
		if fmt.Sprintf("%s", out) != v.Output {
			results = append(results, false)
			log.Println("failed", fmt.Sprintf("{{%s}}", out), v.Output)
		} else {
			results = append(results, true)
			log.Println("success")
		}
	}

	return results

}

func (qp *QuestionPayload) create(questionNumber string, byteArray []byte, fileSize int64) {
	qp.fileSize = fileSize
	qp.byteArray = byteArray
	qp.questionNumber = questionNumber
}

func (qp *QuestionPayload) startTest(questions string) (results []bool) {
	tests := getExamplesOfQuestion(qp.questionNumber, questions)
	results = qp.runTests(tests)
	return results
}

func (qp *QuestionPayload) createFile() {
	path := "./tests/" + strconv.Itoa(int(time.Now().Unix()+qp.fileSize))
	qp.name = path

	err := os.WriteFile(path+".cpp", qp.byteArray, 0644)
	failOnError(err, "failed to create file")

	_, err = exec.Command("g++", path+".cpp", "-o", path+".exe").Output()
	failOnError(err, "failed to build cpp file")

}

func getExamplesOfQuestion(questionNumber, questions string) []Test {
	question := gjson.Get(questions, questionNumber+".examples")

	var examples []Test

	err := json.Unmarshal([]byte(question.String()), &examples)

	failOnError(err, "failed to unmarshal")

	return examples

}

func connectAndListenToRabbit(messageReceived chan string, questions string) {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	err = ch.ExchangeDeclare(
		"messages", // name
		"topic",    // type
		true,       // durable
		false,      // auto-deleted
		false,      // internal
		false,      // no-wait
		nil,        // arguments
	)
	failOnError(err, "Failed to declare an exchange")

	queue, err := ch.QueueDeclare(
		"",    // name
		false, // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	failOnError(err, "Failed to declare a queue")

	//err = ch.ExchangeDeclare(
	//	"messages", // name
	//	"topic",    // type
	//	true,       // durable
	//	false,      // auto-deleted
	//	false,      // internal
	//	false,      // no-wait
	//	nil,        // arguments
	//)
	//failOnError(err, "Failed to declare an exchange")
	//
	//err = ch.Publish(
	//	"messages", // exchange
	//	"#",        // routing key
	//	false,      // mandatory
	//	false,      // immediate
	//	amqp.Publishing{
	//		ContentType: "text/plain",
	//		Body:        []byte("hi"),
	//	})
	//
	//failOnError(err, "Failed to publish a message")

	err = ch.QueueBind(
		queue.Name, // queue name
		"#",        // routing key
		"messages", // exchange
		false,
		nil)

	msgs, err := ch.Consume(
		queue.Name, // queue
		"",         // consumer
		true,       // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	receiveMessage(messageReceived, questions)

	go func() {
		for d := range msgs {
			log.Println("Received a message")
			err := d.Ack(false)
			failOnError(err, "failed to ack")
			messageReceived <- fmt.Sprintf("%s", d.Body)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
}

func receiveMessage(ch chan string, questions string) {
	go func() {
		for data := range ch {

			jsonBody := fmt.Sprintf("%s", data)
			value := gjson.GetMany(jsonBody, "byteArray", "size", "questionNumber")

			var ba = []byte{}
			for _, v := range value[0].Array() {
				ba = append(ba, byte(v.Float()))
			}

			question := QuestionPayload{}
			question.create(
				value[2].String(),
				ba,
				value[1].Int())

			question.createFile()
			_ = question.startTest(questions)
		}
	}()
}

func main() {

	questions := loadQuestions()

	messageReceived := make(chan string)

	connectAndListenToRabbit(messageReceived, questions)

}

func loadQuestions() string {
	byteArray, err := ioutil.ReadFile("./samples.json")

	failOnError(err, "failed to read")

	fileContent := fmt.Sprintf("%s", byteArray)

	return fileContent
}

/*
rabbitmq

conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	failOnError(err, "Failed to connect to RabbitMQ")
	defer conn.Close()

	ch, err := conn.Channel()
	failOnError(err, "Failed to open a channel")
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"submits", // name
		false,     // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	failOnError(err, "Failed to declare a queue")

	msgs, err := ch.Consume(
		q.Name, // queue
		"",     // consumer
		true,   // auto-ack
		false,  // exclusive
		false,  // no-local
		false,  // no-wait
		nil,    // args
	)
	failOnError(err, "Failed to register a consumer")

	forever := make(chan bool)

	go func() {
		for d := range msgs {
			log.Printf("Received a message: %s", d.Body)
		}
	}()

	log.Printf(" [*] Waiting for messages. To exit press CTRL+C")
	<-forever
*/

/*

run code

*/
