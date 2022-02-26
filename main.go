package main

import (
	"encoding/json"
	"log"

	// "os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"

	// "github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type Request events.APIGatewayProxyRequest
type Response events.APIGatewayProxyResponse

// var api = slack.New(os.Getenv("SLACK_BOT_TOKEN"))

func handleRequest(request Request) (*Response, error) {
	body := request.Body
	log.Println(body)
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Println(err)
		return newFailedResponse(), nil
	}

	return &Response{
		StatusCode: 200,
		Body:       eventsAPIEvent.TeamID,
	}, nil
}

func newFailedResponse() *Response {
	return &Response{
		StatusCode: 500,
		Body:       `{"message": "internal server error"}`,
	}
}

func main() {
	lambda.Start(handleRequest)
}
