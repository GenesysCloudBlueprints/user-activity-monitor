package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"user-activity-monitor/src/db"
	"user-activity-monitor/src/genesys"
	"user-activity-monitor/src/groupconfig"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
)

//go:embed app.html
var appHTML embed.FS

var genesysAPIDomain string = os.Getenv("GENESYS_API_DOMAIN")

type Response struct {
	StatusCode int               `json:"statusCode"`
	Headers    map[string]string `json:"headers"`
	Body       string            `json:"body"`
}

// OrganizationResponse represents the response from the Genesys Cloud API
type OrganizationResponse struct {
	Organization struct {
		ID string `json:"id"`
	} `json:"organization"`
}

type ExtendedUserActivity struct {
	db.UserActivity
	UserName              string `json:"userName"`
	UserImage             string `json:"userImage"`
	SecondaryPresenceName string `json:"secondaryPresenceName"`
	Status                string `json:"status"`
	GroupName             string `json:"groupName"`
}

func main() {
	lambda.Start(handleRequestLogger)
}

func handleRequestLogger(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	fmt.Printf("Processing request data for request %s.\n", request.RequestContext.RequestID)

	response, err := handleRequest(ctx, request)
	if err != nil {
		fmt.Printf("Error handling request: %v", err)
		return Response{
			StatusCode: 500,
			Headers: map[string]string{
				"Content-Type": "text/html",
			},
			Body: "<html><body><h1>Internal Server Error</h1><p>An error occurred while processing your request.</p></body></html>",
		}, nil
	}

	return response, nil
}

func handleRequest(ctx context.Context, request events.APIGatewayProxyRequest) (Response, error) {
	// Use Resource instead of Path to avoid stage prefix issues
	path := request.Resource
	
	switch path {
	case "/report/data":
		{
			// Validate authorization
			if err := validateAuthorization(request); err != nil {
				fmt.Printf("Authorization validation failed: %v", err)
				return Response{
					StatusCode: 401,
				}, nil
			}

			// Get pending user activity records
			pendingRecords, err := db.ListUserActivity(true, nil)
			if err != nil {
				fmt.Printf("Error listing pending user activity: %v", err)
				return Response{
					StatusCode: 500,
				}, nil
			}
			extendedPendingRecords, err := extendUserActivity(pendingRecords, "pending")
			if err != nil {
				fmt.Printf("Error extending user activity: %v", err)
				return Response{
					StatusCode: 500,
				}, nil
			}

			// Get exempt user activity records
			exemptRecords, err := db.ListUserActivity(false, nil)
			if err != nil {
				fmt.Printf("Error listing exempt user activity: %v", err)
				return Response{
					StatusCode: 500,
				}, nil
			}
			extendedExemptRecords, err := extendUserActivity(exemptRecords, "exempt")
			if err != nil {
				fmt.Printf("Error extending user activity: %v", err)
				return Response{
					StatusCode: 500,
				}, nil
			}

			// Marshal the records to JSON
			recordsJson, err := json.Marshal(append(extendedPendingRecords, extendedExemptRecords...))
			if err != nil {
				fmt.Printf("Error marshalling records: %v", err)
				return Response{
					StatusCode: 500,
				}, nil
			}

			// Return the records
			return Response{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type": "application/json",
				},
				Body: string(recordsJson),
			}, nil
		}
	case "/report":
		{
			// Read the embedded HTML file
			htmlBytes, err := appHTML.ReadFile("app.html")
			if err != nil {
				fmt.Printf("Error reading embedded HTML file: %v", err)
				return Response{
					StatusCode: 500,
					Headers: map[string]string{
						"Content-Type": "text/html",
					},
					Body: "<html><body><h1>Internal Server Error</h1><p>Failed to load report template.</p></body></html>",
				}, nil
			}

			// Inject configuration into HTML
			htmlContent := string(htmlBytes)
			clientID := os.Getenv("CODE_AUTH_CLIENT_ID")
			apiDomain := os.Getenv("GENESYS_API_DOMAIN")
			
			// Replace placeholders
			htmlContent = strings.ReplaceAll(htmlContent, "{{CLIENT_ID}}", clientID)
			htmlContent = strings.ReplaceAll(htmlContent, "{{GENESYS_DOMAIN}}", apiDomain)
			htmlContent = strings.ReplaceAll(htmlContent, "{{API_STAGE}}", request.RequestContext.Stage)

			// Return the embedded HTML content
			return Response{
				StatusCode: 200,
				Headers: map[string]string{
					"Content-Type":  "text/html",
					"Cache-Control": "no-cache, no-store, must-revalidate",
				},
				Body: htmlContent,
			}, nil
		}
	}

	return Response{
		StatusCode: 404,
		Body:       "Not Found",
	}, nil
}

func validateAuthorization(request events.APIGatewayProxyRequest) error {
	// Get token from Authorization header
	authHeader := request.Headers["Authorization"]
	if authHeader == "" {
		authHeader = request.Headers["authorization"] // Try lowercase
	}
	
	if authHeader == "" {
		return fmt.Errorf("missing Authorization header")
	}

	// Extract Bearer token
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return fmt.Errorf("invalid Authorization header format")
	}
	
	token := parts[1]
	if token == "" {
		return fmt.Errorf("empty token in Authorization header")
	}

	// Get the expected organization ID from environment variable
	expectedOrgID := os.Getenv("EXPECTED_ORGANIZATION_ID")
	if expectedOrgID == "" {
		return fmt.Errorf("EXPECTED_ORGANIZATION_ID environment variable not set")
	}

	// Make HTTP request to Genesys Cloud API
	req, err := http.NewRequest("GET", fmt.Sprintf("https://api.%s/api/v2/users/me?expand=organization", genesysAPIDomain), nil)
	if err != nil {
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to make HTTP request: %w", err)
	}
	defer resp.Body.Close()

	// Check if the request was successful
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API request failed with status: %d", resp.StatusCode)
	}

	// Parse the response
	var orgResp OrganizationResponse
	if err := json.NewDecoder(resp.Body).Decode(&orgResp); err != nil {
		return fmt.Errorf("failed to decode API response: %w", err)
	}

	// Validate the organization ID
	if orgResp.Organization.ID != expectedOrgID {
		return fmt.Errorf("organization ID mismatch: expected %s, got %s", expectedOrgID, orgResp.Organization.ID)
	}

	return nil
}

func getTokenFromCookie(cookieHeader string) (string, error) {
	if cookieHeader == "" {
		return "", fmt.Errorf("no cookie header found")
	}

	cookies := strings.Split(cookieHeader, ";")
	for _, cookie := range cookies {
		cookie = strings.TrimSpace(cookie)
		parts := strings.SplitN(cookie, "=", 2)
		if len(parts) == 2 && parts[0] == "genesys_session" {
			return parts[1], nil
		}
	}

	return "", fmt.Errorf("genesys_session cookie not found")
}

func extendUserActivity(userActivity []db.UserActivity, status string) ([]ExtendedUserActivity, error) {
	extendedUserActivities := make([]ExtendedUserActivity, len(userActivity))

	// Collect all the user IDs
	userIds := make(map[string]bool)
	for _, activity := range userActivity {
		userIds[activity.UserID] = true
	}

	// Convert map to slice
	userIdsSlice := make([]string, 0, len(userIds))
	for userId := range userIds {
		userIdsSlice = append(userIdsSlice, userId)
	}

	// Get the users
	users, err := genesys.GetUsers(userIdsSlice)
	if err != nil {
		return nil, fmt.Errorf("failed to get users: %w", err)
	}

	// Get the presences
	presences, err := genesys.GetPresences()
	if err != nil {
		return nil, fmt.Errorf("failed to get presences: %w", err)
	}

	// Extend the user activity
	for i, activity := range userActivity {
		secondaryPresenceName := "N/A"
		if presence, exists := presences[activity.SecondaryPresenceID]; exists {
			if labels, ok := presence.LanguageLabels["en_US"]; ok {
				secondaryPresenceName = labels
			}
		}

		groupName := "N/A"
		if group, exists := groupconfig.TimeoutGroups[activity.GroupID]; exists {
			groupName = fmt.Sprintf("%s (%v minutes)", group.Name, group.TimeoutMinutes)
		}

		userName := "N/A"
		userImage := "N/A"
		if user, exists := users[activity.UserID]; exists {
			userName = user.Name
			userImage = user.GetImageThumbnail()
		}

		extendedUserActivities[i] = ExtendedUserActivity{
			UserActivity:          activity,
			UserName:              userName,
			UserImage:             userImage,
			SecondaryPresenceName: secondaryPresenceName,
			Status:                status,
			GroupName:             groupName,
		}
	}
	return extendedUserActivities, nil
}
