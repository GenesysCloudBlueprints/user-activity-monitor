# User Activity Monitor

The user activity monitor service monitors Genesys Cloud user presence and conversations to identify idle users and terminate their sessions.

## Project Overview

This is an AWS serverless project that consists of:

- **Amazon EventBridge**: Listens for Genesys Cloud user activity events
- **AWS Lambda**: Go-based function to process events and manage user activity
- **Amazon DynamoDB**: Table to store user activity data and session information

## Prerequisites

- AWS CLI configured with appropriate permissions
- Go 1.23
- NodeJS 22
- Serverless Framework installed globally: `npm install -g serverless`
