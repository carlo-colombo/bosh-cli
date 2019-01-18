package http

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/httpclient"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type AgentRequestMessage struct {
	Method    string        `json:"method"`
	Arguments []interface{} `json:"arguments"`
	ReplyTo   string        `json:"reply_to"`
}

type agentRequest struct {
	directorID          string
	endpoint            string
	alternativeEndpoint string
	httpClient          *httpclient.HTTPClient
	logger              boshlog.Logger
}

func (r agentRequest) Send(method string, arguments []interface{}, response Response) error {
	postBody := AgentRequestMessage{
		Method:    method,
		Arguments: arguments,
		ReplyTo:   r.directorID,
	}

	agentRequestJSON, err := json.Marshal(postBody)
	if err != nil {
		return bosherr.WrapError(err, "Marshaling agent request")
	}

	httpResponse, err := r.performPost(r.endpoint, agentRequestJSON)

	if err != nil {
		return bosherr.WrapErrorf(err, "Performing request to agent")
	}
	defer func() {
		_ = httpResponse.Body.Close()
	}()

	if httpResponse.StatusCode == http.StatusUnauthorized && r.alternativeEndpoint != "" {
		r.logger.Info("agentRequest", "Agent responded with non-successful status code %d on agent endpoint %s, trying alternative endpoint %s", httpResponse.StatusCode, r.endpoint, r.alternativeEndpoint)
		httpResponse, err = r.performPost(r.alternativeEndpoint, agentRequestJSON)
	}
	if httpResponse.StatusCode != http.StatusOK {
		return bosherr.Errorf("Agent responded with non-successful status code: %d", httpResponse.StatusCode)
	}

	responseBody, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		return bosherr.WrapError(err, "Reading agent response")
	}

	err = response.Unmarshal(responseBody)
	if err != nil {
		return bosherr.WrapError(err, "Unmarshaling agent response")
	}

	err = response.ServerError()
	if err != nil {
		return err
	}

	return nil
}

func (r *agentRequest) performPost(endpoint string, agentRequestJSON []byte) (*http.Response, error) {
	return r.httpClient.PostCustomized(endpoint, agentRequestJSON, func(r *http.Request) {
		r.Header["Content-type"] = []string{"application/json"}
	})
}
