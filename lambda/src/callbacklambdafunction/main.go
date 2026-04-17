package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	clientID         = os.Getenv("CODE_AUTH_CLIENT_ID")
	genesysAPIDomain = os.Getenv("GENESYS_API_DOMAIN")
)

type Response struct {
	StatusCode        int                 `json:"statusCode"`
	Headers           map[string]string   `json:"headers"`
	MultiValueHeaders map[string][]string `json:"multiValueHeaders,omitempty"`
	Body              string              `json:"body"`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
}

func main() {
	lambda.Start(handleRequest)
}

func handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	fmt.Printf("Processing OAuth callback for request %s\n", request.RequestContext.RequestID)

	// Get authorization code from query parameters
	code := request.QueryStringParameters["code"]
	if code == "" {
		return Response{
			StatusCode: 400,
			Headers: map[string]string{
				"Content-Type": "text/html",
			},
			Body: "<html><body><h1>Error</h1><p>Missing authorization code</p></body></html>",
		}, nil
	}

	// Get code_verifier from cookie
	codeVerifier, err := getCodeVerifierFromCookie(request.Headers["Cookie"])
	if err != nil {
		fmt.Printf("Error getting code verifier: %v\n", err)
		return Response{
			StatusCode: 400,
			Headers: map[string]string{
				"Content-Type": "text/html",
			},
			Body: "<html><body><h1>Error</h1><p>Missing or invalid PKCE verifier</p></body></html>",
		}, nil
	}

	// Build redirect URI
	redirectURI := fmt.Sprintf("https://%s/%s/callback", request.Headers["Host"], request.RequestContext.Stage)

	// Exchange authorization code for access token
	token, err := exchangeCodeForToken(code, codeVerifier, redirectURI)
	if err != nil {
		fmt.Printf("Error exchanging code for token: %v\n", err)
		return Response{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/html",
			},
			Body: fmt.Sprintf("<html><body><h1>Error</h1><p>Failed to exchange authorization code: %v</p></body></html>", err),
		}, nil
	}

	// Redirect to report page with token in URL hash (for security)
	reportURL := fmt.Sprintf("/%s/report#access_token=%s", request.RequestContext.Stage, token.AccessToken)

	return Response{
		StatusCode: 302,
		Headers: map[string]string{
			"Location": reportURL,
		},
	}, nil
}

func exchangeCodeForToken(code, codeVerifier, redirectURI string) (*TokenResponse, error) {
	// Create HTTP client
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Prepare token request data
	data := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {clientID},
		"code_verifier": {codeVerifier},
	}

	// Create token request
	tokenURL := fmt.Sprintf("https://login.%s/oauth/token", genesysAPIDomain)
	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make token request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse token response
	var tokenResp TokenResponse
	err = json.Unmarshal(bodyBytes, &tokenResp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse token response: %w", err)
	}

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("no access token received")
	}

	return &tokenResp, nil
}

func getCodeVerifierFromCookie(cookieHeader string) (string, error) {
	if cookieHeader == "" {
		return "", fmt.Errorf("no cookie header found")
	}

	cookies := strings.Split(cookieHeader, ";")
	for _, cookie := range cookies {
		cookie = strings.TrimSpace(cookie)
		parts := strings.SplitN(cookie, "=", 2)
		if len(parts) == 2 && parts[0] == "pkce_code_verifier" {
			return parts[1], nil
		}
	}

	return "", fmt.Errorf("pkce_code_verifier cookie not found")
}

func createSessionCookie(token string) string {
	// Create an HTTP-only cookie
	// Expires in 1 hour (3600 seconds)
	// Note: Using SameSite=Lax without Secure for API Gateway compatibility
	return fmt.Sprintf("genesys_session=%s; Path=/; Max-Age=3600; HttpOnly; SameSite=Lax", token)
}

func generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}
