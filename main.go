package main

import (
	"encoding/json"
	"log"

	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

type Request events.APIGatewayProxyRequest
type Response events.APIGatewayProxyResponse

const REGION = "ap-northeast-1"
const ReactionAnimal = "cat"

func handleRequest(request Request) (*Response, error) {
	slackBotToken, err := getSlackBotToken()
	if err != nil {
		log.Println(err.Error())
		return newFailedResponse(), nil
	}

	api := slack.New(slackBotToken)

	body := request.Body
	eventsAPIEvent, err := slackevents.ParseEvent(json.RawMessage(body), slackevents.OptionNoVerifyToken())
	if err != nil {
		log.Println(err.Error())
		return newFailedResponse(), nil
	}

	// Verify url
	if eventsAPIEvent.Type == slackevents.URLVerification {
		var r *slackevents.ChallengeResponse
		if err := json.Unmarshal([]byte(body), &r); err != nil {
			log.Println(err.Error())
			return newFailedResponse(), nil
		}
		log.Println("Success url verification")
		return &Response{Body: string([]byte(r.Challenge)), StatusCode: 200}, nil
	}

	switch ev := eventsAPIEvent.InnerEvent.Data.(type) {
	case *slackevents.AppMentionEvent:
		log.Println("AppMentionEvent")
		if err := api.AddReaction(ReactionAnimal, slack.NewRefToMessage(ev.Channel, ev.TimeStamp)); err != nil {
			log.Println(err.Error())
			return newFailedResponse(), nil
		}
		return &Response{StatusCode: 200}, nil
	}

	log.Println("Did not match event")
	return &Response{
		StatusCode: 200,
	}, nil
}

func getSlackBotToken() (string, error) {
	sess, err := session.NewSession()
	if err != nil {
		return "", err
	}

	svc := secretsmanager.New(sess, aws.NewConfig().WithRegion(REGION))
	secretName := os.Getenv("SECRET_NAME")
	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSCURRENT"),
	}
	result, err := svc.GetSecretValue(input)
	if err != nil {
		return "", err
	}

	secretString := aws.StringValue(result.SecretString)
	res := make(map[string]interface{})
	if err := json.Unmarshal([]byte(secretString), &res); err != nil {
		return "", err
	}

	return res["SlackBotToken"].(string), nil
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
