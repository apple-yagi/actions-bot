package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"

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
	Ref string `json:"ref"`
}

type DispatchRequestBody struct {
	EventType     string                 `json:"event_type"`
	ClientPayload *DispatchClientPayload `json:"client_payload"`
}

func handleRequest(request Request) (*Response, error) {
	slackBotToken, githubAccessToken, err := getSecretValue(os.Getenv("SECRET_ID"))
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
		log.Println(ev.Text)

		// デバッグ用に🐱のスタンプをメッセージに付与
		if err := api.AddReaction(ReactionAnimal, slack.NewRefToMessage(ev.Channel, ev.TimeStamp)); err != nil {
			log.Println(err.Error())
			return newFailedResponse(), nil
		}

		// Dispatch Github Actions
		url := os.Getenv("GITHUB_ACTIONS_URL")
		body := &DispatchRequestBody{
			EventType: "deploy",
			ClientPayload: &DispatchClientPayload{
				Ref: "main",
			},
		}
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
		respbody, _ := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()

		log.Println(string(respbody))

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

func newFailedResponse() *Response {
	return &Response{
		StatusCode: 500,
		Body:       `{"message": "internal server error"}`,
	}
}

func main() {
	lambda.Start(handleRequest)
}
