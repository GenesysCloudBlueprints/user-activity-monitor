package genesys

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type clientCredentials struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"`
}

var accessToken string
var genesysAPIDomain string = os.Getenv("GENESYS_API_DOMAIN")

const (
	secretName = "user-activity-monitor-client-credentials"
	region     = "us-east-1"
)

func init() {
	Reauth()
}

func Reauth() error {
	config, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		log.Fatal(err)
	}

	// Create Secrets Manager client
	svc := secretsmanager.NewFromConfig(config)

	input := &secretsmanager.GetSecretValueInput{
		SecretId:     aws.String(secretName),
		VersionStage: aws.String("AWSCURRENT"), // VersionStage defaults to AWSCURRENT if unspecified
	}

	result, err := svc.GetSecretValue(context.TODO(), input)
	if err != nil {
		// For a list of exceptions thrown, see
		// https://docs.aws.amazon.com/secretsmanager/latest/apireference/API_GetSecretValue.html
		log.Fatal(err.Error())
	}

	// Decrypts secret using the associated KMS key.
	var secretString string = *result.SecretString
	var clientCredentials clientCredentials
	err = json.Unmarshal([]byte(secretString), &clientCredentials)
	if err != nil {
		log.Fatal(err)
	}

	// Get access token
	accessToken, err = getAccessToken(clientCredentials)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func GetUser(userID string) (*GenesysUser, error) {
	var response GenesysUser

	err := apiGet(fmt.Sprintf("/api/v2/users/%s?expand=groups,presence,conversationSummary", url.QueryEscape(userID)), &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get Genesys user: %w", err)
	}

	return &response, nil
}

func GetUsers(userIDs []string) (map[string]*GenesysUser, error) {
	// Create map of users
	users := make(map[string]*GenesysUser)

	// Batch IDs into groups (API page size limit is 500)
	const batchSize = 500

	for i := 0; i < len(userIDs); i += batchSize {
		end := i + batchSize
		if end > len(userIDs) {
			end = len(userIDs)
		}

		// Get the current batch
		batch := userIDs[i:end]

		// Join batch ids with commas
		ids := strings.Join(batch, ",")

		// Get users for this batch
		var response genesysUserResponse
		err := apiGet(fmt.Sprintf("/api/v2/users?id=%s&pageSize=500", url.QueryEscape(ids)), &response)
		if err != nil {
			return nil, fmt.Errorf("failed to get Genesys users for batch %d-%d: %w", i+1, end, err)
		}

		// Add users from this batch to the result map
		for _, user := range response.Entities {
			users[user.ID] = &user
		}
	}

	return users, nil
}

func GetPresences() (map[string]GenesysPresence, error) {
	var response genesysPresenceResponse
	err := apiGet("/api/v2/presence/definitions", &response)
	if err != nil {
		return nil, fmt.Errorf("failed to get Genesys presences: %w", err)
	}

	// Convert presences to a map of ID
	presenceMap := make(map[string]GenesysPresence)
	for _, presence := range response.Entities {
		presenceMap[presence.ID] = presence
	}

	return presenceMap, nil
}

func LogoutUser(userID string) error {
	fmt.Printf("Logging out Genesys user: %s\n", userID)
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 16 * time.Second,
	}

	// Create request
	url := fmt.Sprintf("https://api.%s/api/v2/tokens/%s", genesysAPIDomain, url.PathEscape(userID))
	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+accessToken)

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

func apiGet(urlPath string, response interface{}) error {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 16 * time.Second,
	}

	// ensure path starts with /
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}

	// Create request
	url := fmt.Sprintf("https://api.%s%s", genesysAPIDomain, urlPath)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse response
	err = json.Unmarshal(bodyBytes, &response)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

	return nil
}

func getAccessToken(clientCredentials clientCredentials) (string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Prepare token request data
	tokenData := map[string]string{
		"grant_type": "client_credentials",
	}

	// Convert to form data
	formData := ""
	for key, value := range tokenData {
		if formData != "" {
			formData += "&"
		}
		formData += key + "=" + value
	}

	// Create token request with form data
	req, err := http.NewRequest("POST", fmt.Sprintf("https://login.%s/oauth/token", genesysAPIDomain), strings.NewReader(formData))
	if err != nil {
		return "", fmt.Errorf("failed to create token request: %w", err)
	}

	// Set headers for basic auth
	credentials := fmt.Sprintf("%s:%s", clientCredentials.ClientID, clientCredentials.ClientSecret)
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(credentials)))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make token request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read token response: %w", err)
	}

	// Parse token response
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int    `json:"expires_in"`
	}

	err = json.Unmarshal(bodyBytes, &tokenResp)
	if err != nil {
		return "", fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("no access token received")
	}

	return tokenResp.AccessToken, nil
}
