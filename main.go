package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

const Region = "ap-northeast-1"
const ReactionAnimal = "cat"

type Request events.APIGatewayProxyRequest
type Response events.APIGatewayProxyResponse

type DispatchClientPayload struct {
	Ref string `label:"デプロイするブランチ名" json:"ref"`
}

type DispatchRequestBody struct {
	EventType     string                 `label:"repository_dispatchのtype" json:"event_type"`
	ClientPayload *DispatchClientPayload `json:"client_payload"`
}

func handleRequest(request Request) (*Response, error) {
	slackBotToken, githubAccessToken, err := getSecretValue(os.Getenv("SECRET_ID"))
	if err != nil {
		log.Println(err.Error())
		return newFailedResponse(), nil
	}

	slackClient := slack.New(slackBotToken)

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
		log.Println(ev.Text)

		// Dispatch Github Actions
		url := os.Getenv("GITHUB_ACTIONS_URL")

		// ev.Text format:@actions_bot <event_type> <branch_name>
		body := convertEventTextToDispatchRequestBody(ev.Text)
		jsonString, err := json.Marshal(body)
		if err != nil {
			log.Println(err.Error())
			return newFailedResponse(), nil
		}

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonString))
		if err != nil {
			log.Println(err.Error())
			return newFailedResponse(), nil
		}

		req.Header.Set("Authorization", "token "+githubAccessToken)
		req.Header.Set("Accept", "application/vnd.github.everest-preview+json")

		client := new(http.Client)
		resp, err := client.Do(req)
		if err != nil {
			log.Println(err.Error())
			return newFailedResponse(), nil
		}
		defer resp.Body.Close()

		// Github Actionsのスタートを通知する
		if _, _, err := slackClient.PostMessage(ev.Channel, slack.MsgOptionText("<@"+ev.User+"> デプロイを開始します", false)); err != nil {
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

func getSecretValue(secretId string) (string, string, error) {
	sess, err := session.NewSession()
	if err != nil {
		return "", "", err
	}

	svc := secretsmanager.New(sess, aws.NewConfig().WithRegion(Region))
	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretId),
		VersionStage: aws.String("AWSCURRENT"),
	}
	result, err := svc.GetSecretValue(input)
	if err != nil {
		return "", "", err
	}

	secretString := aws.StringValue(result.SecretString)
	res := make(map[string]interface{})
	if err := json.Unmarshal([]byte(secretString), &res); err != nil {
		return "", "", err
	}

	return res["slack_bot_token"].(string), res["github_access_token"].(string), nil
}

func convertEventTextToDispatchRequestBody(eventText string) *DispatchRequestBody {
	splitText := strings.Split(eventText, " ")
	eventType := splitText[1]
	ref := splitText[2]
	return &DispatchRequestBody{
		EventType: eventType,
		ClientPayload: &DispatchClientPayload{
			Ref: ref,
		},
	}
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
