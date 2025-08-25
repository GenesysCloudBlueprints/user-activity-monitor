package main

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
	"user-activity-monitor/src/db"
	"user-activity-monitor/src/genesys"

	"github.com/aws/aws-lambda-go/lambda"
)

func main() {
	lambda.Start(handleRequestLogger)
}

func handleRequestLogger() error {
	err := handleRequest()
	if err != nil {
		log.Printf("Error handling request: %v", err)
	}
	return err
}

func handleRequest() error {
	now := time.Now().UnixMilli()
	fmt.Printf("Reaping entries before %d\n", now)

	uaList, err := db.ListUserActivity(true, &now)
	if err != nil {
		return fmt.Errorf("failed to list UserActivity: %v", err)
	}

	// debug uaList
	uaListBytes, err := json.Marshal(uaList)
	if err == nil {
		fmt.Printf("User activity list: %s\n", string(uaListBytes))
	}

	// Reauth if there are any pending user activities (the token can expire if the lambda function is kept warm for too long)
	if len(uaList) > 0 {
		err = genesys.Reauth()
		if err != nil {
			return fmt.Errorf("failed to reauth Genesys: %v", err)
		}
	}

	// Logout all pending user activities
	fmt.Printf("Logging out %d users\n", len(uaList))
	for _, ua := range uaList {
		err = genesys.LogoutUser(ua.UserID)
		if err != nil {
			fmt.Printf("failed to logout Genesys user: %v\n", err)
		} else {
			fmt.Printf("logged out Genesys user: %s\n", ua.UserID)
		}
		err = db.WriteUserActivity(ua, true)
		if err != nil {
			fmt.Printf("failed to write user activity after logout: %v\n", err)
		}
	}

	return nil
}
